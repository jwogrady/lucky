package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/envfile"
	"github.com/spf13/cobra"
)

type Config struct{ Account string }

// Defaults supplies settings resolved from flags, environment and config file.
// It is an interface so the CLI keeps no dependency on viper, and so tests can
// run with no configuration at all.
type Defaults interface {
	Vault() string
	Templates() string
	Used() string
}

func (a App) defaultVault() string {
	if a.Config == nil {
		return ""
	}
	return a.Config.Vault()
}

func (a App) templatesDir() string {
	if a.Config == nil {
		return ""
	}
	return a.Config.Templates()
}

type Vault = credential.Vault
type Item = credential.Item
type Client = credential.Client

type App struct {
	In        io.Reader
	Out       io.Writer
	Err       io.Writer
	Config    Defaults
	NewClient func(context.Context, Config) (Client, error)

	// HTTP performs the calls `lucky verify` makes to vendors. Nil is the
	// normal case and means a real client; a test supplies its own.
	HTTP Doer
}

// ExitError carries a child process's status so the caller can exit with it
// instead of flattening every failure to 1. It is not a Lucky error and carries
// no diagnostic of its own: the child has already reported whatever went wrong.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("command exited with status %d", e.Code) }

func (a App) Run(ctx context.Context, args []string) error {
	if a.Out == nil || a.Err == nil || a.NewClient == nil {
		return errors.New("invalid application configuration")
	}
	root := a.command()
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func (a App) command() *cobra.Command {
	account := os.Getenv("LUCKY_OP_ACCOUNT")
	root := &cobra.Command{Use: "lucky", Short: "CosmOS credential custodian", SilenceErrors: true, SilenceUsage: true, Args: cobra.ArbitraryArgs}
	// Bare words go to the customer Lucky is already working for:
	//
	//	lucky for status26       once
	//	lucky blare api key      every time after
	//
	// Typing speed is the whole point of this surface. A person on a phone call
	// should not be retyping the customer's name into every lookup, and the
	// context is already set and already persisted.
	//
	// It stays unambiguous because a subcommand always wins — cobra matches
	// those before RunE is reached — and because it only fires when a working
	// customer exists. With none set it says so rather than guessing which of
	// the words was meant to be the customer.
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return a.brief(cmd, account)
		}
		working := a.defaultVault()
		if working == "" {
			return fmt.Errorf("no customer set: run `lucky for <customer>` first, or name one: `lucky for <customer> %s`", strings.Join(args, " "))
		}
		return a.forWorkingVault(cmd, &account, working, args)
	}
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	root.PersistentFlags().StringVar(&account, "account", account, "1Password account name or UUID for desktop authentication")
	client := func(cmd *cobra.Command) (Client, error) { return a.NewClient(cmd.Context(), Config{Account: account}) }

	root.AddCommand(&cobra.Command{Use: "status", Short: "Verify authentication and report accessible vault count", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := client(cmd)
		if err != nil {
			return err
		}
		vaults, err := c.Vaults(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "authenticated\t%s\nvaults\t%d\n", c.AuthMode(), len(vaults))
		return nil
	}})
	root.AddCommand(&cobra.Command{Use: "vaults", Short: "List accessible vaults without secret values", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := client(cmd)
		if err != nil {
			return err
		}
		vaults, err := c.Vaults(cmd.Context())
		if err != nil {
			return err
		}
		sort.Slice(vaults, func(i, j int) bool { return vaults[i].Title < vaults[j].Title })
		for _, v := range vaults {
			fmt.Fprintf(a.Out, "%s\t%s\t%d\n", v.ID, v.Title, v.ItemCount)
		}
		return nil
	}})
	var vaultName string
	items := &cobra.Command{Use: "items", Short: "List item metadata in a vault", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(vaultName) == "" {
			return errors.New("--vault is required")
		}
		c, err := client(cmd)
		if err != nil {
			return err
		}
		vaultID, err := resolveVault(cmd.Context(), c, vaultName)
		if err != nil {
			return err
		}
		result, err := c.Items(cmd.Context(), vaultID)
		if err != nil {
			return err
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Title < result[j].Title })
		for _, item := range result {
			fmt.Fprintf(a.Out, "%s\t%s\t%s\n", item.ID, item.Title, item.Category)
		}
		return nil
	}}
	items.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	root.AddCommand(items)
	root.AddCommand(&cobra.Command{Use: "get op://vault/item/field", Short: "Resolve one secret reference to stdout", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client(cmd)
		if err != nil {
			return err
		}
		secret, err := c.Resolve(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		_, err = io.WriteString(a.Out, secret)
		return err
	}})
	root.AddCommand(a.providersCommand())
	root.AddCommand(a.profileCommand(&account))
	root.AddCommand(a.putCommand(&account))
	root.AddCommand(a.inventoryCommand(&account))
	root.AddCommand(a.verifyCommand(&account))
	root.AddCommand(a.forCommand(&account))
	root.AddCommand(a.archiveCommand(&account))
	root.AddCommand(a.newVaultCommand(&account))

	var envFile string
	run := &cobra.Command{Use: "run --env-file FILE -- command [args...]", Short: "Run a command with op:// references resolved into its environment", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(envFile) == "" {
			return errors.New("--env-file is required")
		}
		file, err := os.Open(envFile)
		if err != nil {
			return err
		}
		entries, err := envfile.Parse(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", envFile, err)
		}
		resolved, err := resolveAll(cmd.Context(), func() (Client, error) { return client(cmd) }, entries)
		if err != nil {
			return err
		}
		// The resolved values leave this process only through the child's
		// environment: never stdout, stderr, a log line, or disk.
		env := os.Environ()
		for _, e := range entries {
			env = append(env, e.Key+"="+envfile.Expand(e.Value, resolved))
		}
		return a.exec(cmd.Context(), args, env)
	}}
	run.Flags().StringVar(&envFile, "env-file", "", "env file of op:// references to resolve")
	run.Flags().SetInterspersed(false) // flags after the command belong to the child
	root.AddCommand(run)
	return root
}

// vaultFrom picks the vault a command should act on.
//
// The vault is the one argument every operator command needs, so it reads as a
// positional: `lucky put blare`, matching `lucky new-customer "Blare"`. The
// flag still works, and a configured default still applies, in that order —
// what you typed on this invocation beats what you typed in a config file
// last month.
func vaultFrom(args []string, flag, configured string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0])
	}
	return orDefault(flag, configured)
}

// resolveAll resolves every reference before the child starts: a child holding a
// partially resolved environment is worse than no child at all. It attempts all
// of them and reports every failure at once, because a single stale item name
// otherwise reads as "the vault is broken" rather than "this line is wrong".
func resolveAll(ctx context.Context, newClient func() (Client, error), entries []envfile.Entry) (map[string]string, error) {
	refs := map[string][]envfile.Entry{}
	var order []string
	for _, e := range entries {
		for _, ref := range envfile.Refs(e.Value) {
			if _, seen := refs[ref]; !seen {
				order = append(order, ref)
			}
			refs[ref] = append(refs[ref], e)
		}
	}
	if len(order) == 0 {
		return nil, nil // a file of literals needs no 1Password session at all
	}
	c, err := newClient()
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(order))
	var failures []string
	for _, ref := range order {
		secret, err := c.Resolve(ctx, ref)
		if err != nil {
			for _, e := range refs[ref] {
				failures = append(failures, fmt.Sprintf("%s (line %d): %s", e.Key, e.Line, redact(err.Error(), ref)))
			}
			continue
		}
		resolved[ref] = secret
	}
	if len(failures) > 0 {
		// Redact anything that did resolve, in case a vendor error quoted it.
		for i, f := range failures {
			for _, secret := range resolved {
				f = redact(f, secret)
			}
			failures[i] = f
		}
		return nil, fmt.Errorf("could not resolve %d of %d references:\n  %s", len(failures), len(order), strings.Join(failures, "\n  "))
	}
	return resolved, nil
}

// exec runs the child with stdio wired straight through and mirrors its status.
func (a App) exec(ctx context.Context, args []string, env []string) error {
	stdin := a.In
	if stdin == nil {
		stdin = os.Stdin
	}
	child := exec.CommandContext(ctx, args[0], args[1:]...)
	child.Env = env
	child.Stdin, child.Stdout, child.Stderr = stdin, a.Out, a.Err
	err := child.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if code < 0 { // killed by a signal
			code = 1
		}
		return &ExitError{Code: code}
	}
	return err
}

// redact mirrors opclient.safeError: sensitive input never reaches a message.
func redact(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return message
}

func resolveVault(ctx context.Context, client Client, wanted string) (string, error) {
	vaults, err := client.Vaults(ctx)
	if err != nil {
		return "", err
	}
	for _, v := range vaults {
		if v.ID == wanted || strings.EqualFold(v.Title, wanted) {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("vault %q is not accessible", wanted)
}
