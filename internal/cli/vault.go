package cli

import (
	"fmt"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/prompt"
	"github.com/spf13/cobra"
)

// newVaultCommand provisions a customer boundary, which is the first step of
// taking on a client and the thing every later step resolves through.
//
// The order is deliberate and the command states it: vault, then profile, then
// credentials. A credential with no customer boundary to land in is how keys
// end up in a shared vault that nobody can hand back.
func (a App) newVaultCommand(account *string) *cobra.Command {
	var description string
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "new-customer [name]",
		Short: "Create a customer's vault",
		Long: "Create the 1Password vault that is a customer's boundary.\n\n" +
			"Everything else Lucky does resolves through it: the profile lives in it,\n" +
			"every credential lands in it, and every op:// reference for that customer\n" +
			"begins with its name.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.NewClient(cmd.Context(), Config{Account: *account})
			if err != nil {
				return err
			}
			provisioner, ok := client.(credential.Provisioner)
			if !ok {
				return fmt.Errorf("this client cannot create vaults")
			}
			p := prompt.New(a.in(), a.Err)
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			if name, err = required(p, name, "customer", "the business name"); err != nil {
				return err
			}
			v := credential.NewVault{Customer: name, Description: description}
			if err := v.Validate(); err != nil {
				return err
			}
			fmt.Fprintf(a.Err, "\nvault will be named %q\n", v.Title())
			if !assumeYes {
				ok, err := p.Confirm("create it?")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(a.Err, "nothing created")
					return nil
				}
			}
			created, err := provisioner.CreateVault(cmd.Context(), v)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.Out, "%s\t%s\n", created.ID, created.Title)
			fmt.Fprintf(a.Err, "\ncreated. next:\n  lucky profile --vault %s\n  lucky put --vault %s\n  lucky inventory --vault %s\n",
				created.Title, created.Title, created.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "vault description")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation")
	return cmd
}
