package opclient

import (
	"context"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/jwogrady/lucky/credential"
)

// Create writes a new credential item and returns enough to build references
// to it. The values written are never returned, logged, or echoed.
func (c *Client) Create(ctx context.Context, item credential.NewItem) (credential.Created, error) {
	if err := item.Validate(); err != nil {
		return credential.Created{}, err
	}
	var fields []onepassword.ItemField
	var stored []string
	for _, spec := range item.Fields {
		value := item.Values[spec.Label]
		if strings.TrimSpace(value) == "" {
			continue // an optional field the operator skipped
		}
		fieldType := onepassword.ItemFieldTypeText
		if spec.Secret {
			fieldType = onepassword.ItemFieldTypeConcealed
		}
		fields = append(fields, onepassword.ItemField{
			ID:        fieldID(spec.Label),
			Title:     spec.Label,
			FieldType: fieldType,
			Value:     value,
		})
		stored = append(stored, spec.Label)
	}
	params := onepassword.ItemCreateParams{
		Category: category(item.Category),
		VaultID:  item.VaultID,
		Title:    item.Title(),
		Fields:   fields,
		Tags:     []string{"lucky", item.Provider},
	}
	if item.Notes != "" {
		params.Notes = &item.Notes
	}
	created, err := c.sdk.Items().Create(ctx, params)
	if err != nil {
		// The error may quote the values it was handed; redact every one.
		return credential.Created{}, safeError("could not create item", err, append(secretValues(item), c.token)...)
	}
	return credential.Created{
		ID:        created.ID,
		Title:     created.Title,
		VaultName: item.VaultName,
		Fields:    stored,
	}, nil
}

// category maps a shape onto a 1Password item category.
//
// An SshKey service deliberately lands in ApiCredentials with the key in a Concealed
// field rather than in the native SshKey category. The native category carries
// type-specific field details this code cannot verify without a live vault, and
// a wrong guess there would fail at write time against a customer's real vault.
// A concealed field stores and resolves identically through op:// references.
// Revisit once there is a vault to test against.
func category(name string) onepassword.ItemCategory {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "login":
		return onepassword.ItemCategoryLogin
	case "securenote":
		return onepassword.ItemCategorySecureNote
	default:
		return onepassword.ItemCategoryAPICredentials
	}
}

func fieldID(label string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(label)), " ", "_")
}

func secretValues(item credential.NewItem) []string {
	var out []string
	for _, spec := range item.Fields {
		if v := item.Values[spec.Label]; v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Archive retires an item. 1Password keeps archived items recoverable, which is
// the point: Lucky never destroys the record of a credential that once existed.
func (c *Client) Archive(ctx context.Context, vaultID, itemID string) error {
	if err := c.sdk.Items().Archive(ctx, vaultID, itemID); err != nil {
		return safeError("could not archive item", err, c.token)
	}
	return nil
}

// CreateVault provisions a customer's vault.
func (c *Client) CreateVault(ctx context.Context, v credential.NewVault) (credential.Vault, error) {
	if err := v.Validate(); err != nil {
		return credential.Vault{}, err
	}
	params := onepassword.VaultCreateParams{Title: v.Title()}
	if v.Description != "" {
		params.Description = &v.Description
	}
	created, err := c.sdk.Vaults().Create(ctx, params)
	if err != nil {
		return credential.Vault{}, safeError("could not create vault", err, c.token)
	}
	return credential.Vault{ID: created.ID, Title: created.Title}, nil
}
