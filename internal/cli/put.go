package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/prompt"
	"github.com/spf13/cobra"
)

// The operator commands: capture a customer's profile into their vault, and
// store credentials against provider templates.
//
// The experience these exist to deliver: something arrives by email or text,
// it gets pasted into a terminal once, and it lands in the right vault under
// the right name with the right field labels. Nothing is written to disk,
// nothing is echoed, and what comes back is a reference.

func (a App) catalog() (credential.Catalog, error) {
	dir := a.templatesDir()
	if dir == "" {
		if home, err := os.UserConfigDir(); err == nil {
			dir = home + "/lucky/providers"
		}
	}
	return credential.LoadCatalog(dir)
}

func (a App) in() io.Reader {
	if a.In != nil {
		return a.In
	}
	return os.Stdin
}

func (a App) providersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "List provider templates and where to get each credential",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cat, err := a.catalog()
			if err != nil {
				return err
			}
			for _, name := range cat.Providers() {
				t := cat[name]
				fmt.Fprintf(a.Out, "%s\t%s\t%s\n", name, t.Title, strings.Join(cat.Services(name), ","))
				if t.Where != "" {
					fmt.Fprintf(a.Err, "    %s\n", t.Where)
				}
				if t.Docs != "" {
					fmt.Fprintf(a.Err, "    %s\n", t.Docs)
				}
			}
			return nil
		},
	}
}

func (a App) profileCommand(account *string) *cobra.Command {
	var vaultName string
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "profile [vault]",
		Short: "Capture a customer's business profile into their vault",
		Long: "Collect the customer and business details and store them in the customer's\n" +
			"vault. None of it is secret. It lives there because the vault is the\n" +
			"customer boundary, and identity belongs next to authority.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, creator, err := a.custodian(cmd.Context(), *account)
			if err != nil {
				return err
			}
			p := prompt.New(a.in(), a.Err)
			if vaultName, err = required(p, vaultFrom(args, vaultName, a.defaultVault()), "vault", "the customer's vault"); err != nil {
				return err
			}
			vaultID, err := resolveVault(cmd.Context(), client, vaultName)
			if err != nil {
				return err
			}
			values, err := collect(p, credential.ProfileFields())
			if err != nil {
				return err
			}
			item := credential.ProfileItem(vaultID, vaultName, values)
			if err := item.Validate(); err != nil {
				return err
			}
			return a.write(cmd.Context(), p, creator, item, assumeYes)
		},
	}
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation")
	return cmd
}

func (a App) putCommand(account *string) *cobra.Command {
	var vaultName, provider, service, notes string
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "put [vault]",
		Short: "Store a credential and get its op:// reference back",
		Long: "Store a credential against a provider template and print its op:// references.\n\n" +
			"The secret is read without being echoed, written straight to 1Password, and\n" +
			"never placed on disk. What comes back is the reference, which is safe to\n" +
			"paste into configuration and to commit.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := a.catalog()
			if err != nil {
				return err
			}
			client, creator, err := a.custodian(cmd.Context(), *account)
			if err != nil {
				return err
			}
			p := prompt.New(a.in(), a.Err)

			if provider == "" {
				i, err := p.Choose("which provider?", cat.Providers())
				if err != nil {
					return err
				}
				provider = cat.Providers()[i]
			}
			if t, ok := cat[provider]; ok && t.Where != "" {
				fmt.Fprintf(a.Err, "\n  where to get it: %s\n", t.Where)
			}
			if service == "" {
				services := cat.Services(provider)
				if len(services) == 1 {
					service = services[0]
				} else {
					i, err := p.Choose("which service?", services)
					if err != nil {
						return err
					}
					service = services[i]
				}
			}
			fields, err := cat.Fields(provider, service)
			if err != nil {
				return err
			}
			if vaultName, err = required(p, vaultFrom(args, vaultName, a.defaultVault()), "vault", "the customer's vault"); err != nil {
				return err
			}
			vaultID, err := resolveVault(cmd.Context(), client, vaultName)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.Err, "\nstoring %s %s in vault %s\n\n", provider, service, vaultName)
			values, err := collect(p, fields)
			if err != nil {
				return err
			}
			item := credential.NewItem{
				VaultID: vaultID, VaultName: vaultName,
				Provider: provider, Service: service,
				Fields: fields, Values: values, Notes: notes,
			}
			if err := item.Validate(); err != nil {
				return err
			}
			a.warnIfMisrouted(item)
			return a.write(cmd.Context(), p, creator, item, assumeYes)
		},
	}
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	cmd.Flags().StringVar(&provider, "provider", "", "provider template")
	cmd.Flags().StringVar(&service, "service", "", "which service of that provider")
	cmd.Flags().StringVar(&notes, "notes", "", "non-secret note stored on the item")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation")
	return cmd
}

// warnIfMisrouted is the "know it is going to the right place" guarantee.
// Detection reads structure only, and a disagreement is surfaced rather than
// enforced: the operator may genuinely be storing something unusual, but they
// should never do it by accident.
func (a App) warnIfMisrouted(item credential.NewItem) {
	for _, f := range item.Fields {
		guess, ok := credential.Detect(item.Values[f.Label])
		if !ok {
			continue
		}
		if guess.Provider == item.Provider && (guess.Service == item.Service || guess.Field == f.Label) {
			continue
		}
		fmt.Fprintf(a.Err, "\n  heads up: %q looks like %s — you are filing it as %s %s / %s\n",
			f.Label, guess.Reason, item.Provider, item.Service, f.Label)
	}
}

func (a App) custodian(ctx context.Context, account string) (Client, credential.Creator, error) {
	client, err := a.NewClient(ctx, Config{Account: account})
	if err != nil {
		return nil, nil, err
	}
	creator, ok := client.(credential.Creator)
	if !ok {
		return nil, nil, fmt.Errorf("this client cannot write to a vault")
	}
	return client, creator, nil
}

func (a App) write(ctx context.Context, p *prompt.Prompter, creator credential.Creator, item credential.NewItem, assumeYes bool) error {
	fmt.Fprintf(a.Err, "\n%s in vault %s:\n", item.Title(), item.VaultName)
	for _, f := range item.Fields {
		if v := item.Values[f.Label]; v != "" {
			fmt.Fprintf(a.Err, "  %s\n", prompt.Describe(f.Label, v))
		}
	}
	if !assumeYes {
		ok, err := p.Confirm("\nwrite it?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(a.Err, "nothing written")
			return nil
		}
	}
	created, err := creator.Create(ctx, item)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Err, "\nstored. it is in the vault and nowhere else.\n\nreferences:\n")
	for _, f := range created.Fields {
		fmt.Fprintf(a.Err, "  %s\n", created.Reference(f))
	}
	fmt.Fprintf(a.Err, "\n.env.op lines:\n")
	for _, f := range created.Fields {
		fmt.Fprintf(a.Out, "%s\n", envLine(item.Provider, item.Service, f, created.Reference(f), item.Values[f]))
	}
	return nil
}

func collect(p *prompt.Prompter, fields []credential.FieldSpec) (map[string]string, error) {
	values := map[string]string{}
	for _, spec := range fields {
		v, err := read(p, spec)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(v) == "" && spec.Default != "" {
			v = spec.Default
		}
		values[spec.Label] = v
	}
	return values, nil
}

func required(p *prompt.Prompter, given, label, help string) (string, error) {
	if strings.TrimSpace(given) != "" {
		return given, nil
	}
	v, err := p.Line(label, help, false)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return v, nil
}

func read(p *prompt.Prompter, spec credential.FieldSpec) (string, error) {
	help := spec.Help
	if spec.Default != "" {
		help = strings.TrimSpace(help + " (default: " + spec.Default + ")")
	}
	switch {
	case spec.Multiline:
		return p.Multiline(spec.Label, help, spec.Optional)
	case spec.Secret:
		return p.Secret(spec.Label, help, spec.Optional)
	default:
		return p.Line(spec.Label, help, spec.Optional)
	}
}

var notVarChar = regexp.MustCompile(`[^A-Z0-9]+`)

// envLine builds the .env.op line for a stored field.
//
// The quoting is chosen from the STORED value, not from the reference, and that
// is the subtlety. op inject substitutes the resolved secret into this line, so
// a value containing a double quote — every service-account JSON blob begins
// {"type": — must sit inside single quotes, or the parser terminates the string
// at that inner quote and the variable silently becomes `{`. The wtp .env.op
// documents that failure in a comment written after it happened. Here it is
// decided automatically, because this is the only moment anything knows both
// the value and the line it is going into.
func envLine(provider, service, field, reference, value string) string {
	name := notVarChar.ReplaceAllString(strings.ToUpper(provider+"_"+service+"_"+field), "_")
	name = strings.Trim(name, "_")
	if strings.Contains(value, `"`) {
		return fmt.Sprintf("%s='%s'", name, reference)
	}
	return fmt.Sprintf("%s=%q", name, reference)
}

func orDefault(given, fallback string) string {
	if strings.TrimSpace(given) != "" {
		return given
	}
	return fallback
}
