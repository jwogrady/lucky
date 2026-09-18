package cli

import (
	"fmt"
	"strings"

	"github.com/jwogrady/lucky/credential"
	"github.com/spf13/cobra"
)

// archiveCommand retires a credential that has been rotated or revoked.
//
// There is no delete command and there will not be one. An expired key is still
// the record of what was issued and when it stopped being trusted, which is
// what a rotation, an incident review, or a customer asking "what did that
// vendor ever have access to" all depend on. Archiving takes it out of the
// working set and keeps it recoverable.
func (a App) archiveCommand(account *string) *cobra.Command {
	var vaultName, provider, service string
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "archive [vault] [credential...]",
		Short: "Retire a credential, keeping the record",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.NewClient(cmd.Context(), Config{Account: *account})
			if err != nil {
				return err
			}
			archiver, ok := client.(credential.Archiver)
			if !ok {
				return fmt.Errorf("this client cannot archive")
			}
			vaultName = vaultFrom(args, vaultName, a.defaultVault())
			// A credential named directly, the way `lucky for` files one. The
			// provider/service pair still works for templated items, but it
			// could not name anything captured on the fly — which meant Lucky
			// could create credentials it had no way to retire.
			named := strings.TrimSpace(strings.Join(args[min(1, len(args)):], " "))
			if strings.TrimSpace(vaultName) == "" {
				return fmt.Errorf("a vault is required")
			}
			if named == "" && (strings.TrimSpace(provider) == "" || strings.TrimSpace(service) == "") {
				return fmt.Errorf("name the credential, or give --provider and --service")
			}
			vaultID, err := resolveVault(cmd.Context(), client, vaultName)
			if err != nil {
				return err
			}
			title := named
			if title == "" {
				title = strings.ToLower(provider + " " + service)
			}
			items, err := client.Items(cmd.Context(), vaultID)
			if err != nil {
				return err
			}
			for _, item := range items {
				if strings.EqualFold(strings.TrimSpace(item.Title), title) {
					if !assumeYes {
						p := a.prompter()
						ok, err := p.Confirm(fmt.Sprintf("archive %q in %s? it stays recoverable", item.Title, vaultName))
						if err != nil {
							return err
						}
						if !ok {
							fmt.Fprintln(a.Err, "left alone")
							return nil
						}
					}
					if err := archiver.Archive(cmd.Context(), vaultID, item.ID); err != nil {
						return err
					}
					fmt.Fprintf(a.Err, "archived %s — out of the working set, still on the record\n", item.Title)
					return nil
				}
			}
			return fmt.Errorf("no item titled %q in vault %s", title, vaultName)
		},
	}
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	cmd.Flags().StringVar(&provider, "provider", "", "provider")
	cmd.Flags().StringVar(&service, "service", "", "service")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation")
	return cmd
}
