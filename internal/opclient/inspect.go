package opclient

import (
	"context"
	"fmt"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/jwogrady/lucky/credential"
)

// ItemFields reads one credential's shape without its secrets.
//
// The SDK hands back every value on Get, including concealed ones, and this
// drops them — the same rule the CLI backend follows, for the same reason:
// Client.Resolve stays the one way a secret leaves this package, so "never
// logged, never on disk" is a property of the code rather than a promise each
// caller has to keep.
//
// The two backends have to agree here, not merely both work. A credential that
// reads back differently depending on whether a service-account token happened
// to be set is a credential nobody can reason about.
func (c *Client) ItemFields(ctx context.Context, vaultID, itemID string) ([]credential.Field, error) {
	item, err := c.sdk.Items().Get(ctx, vaultID, itemID)
	if err != nil {
		return nil, safeError("could not read item", err, c.token)
	}
	vaultName, err := c.vaultTitle(ctx, vaultID)
	if err != nil {
		return nil, err
	}
	sections := map[string]string{}
	for _, s := range item.Sections {
		sections[s.ID] = s.Title
	}

	out := make([]credential.Field, 0, len(item.Fields))
	for _, f := range item.Fields {
		secret := f.FieldType == onepassword.ItemFieldTypeConcealed
		field := credential.Field{
			ID:        f.ID,
			Label:     f.Title,
			Type:      string(f.FieldType),
			Secret:    secret,
			Reference: reference(vaultName, item.Title, sectionTitle(sections, f.SectionID), f.Title),
		}
		if !secret {
			field.Value = f.Value
		}
		out = append(out, field)
	}
	return out, nil
}

// AddField adds or replaces one key on an existing credential.
//
// The SDK has no append: Put replaces the whole item. So the item is read,
// modified and written back, and Version travels with it — 1Password rejects a
// Put carrying a stale version, which is what stops two operators taking the
// same credential down at once and one of them silently winning.
func (c *Client) AddField(ctx context.Context, vaultID, itemID string, spec credential.FieldSpec, value string) error {
	item, err := c.sdk.Items().Get(ctx, vaultID, itemID)
	if err != nil {
		return safeError("could not read item", err, c.token)
	}
	fieldType := onepassword.ItemFieldTypeText
	if spec.Secret {
		fieldType = onepassword.ItemFieldTypeConcealed
	}
	id := fieldID(spec.Label)
	var replaced bool
	for i := range item.Fields {
		if item.Fields[i].ID == id || strings.EqualFold(item.Fields[i].Title, spec.Label) {
			item.Fields[i].Value = value
			item.Fields[i].FieldType = fieldType
			replaced = true
			break
		}
	}
	if !replaced {
		item.Fields = append(item.Fields, onepassword.ItemField{
			ID:        id,
			Title:     spec.Label,
			FieldType: fieldType,
			Value:     value,
		})
	}
	if _, err := c.sdk.Items().Put(ctx, item); err != nil {
		return safeError("could not update item", err, c.token, value)
	}
	return nil
}

// vaultTitle resolves a vault's name, which the item does not carry and a
// reference cannot be built without.
func (c *Client) vaultTitle(ctx context.Context, vaultID string) (string, error) {
	vaults, err := c.sdk.Vaults().List(ctx)
	if err != nil {
		return "", safeError("could not list vaults", err, c.token)
	}
	for _, v := range vaults {
		if v.ID == vaultID {
			return v.Title, nil
		}
	}
	return "", fmt.Errorf("vault %s is not accessible", vaultID)
}

func sectionTitle(sections map[string]string, id *string) string {
	if id == nil {
		return ""
	}
	if title, ok := sections[*id]; ok && strings.TrimSpace(title) != "" {
		return title
	}
	// A section with no title still separates fields, and its ID is what a
	// reference has to name.
	return *id
}

// reference builds op://vault/item[/section]/field, which is the addressing
// scheme and not a secret.
func reference(vault, item, section, field string) string {
	if section == "" {
		return fmt.Sprintf("op://%s/%s/%s", vault, item, field)
	}
	return fmt.Sprintf("op://%s/%s/%s/%s", vault, item, section, field)
}
