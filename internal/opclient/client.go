package opclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/jwogrady/lucky/internal/credential"
)

type Config struct{ Account, Version string }

type Client struct {
	sdk         *onepassword.Client
	mode, token string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	version := cfg.Version
	if version == "" {
		version = "dev"
	}
	options := []onepassword.ClientOption{onepassword.WithIntegrationInfo("Lucky", version)}
	mode := "desktop"
	if token != "" {
		mode = "service-account"
		options = append(options, onepassword.WithServiceAccountToken(token))
	} else {
		if strings.TrimSpace(cfg.Account) == "" {
			return nil, errors.New("set LUCKY_OP_ACCOUNT (or --account) for desktop authentication, or OP_SERVICE_ACCOUNT_TOKEN for unattended use")
		}
		options = append(options, onepassword.WithDesktopAppIntegration(cfg.Account))
	}
	sdk, err := onepassword.NewClient(ctx, options...)
	if err != nil {
		return nil, safeError("authentication failed", err, token)
	}
	return &Client{sdk: sdk, mode: mode, token: token}, nil
}

func (c *Client) AuthMode() string { return c.mode }

func (c *Client) Vaults(ctx context.Context) ([]credential.Vault, error) {
	result, err := c.sdk.Vaults().List(ctx)
	if err != nil {
		return nil, safeError("could not list vaults", err, c.token)
	}
	vaults := make([]credential.Vault, 0, len(result))
	for _, v := range result {
		vaults = append(vaults, credential.Vault{ID: v.ID, Title: v.Title, ItemCount: v.ActiveItemCount})
	}
	return vaults, nil
}

func (c *Client) Items(ctx context.Context, vaultID string) ([]credential.Item, error) {
	result, err := c.sdk.Items().List(ctx, vaultID)
	if err != nil {
		return nil, safeError("could not list items", err, c.token)
	}
	items := make([]credential.Item, 0, len(result))
	for _, item := range result {
		items = append(items, credential.Item{ID: item.ID, Title: item.Title, Category: string(item.Category)})
	}
	return items, nil
}

func (c *Client) Resolve(ctx context.Context, reference string) (string, error) {
	if err := ValidateReference(reference); err != nil {
		return "", err
	}
	if err := onepassword.Secrets.ValidateSecretReference(ctx, reference); err != nil {
		return "", safeError("invalid secret reference", err, c.token, reference)
	}
	secret, err := c.sdk.Secrets().Resolve(ctx, reference)
	if err != nil {
		return "", safeError("could not resolve secret reference", err, c.token, reference)
	}
	return secret, nil
}

func ValidateReference(reference string) error {
	if !strings.HasPrefix(reference, "op://") {
		return errors.New("secret reference must start with op://")
	}
	rest := strings.TrimPrefix(reference, "op://")
	if strings.ContainsAny(rest, "?#\\") {
		return errors.New("secret reference contains unsupported characters")
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 3 || len(parts) > 4 {
		return errors.New("secret reference must be op://vault/item/field or op://vault/item/section/field")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || part == "." || part == ".." {
			return errors.New("secret reference contains an empty or invalid segment")
		}
	}
	return nil
}

func safeError(action string, err error, secrets ...string) error {
	message := err.Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return fmt.Errorf("%s: %s", action, message)
}
