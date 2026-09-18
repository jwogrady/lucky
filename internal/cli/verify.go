package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jwogrady/lucky/credential"
	"github.com/spf13/cobra"
)

// Doer performs the one HTTP call a check makes. It is an interface so the
// command can be tested against a vendor that does not exist.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// verifyCommand proves each credential in a vault against the system it is for.
//
// Existing is not the same as working. A key can resolve from 1Password, be
// perfectly well-formed, and still be revoked, scoped to the wrong account, or
// pointed at a different customer's tenant — and every one of those shows up
// later as something that looks like a different bug, usually in whichever
// collector ran first.
//
// This is also the strongest argument for Lucky holding access rather than a
// separate service: if Lucky owns the keys and access is part of the key, then
// Lucky owns proving it, and one command can answer "is this customer actually
// connected" for a whole vault. Nothing else in the platform is positioned to
// answer that, because nothing else holds the credentials.
//
// What it does not do is guess. A service whose template carries no verify
// stanza is reported as unchecked, in the output, every time — an unchecked
// credential silently omitted would make this report a liar.
func (a App) verifyCommand(account *string) *cobra.Command {
	var vaultName, provider string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "verify [vault]",
		Short: "Prove each credential in a vault against the system it is for",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := a.catalog()
			if err != nil {
				return err
			}
			client, err := a.NewClient(cmd.Context(), Config{Account: *account})
			if err != nil {
				return err
			}
			inspector, ok := client.(credential.Inspector)
			if !ok {
				return errors.New("this client cannot read item fields, which verification needs")
			}
			vaultName = vaultFrom(args, vaultName, a.defaultVault())
			if strings.TrimSpace(vaultName) == "" {
				return fmt.Errorf("--vault is required")
			}
			vaultID, err := resolveVault(cmd.Context(), client, vaultName)
			if err != nil {
				return err
			}
			items, err := client.Items(cmd.Context(), vaultID)
			if err != nil {
				return err
			}
			matched, _ := cat.MatchItems(items)
			sort.Slice(matched, func(i, j int) bool {
				if matched[i].Provider != matched[j].Provider {
					return matched[i].Provider < matched[j].Provider
				}
				return matched[i].Service < matched[j].Service
			})

			http := a.doer(timeout)
			var checked, passed, unchecked int
			for _, m := range matched {
				if provider != "" && !strings.EqualFold(provider, m.Provider) {
					continue
				}
				spec, ok := cat.Verification(m.Provider, m.Service)
				if !ok {
					unchecked++
					fmt.Fprintf(a.Out, "unchecked\t%s %s\t%s\tno verify stanza for this service\n", m.Provider, m.Service, m.ItemName)
					continue
				}
				checked++
				outcome, err := a.probe(cmd.Context(), http, client, inspector, cat, vaultID, m, *spec)
				switch {
				case err != nil:
					fmt.Fprintf(a.Out, "failed\t%s %s\t%s\t%s\n", m.Provider, m.Service, m.ItemName, err)
				case outcome.OK:
					passed++
					fmt.Fprintf(a.Out, "ok\t%s %s\t%s\t%s\n", m.Provider, m.Service, m.ItemName, outcome.Detail)
				default:
					fmt.Fprintf(a.Out, "failed\t%s %s\t%s\t%s\n", m.Provider, m.Service, m.ItemName, outcome.Detail)
				}
			}

			fmt.Fprintf(a.Err, "\n%d of %d credentials proved against their system", passed, checked)
			if unchecked > 0 {
				fmt.Fprintf(a.Err, "; %d held with no check available", unchecked)
			}
			fmt.Fprintln(a.Err, ".")
			if checked > passed {
				// A non-zero exit is the point of running this in a pipeline.
				return fmt.Errorf("%d credential(s) did not prove", checked-passed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	cmd.Flags().StringVar(&provider, "provider", "", "check only this provider")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "per-credential timeout")
	return cmd
}

func (a App) doer(timeout time.Duration) Doer {
	if a.HTTP != nil {
		return a.HTTP
	}
	return &http.Client{Timeout: timeout}
}

// probe resolves one credential and makes its single call.
//
// Every error out of here has been through Probe.Redact, including the ones
// that come back from the transport carrying the URL — Google takes its key in
// the query string, so "Get https://…?key=AIza…: timeout" is a secret printed
// to a terminal by an error path nobody was thinking about.
func (a App) probe(ctx context.Context, doer Doer, client Client, inspector credential.Inspector, cat credential.Catalog, vaultID string, m credential.Match, spec credential.Verify) (credential.Outcome, error) {
	fields, err := inspector.ItemFields(ctx, vaultID, m.ItemID)
	if err != nil {
		return credential.Outcome{}, err
	}

	var values credential.Values
	for _, f := range fields {
		value := f.Value
		if f.Secret {
			if f.Reference == "" {
				continue
			}
			// The secret takes the one path out of the credential package.
			value, err = client.Resolve(ctx, f.Reference)
			if err != nil {
				return credential.Outcome{}, fmt.Errorf("could not resolve %q: %w", f.Label, err)
			}
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		values = append(values, credential.FieldValue{Label: f.Label, Value: value, Secret: f.Secret})
	}
	if specFields, err := cat.Fields(m.Provider, m.Service); err == nil {
		values = values.WithDefaults(specFields)
	}

	p, err := credential.BuildProbe(spec, values)
	if err != nil {
		return credential.Outcome{}, errors.New(p.Redact(err.Error()))
	}
	req, err := http.NewRequestWithContext(ctx, p.Method, p.URL, nil)
	if err != nil {
		return credential.Outcome{}, errors.New(p.Redact(err.Error()))
	}
	req.Header.Set("Accept", "application/json")
	for _, h := range p.Headers {
		req.Header.Set(h.Name, h.Value)
	}
	resp, err := doer.Do(req)
	if err != nil {
		return credential.Outcome{}, errors.New(p.Redact(err.Error()))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return credential.Outcome{}, errors.New(p.Redact(err.Error()))
	}
	outcome := spec.Judge(resp.StatusCode, string(body))
	outcome.Detail = p.Redact(outcome.Detail)
	return outcome, nil
}
