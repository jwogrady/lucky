package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jwogrady/lucky/credential"
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
	Out       io.Writer
	Err       io.Writer
	In        io.Reader
	Config    Defaults
	NewClient func(context.Context, Config) (Client, error)

	// HTTP performs the calls `lucky verify` makes to vendors. Nil is the
	// normal case and means a real client; a test supplies its own.
	HTTP Doer
}

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
	root := &cobra.Command{Use: "lucky", Short: "CosmOS credential custodian", SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs}
	root.RunE = func(cmd *cobra.Command, _ []string) error { return a.brief(cmd, account) }
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
	root.AddCommand(a.archiveCommand(&account))
	root.AddCommand(a.newVaultCommand(&account))
	return root
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
