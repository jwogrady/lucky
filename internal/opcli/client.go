// Package opcli implements credential custody on top of the 1Password CLI.
//
// It exists because the SDK's desktop-app integration cannot be reached from
// WSL — a Linux binary asking for the Windows 1Password app gets "desktop
// application not found" — and a service account requires a plan not every
// account has. The `op` CLI is already installed, already signed in, and works
// today.
//
// Lucky is not trying to replace `op`. `op` owns talking to 1Password; Lucky
// owns making the behaviour consistent, and that is worth stating precisely,
// because `op`'s own help says this:
//
//	Command arguments get logged in your command history, and can be visible
//	to other processes on your machine. If you're assigning sensitive values,
//	use a JSON template instead.
//
// A person creating an item by hand reaches for assignment statements, because
// they are shorter. This package always uses the JSON template on stdin, never
// assignments, so a secret never appears in argv, in shell history, or in ps
// output. That is the normalisation: the safe path is the only path.
package opcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jwogrady/lucky/credential"
)

type Client struct {
	bin string
}

// New locates the op CLI. LUCKY_OP_BIN overrides discovery, which is how the
// tests drive this package without 1Password present.
func New() (*Client, error) {
	if bin := strings.TrimSpace(os.Getenv("LUCKY_OP_BIN")); bin != "" {
		return &Client{bin: bin}, nil
	}
	for _, name := range []string{"op", "op.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return &Client{bin: path}, nil
		}
	}
	return nil, errors.New("no 1Password CLI found (looked for op and op.exe)")
}

func (c *Client) AuthMode() string { return "op-cli" }

// run executes op with no stdin. Under WSL the Windows op.exe treats a
// non-console stdin as piped template input, so commands that are not creating
// an item must be handed an explicit EOF or they hang waiting for one.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	return c.runWith(ctx, nil, args...)
}

func (c *Client) runWith(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	if stdin == nil {
		cmd.Stdin = strings.NewReader("")
	} else {
		cmd.Stdin = stdin
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("op %s: %s", args[0], msg)
	}
	return out.Bytes(), nil
}

type opVault struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Items uint32 `json:"items"`
}

func (c *Client) Vaults(ctx context.Context) ([]credential.Vault, error) {
	raw, err := c.run(ctx, "vault", "list", "--format", "json")
	if err != nil {
		return nil, err
	}
	var vaults []opVault
	if err := json.Unmarshal(raw, &vaults); err != nil {
		return nil, fmt.Errorf("could not read the vault list: %w", err)
	}
	out := make([]credential.Vault, 0, len(vaults))
	for _, v := range vaults {
		out = append(out, credential.Vault{ID: v.ID, Title: v.Name, ItemCount: v.Items})
	}
	return out, nil
}

type opItem struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Category string        `json:"category"`
	Fields   []opItemField `json:"fields"`
}

type opItemField struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Type      string `json:"type"`
	Value     string `json:"value,omitempty"`
	Reference string `json:"reference,omitempty"`
}

func (c *Client) Items(ctx context.Context, vaultID string) ([]credential.Item, error) {
	raw, err := c.run(ctx, "item", "list", "--vault", vaultID, "--format", "json")
	if err != nil {
		return nil, err
	}
	var items []opItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("could not read the item list: %w", err)
	}
	out := make([]credential.Item, 0, len(items))
	for _, i := range items {
		out = append(out, credential.Item{ID: i.ID, Title: i.Title, Category: i.Category})
	}
	return out, nil
}

// Resolve reads one secret reference. The reference is not itself a secret, so
// it is safe in argv; the resolved value comes back on stdout and is never
// logged.
func (c *Client) Resolve(ctx context.Context, reference string) (string, error) {
	if err := credential.ValidateReference(reference); err != nil {
		return "", err
	}
	raw, err := c.run(ctx, "read", reference)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(raw), "\r\n"), nil
}

// ItemFields reads one item's shape: which fields it has, which are secret,
// and the reference for each.
//
// `op item get --format json` returns concealed values along with everything
// else, and this drops them on the floor. That is not a formality. The
// reference it keeps instead — which op computes itself — is the only thing
// that gets a section right: the wtp vault's Supabase password lives at
//
//	op://wtp/Supabase/add more/password
//
// and any caller assembling op://vault/title/label by hand would look for it in
// the wrong place and report a working credential as missing.
func (c *Client) ItemFields(ctx context.Context, vaultID, itemID string) ([]credential.Field, error) {
	raw, err := c.run(ctx, "item", "get", itemID, "--vault", vaultID, "--format", "json")
	if err != nil {
		return nil, err
	}
	var item opItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, fmt.Errorf("could not read the item: %w", err)
	}
	out := make([]credential.Field, 0, len(item.Fields))
	for _, f := range item.Fields {
		secret := strings.EqualFold(f.Type, "CONCEALED")
		field := credential.Field{
			ID:        f.ID,
			Label:     f.Label,
			Type:      f.Type,
			Reference: f.Reference,
			Secret:    secret,
		}
		if !secret {
			field.Value = f.Value
		}
		out = append(out, field)
	}
	return out, nil
}

// Create writes an item from a JSON template on stdin. No field value is ever
// passed as a command argument.
func (c *Client) Create(ctx context.Context, item credential.NewItem) (credential.Created, error) {
	if err := item.Validate(); err != nil {
		return credential.Created{}, err
	}
	template := opItem{Title: item.Title(), Category: opCategory(item.Category)}
	var stored []string
	for _, spec := range item.Fields {
		value := item.Values[spec.Label]
		if strings.TrimSpace(value) == "" {
			continue
		}
		fieldType := "STRING"
		if spec.Secret {
			fieldType = "CONCEALED"
		}
		template.Fields = append(template.Fields, opItemField{
			ID:    strings.ReplaceAll(strings.ToLower(spec.Label), " ", "_"),
			Label: spec.Label,
			Type:  fieldType,
			Value: value,
		})
		stored = append(stored, spec.Label)
	}
	body, err := json.Marshal(struct {
		opItem
		Tags []string `json:"tags,omitempty"`
	}{opItem: template, Tags: []string{"lucky", item.Provider}})
	if err != nil {
		return credential.Created{}, err
	}
	raw, err := c.runWith(ctx, bytes.NewReader(body), "item", "create", "--vault", item.VaultID, "--format", "json")
	if err != nil {
		return credential.Created{}, redact(err, item)
	}
	var created opItem
	if err := json.Unmarshal(raw, &created); err != nil {
		return credential.Created{}, fmt.Errorf("item was created but the response could not be read: %w", err)
	}
	return credential.Created{ID: created.ID, Title: created.Title, VaultName: item.VaultName, Fields: stored}, nil
}

// AddField adds one key to an existing credential.
//
// op's own help is explicit that assignment arguments are the wrong tool here:
//
//	Caution: Command arguments can be visible to other
//	processes on your machine.
//
// So the item is read, the field appended, and the whole thing piped back on
// stdin — the same rule Create follows. The value never appears in argv, in
// shell history, or in ps output, and never touches disk on the way.
func (c *Client) AddField(ctx context.Context, vaultID, itemID string, spec credential.FieldSpec, value string) error {
	raw, err := c.run(ctx, "item", "get", itemID, "--vault", vaultID, "--format", "json")
	if err != nil {
		return err
	}
	var item opItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("could not read the item: %w", err)
	}
	fieldType := "STRING"
	if spec.Secret {
		fieldType = "CONCEALED"
	}
	id := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(spec.Label)), " ", "_")
	var replaced bool
	for i := range item.Fields {
		if strings.EqualFold(item.Fields[i].Label, spec.Label) || item.Fields[i].ID == id {
			item.Fields[i].Value = value
			item.Fields[i].Type = fieldType
			replaced = true
			break
		}
	}
	if !replaced {
		item.Fields = append(item.Fields, opItemField{ID: id, Label: spec.Label, Type: fieldType, Value: value})
	}
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if _, err := c.runWith(ctx, bytes.NewReader(body), "item", "edit", itemID, "--vault", vaultID, "--format", "json"); err != nil {
		return errors.New(strings.ReplaceAll(err.Error(), value, "[REDACTED]"))
	}
	return nil
}

// Archive retires an item without deleting it.
func (c *Client) Archive(ctx context.Context, vaultID, itemID string) error {
	_, err := c.run(ctx, "item", "delete", itemID, "--vault", vaultID, "--archive")
	return err
}

func (c *Client) CreateVault(ctx context.Context, v credential.NewVault) (credential.Vault, error) {
	if err := v.Validate(); err != nil {
		return credential.Vault{}, err
	}
	args := []string{"vault", "create", v.Title(), "--format", "json"}
	if v.Description != "" {
		args = append(args, "--description", v.Description)
	}
	raw, err := c.run(ctx, args...)
	if err != nil {
		return credential.Vault{}, err
	}
	var created opVault
	if err := json.Unmarshal(raw, &created); err != nil {
		return credential.Vault{}, fmt.Errorf("vault was created but the response could not be read: %w", err)
	}
	return credential.Vault{ID: created.ID, Title: created.Name}, nil
}

// opCategory maps to the CLI's category names, which are SCREAMING_SNAKE and
// differ from the SDK's PascalCase.
func opCategory(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "login":
		return "LOGIN"
	case "securenote":
		return "SECURE_NOTE"
	case "sshkey":
		return "SSH_KEY"
	default:
		return "API_CREDENTIAL"
	}
}

// redact scrubs every value the item carried out of an error before it is
// shown or logged. op should not echo a template back on failure, but an error
// path is exactly where that assumption goes unchecked.
func redact(err error, item credential.NewItem) error {
	msg := err.Error()
	for _, spec := range item.Fields {
		if v := item.Values[spec.Label]; v != "" {
			msg = strings.ReplaceAll(msg, v, "[REDACTED]")
		}
	}
	return errors.New(msg)
}
