package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/prompt"
	"github.com/spf13/cobra"
)

// VaultSetter persists which customer Lucky is working for. Optional, so the
// CLI keeps no dependency on viper and tests run with no configuration.
type VaultSetter interface {
	SetVault(string) (string, error)
}

// forCommand is the taxonomy, built on the fly:
//
//	user > credential > key > value
//
//	lucky for blare                    who we're working for
//	lucky for blare mailgun            what we hold for that credential
//	lucky for blare mailgun api key    the value — handed over, or taken down
//
// Nothing has to exist first. A credential nobody has filed yet is created by
// naming it; a key nobody has filed yet is added by naming it. That is the
// whole point: what arrives by voice, email, photograph, text or paper arrives
// before anyone has decided what shape it is, and an intake that demands the
// shape first is an intake that does not happen.
//
// One credential holds as many keys as it needs — an endpoint, a username and
// three tokens are one thing to a person, so they are one item here.
func (a App) forCommand(account *string) *cobra.Command {
	var plain, asEnv bool
	cmd := &cobra.Command{
		Use:   "for <user> [credential] [key...]",
		Short: "Work for someone: hand over a credential, or take one down",
		Long: "user > credential > key > value, created as you go.\n\n" +
			"With a key, return the value — or, if Lucky holds nothing by that name,\n" +
			"offer a blank to enter it and keep it. With a credential and no key, say\n" +
			"what is on it. With neither, set who the other commands work for.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.NewClient(cmd.Context(), Config{Account: *account})
			if err != nil {
				return err
			}
			user := strings.TrimSpace(args[0])
			vaultID, err := resolveVault(cmd.Context(), client, user)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				return a.workFor(user)
			}

			name := strings.TrimSpace(args[1])
			items, err := client.Items(cmd.Context(), vaultID)
			if err != nil {
				return err
			}
			item, held := matchItem(items, name)
			key := strings.Join(args[2:], " ")

			switch {
			case !held && key == "":
				// Nothing by that name and no key named: take the whole thing
				// down now, however many pairs it turns out to have.
				return a.takeDown(cmd, client, vaultID, user, name, nil, plain, items)
			case !held:
				return a.takeDown(cmd, client, vaultID, user, name, []string{key}, plain, items)
			case key == "" && asEnv:
				return a.asEnv(cmd, client, vaultID, item)
			case key == "":
				return a.whatsOnIt(cmd, client, vaultID, item)
			}
			return a.handOverOrAdd(cmd, client, vaultID, user, item, key, plain)
		},
	}
	cmd.Flags().BoolVar(&plain, "plain", false, "store readable — for an endpoint or a username, which are not secrets")
	cmd.Flags().BoolVar(&asEnv, "env", false, "print every key as an environment variable, for a process that needs all of them")
	return cmd
}

// forWorkingVault is `lucky <credential> [key...]` against the customer already
// set, with the same behaviour as naming them explicitly.
func (a App) forWorkingVault(cmd *cobra.Command, account *string, working string, args []string) error {
	return a.forCommand(account).RunE(cmd, append([]string{working}, args...))
}

func (a App) workFor(user string) error {
	setter, ok := a.Config.(VaultSetter)
	if !ok {
		return fmt.Errorf("cannot remember a customer without a config file")
	}
	path, err := setter.SetVault(user)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "%s\n", user)
	fmt.Fprintf(a.Err, "workin' for %s now. it's in %s.\n", user, path)
	return nil
}

// whatsOnIt lists the keys, never the values.
func (a App) whatsOnIt(cmd *cobra.Command, client Client, vaultID string, item Item) error {
	inspector, ok := client.(credential.Inspector)
	if !ok {
		return fmt.Errorf("this client cannot read item fields")
	}
	fields, err := inspector.ItemFields(cmd.Context(), vaultID, item.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Err, "%s:\n", item.Title)
	var shown int
	for _, f := range fields {
		if !f.Secret && strings.TrimSpace(f.Value) == "" {
			continue
		}
		shown++
		if f.Secret {
			fmt.Fprintf(a.Out, "%s\theld\n", f.Label)
			continue
		}
		fmt.Fprintf(a.Out, "%s\t%s\n", f.Label, f.Value)
	}
	if shown == 0 {
		fmt.Fprintf(a.Err, "  nothin' on it yet\n")
	}
	return nil
}

// asEnv prints every key of one credential as an environment assignment.
//
// A credential is rarely useful one key at a time. Calling BLARE's endpoint
// needs the user, the key and the endpoint together, and fetching them with
// three separate invocations means three chances to pair the wrong ones and a
// secret sitting in a shell variable in between.
//
// Values are single-quoted with any inner quote escaped, because the quoting
// has to survive whatever the value contains — and what arrives by voice, email
// or photograph contains anything.
//
// This is the weaker half of a pair. `lucky run` puts these into one child
// process and nowhere else; this prints them, so whatever consumes the output
// owns them from that point. Prefer run where a command can be wrapped.
func (a App) asEnv(cmd *cobra.Command, client Client, vaultID string, item Item) error {
	inspector, ok := client.(credential.Inspector)
	if !ok {
		return fmt.Errorf("this client cannot read item fields")
	}
	fields, err := inspector.ItemFields(cmd.Context(), vaultID, item.ID)
	if err != nil {
		return err
	}
	for _, f := range fields {
		if !f.Secret && strings.TrimSpace(f.Value) == "" {
			continue
		}
		value := f.Value
		if f.Secret {
			if value, err = client.Resolve(cmd.Context(), f.Reference); err != nil {
				return err
			}
		}
		fmt.Fprintf(a.Out, "%s=%s\n", envName(item.Title, f.Label), shellQuote(value))
	}
	return nil
}

func envName(item, key string) string {
	name := notVarChar.ReplaceAllString(strings.ToUpper(item+"_"+key), "_")
	return strings.Trim(name, "_")
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

// handOverOrAdd returns the value if it is there, and takes it down if it is not.
func (a App) handOverOrAdd(cmd *cobra.Command, client Client, vaultID, user string, item Item, key string, plain bool) error {
	inspector, ok := client.(credential.Inspector)
	if !ok {
		return fmt.Errorf("this client cannot read item fields")
	}
	fields, err := inspector.ItemFields(cmd.Context(), vaultID, item.ID)
	if err != nil {
		return err
	}
	if field, ok := matchField(fields, key); ok {
		value := field.Value
		if field.Secret {
			if value, err = client.Resolve(cmd.Context(), field.Reference); err != nil {
				return err
			}
		}
		fmt.Fprintf(a.Err, "%s / %s:\n", item.Title, field.Label)
		fmt.Fprintf(a.Out, "%s\n", value)
		return nil
	}

	updater, ok := client.(credential.Updater)
	if !ok {
		return fmt.Errorf("this client cannot add a key to an existing credential")
	}
	fmt.Fprintf(a.Err, "%s has no key called %q.\n", item.Title, key)
	if labels := keyLabels(fields); len(labels) > 0 {
		fmt.Fprintf(a.Err, "  it has: %s\n", strings.Join(labels, ", "))
	}
	fmt.Fprintf(a.Err, "adding it.\n\n")
	p := prompt.New(a.in(), a.Err)
	spec := credential.FieldSpec{Label: key, Secret: concealed(plain)}
	value, err := a.readKey(p, spec)
	if err != nil || value == "" {
		return err
	}
	if err := updater.AddField(cmd.Context(), vaultID, item.ID, spec, value); err != nil {
		return err
	}
	fmt.Fprintf(a.Err, "\ngot it. on %s in %s, and nowhere else.\n", item.Title, user)
	fmt.Fprintf(a.Out, "op://%s/%s/%s\n", user, item.Title, key)
	return nil
}

// takeDown creates a credential that does not exist yet, taking as many keys as
// the operator has to give.
func (a App) takeDown(cmd *cobra.Command, client Client, vaultID, user, name string, keys []string, plain bool, existing []Item) error {
	creator, ok := client.(credential.Creator)
	if !ok {
		return fmt.Errorf("this client cannot write to a vault")
	}
	p := prompt.New(a.in(), a.Err)
	fmt.Fprintf(a.Err, "i got nothin' called %q for %s. read it to me.\n", name, user)
	// Names match exactly, so a typo makes a second credential rather than
	// finding the first. Showing what is already filed costs one line and is
	// the only warning available before a duplicate exists.
	if len(existing) > 0 {
		fmt.Fprintf(a.Err, "  already on file: %s\n", strings.Join(itemTitles(existing), ", "))
	}
	fmt.Fprintln(a.Err)

	var specs []credential.FieldSpec
	values := map[string]string{}
	ask := func(key string) error {
		spec := credential.FieldSpec{Label: key, Secret: concealed(plain)}
		value, err := a.readKey(p, spec)
		if err != nil || value == "" {
			return err
		}
		specs = append(specs, spec)
		values[key] = value
		return nil
	}
	for _, key := range keys {
		if err := ask(key); err != nil {
			return err
		}
	}
	// One endpoint might carry five pairs, so keep asking until nothing is
	// named. Entering them one command at a time is how half of them end up
	// never entered.
	for {
		key, err := p.Line("key", "endpoint, username, api key… blank when that's everything", true)
		if err != nil {
			return err
		}
		if strings.TrimSpace(key) == "" {
			break
		}
		if err := ask(strings.TrimSpace(key)); err != nil {
			return err
		}
	}
	if len(specs) == 0 {
		fmt.Fprintln(a.Err, "nothin' entered, nothin' kept")
		return nil
	}

	item := credential.NewItem{
		VaultID: vaultID, VaultName: user,
		Name:   name,
		Fields: specs, Values: values,
		Notes: "Taken down by lucky for. No template matched; classify it later.",
	}
	fmt.Fprintf(a.Err, "\n%s for %s:\n", name, user)
	for _, spec := range specs {
		fmt.Fprintf(a.Err, "  %s\n", prompt.Describe(spec.Label, values[spec.Label]))
		if guess, ok := credential.Detect(values[spec.Label]); ok {
			fmt.Fprintf(a.Err, "    looks like %s\n", guess.Reason)
		}
	}
	yes, err := p.Confirm("\nthat right?")
	if err != nil {
		return err
	}
	if !yes {
		fmt.Fprintln(a.Err, "nothin' kept")
		return nil
	}
	created, err := creator.Create(cmd.Context(), item)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Err, "\ngot it. it's in %s and nowhere else.\n", user)
	for _, f := range created.Fields {
		fmt.Fprintf(a.Out, "%s\n", created.Reference(f))
	}
	return nil
}

func (a App) readKey(p *prompt.Prompter, spec credential.FieldSpec) (string, error) {
	if spec.Secret {
		return p.Secret(spec.Label, "", true)
	}
	return p.Line(spec.Label, "", true)
}

// concealed decides whether a key holds a secret.
//
// It conceals, unless told otherwise. An earlier version read the key's name —
// "endpoint" plain, "api key" secret — and it was wrong to: the operator cannot
// predict what Lucky will do with a name it has not seen, and a key stored
// readable because nobody anticipated its name is a secret on a shared screen.
//
// Deterministic beats clever here. One rule, no list of markers to remember,
// and --plain when a value genuinely is not sensitive.
func concealed(plain bool) bool { return !plain }

// matchItem finds the credential by name, exactly.
//
// An earlier version scored titles by how many words overlapped and returned
// the best. That is the wrong trade here. Loose matching earns its place in
// `inventory`, where a false "missing" sends an operator to rotate a key that
// already works — a report can be slightly wrong and a person corrects it. This
// path returns a secret value, and the failure mode of a near-match is handing
// over the wrong one, which nobody corrects because it looks like an answer.
//
// So: exact, case-insensitive, trimmed. No match is an error that lists what is
// actually there, which is both deterministic and more useful than a guess.
func matchItem(items []Item, name string) (Item, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Title)) == want {
			return item, true
		}
	}
	return Item{}, false
}

func matchField(fields []credential.Field, key string) (credential.Field, bool) {
	want := strings.ToLower(strings.TrimSpace(key))
	for _, f := range fields {
		if strings.TrimSpace(f.Value) == "" && !f.Secret {
			continue
		}
		if strings.ToLower(strings.TrimSpace(f.Label)) == want {
			return f, true
		}
	}
	return credential.Field{}, false
}

func itemTitles(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Title)
	}
	sort.Strings(out)
	return out
}

func keyLabels(fields []credential.Field) []string {
	var out []string
	for _, f := range fields {
		if f.Secret || strings.TrimSpace(f.Value) != "" {
			out = append(out, f.Label)
		}
	}
	sort.Strings(out)
	return out
}
