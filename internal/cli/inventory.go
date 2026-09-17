package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// inventoryCommand answers "what do I have for this customer, and what am I
// missing" by crossing the provider catalog against what is in the vault.
//
// Lucky forgets every secret the moment it is handed over; what it does not
// forget is where things live, and this is that memory made legible. Titles
// only — no field is read and no secret is resolved — so it is safe to run in
// front of the customer.
//
// Matching is loose and says so. A vault names things the way a person named
// them, and a false "missing" would send an operator to reissue a key that
// already works.
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
			vaultName = orDefault(vaultName, a.defaultVault())
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
			matched, unmatched := cat.MatchItems(items)
			missing := cat.Missing(matched)

			var profile bool
			for _, item := range items {
				if strings.EqualFold(strings.TrimSpace(item.Title), "cosmic profile") {
					profile = true
				}
			}
			if !profile {
				fmt.Fprintf(a.Err, "no customer profile in %s — run: lucky profile --vault %s\n\n", vaultName, vaultName)
			}
			if !missingOnly {
				for _, m := range matched {
					state := "likely"
					if m.Exact {
						state = "held"
					}
					fmt.Fprintf(a.Out, "%s\t%s %s\t%s\top://%s/%s\n", state, m.Provider, m.Service, m.ItemName, vaultName, m.ItemName)
				}
			}
			for _, t := range missing {
				fmt.Fprintf(a.Out, "missing\t%s\t\t\n", t)
			}
			if !missingOnly {
				for _, item := range unmatched {
					fmt.Fprintf(a.Out, "untemplated\t\t%s\top://%s/%s\n", item.Title, vaultName, item.Title)
				}
			}
			fmt.Fprintf(a.Err, "\n%d matched, %d missing of %d templated services; %d items in the vault with no template\n",
				len(matched), len(missing), len(matched)+len(missing), len(unmatched))
			if len(matched) > 0 {
				fmt.Fprintf(a.Err, "\"likely\" is a loose title match, not a convention match — check it before trusting it.\n")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault title or ID")
	cmd.Flags().BoolVar(&missingOnly, "missing", false, "only what is not there yet")
	return cmd
}
