package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// inventoryCommand answers "what do I have for this customer, and what am I
// missing" by crossing the provider catalog against what is actually in the
// vault.
//
// Lucky forgets every secret the moment it is handed over. What Lucky does not
// forget is where things live — and this is that memory made legible. Item
// titles only; no field is read and no secret is resolved, so this is safe to
// run in front of the customer.
func (a App) inventoryCommand(account *string) *cobra.Command {
	var vaultName string
	var missingOnly bool
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Show which provider credentials a vault holds, and which are missing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cat, err := a.catalog()
			if err != nil {
				return err
			}
			client, err := a.NewClient(cmd.Context(), Config{Account: *account})
			if err != nil {
				return err
			}
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
			held := map[string]bool{}
			for _, item := range items {
				held[strings.ToLower(strings.TrimSpace(item.Title))] = true
			}
			var have, missing []string
			for _, provider := range cat.Providers() {
				for _, service := range cat.Services(provider) {
					title := provider + " " + service
					if held[title] {
						have = append(have, title)
					} else {
						missing = append(missing, title)
					}
				}
			}
			sort.Strings(have)
			sort.Strings(missing)
			if !held["cosmic profile"] {
				fmt.Fprintf(a.Err, "no customer profile in %s — run: lucky profile --vault %s\n\n", vaultName, vaultName)
			}
			if !missingOnly {
				for _, t := range have {
					fmt.Fprintf(a.Out, "held\t%s\top://%s/%s\n", t, vaultName, t)
				}
			}
			for _, t := range missing {
				fmt.Fprintf(a.Out, "missing\t%s\t\n", t)
			}
			fmt.Fprintf(a.Err, "\n%d held, %d missing, of %d templated services\n", len(have), len(missing), len(have)+len(missing))
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	cmd.Flags().BoolVar(&missingOnly, "missing", false, "only what is not there yet")
	return cmd
}
