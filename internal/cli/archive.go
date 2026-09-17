package cli

import (
	"fmt"
	"strings"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/prompt"
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
		Use:   "archive",
		Short: "Retire a credential, keeping the record",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.NewClient(cmd.Context(), Config{Account: *account})
			if err != nil {
				return err
			}
			archiver, ok := client.(credential.Archiver)
			if !ok {
				return fmt.Errorf("this client cannot archive")
			}
			if strings.TrimSpace(vaultName) == "" || strings.TrimSpace(provider) == "" || strings.TrimSpace(service) == "" {
				return fmt.Errorf("--vault, --provider and --service are required")
			}
			vaultID, err := resolveVault(cmd.Context(), client, vaultName)
			if err != nil {
				return err
			}
			title := strings.ToLower(provider + " " + service)
			items, err := client.Items(cmd.Context(), vaultID)
			if err != nil {
				return err
			}
			for _, item := range items {
				if strings.EqualFold(strings.TrimSpace(item.Title), title) {
					if !assumeYes {
						p := prompt.New(a.in(), a.Err)
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
