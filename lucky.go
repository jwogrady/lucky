// Package lucky is the library entry point for the CosmOS credential custodian.
//
// A consumer that wants a credential needs exactly two things: a way to
// construct a client, and the credential.Client interface it returns. Both are
// public. The 1Password SDK, the authentication modes, and the redaction of
// secrets out of error text stay in internal/opclient, where a consumer neither
// sees them nor can depend on them.
//
//	client, err := lucky.New(ctx, lucky.Options{Account: os.Getenv("LUCKY_OP_ACCOUNT")})
//	secret, err := client.Resolve(ctx, "op://wtp/Housecall Pro API key/credential")
//
// Lucky knows the keys. Connections knows the doors. Collect brings the data
// home. Nothing about vendor authority or collected data belongs in here.
package lucky

import (
	"context"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/opclient"
)

// Options configures how Lucky authenticates to 1Password.
//
// Account names the 1Password account for desktop authentication and is
// required unless OP_SERVICE_ACCOUNT_TOKEN is set, which takes precedence.
// Version is reported to 1Password as integration info; it is cosmetic.
type Options struct {
	Account string
	Version string
}

// New returns a credential client. The concrete type is deliberately not
// exported: consumers depend on credential.Client, so Lucky can change
// providers without breaking them.
func New(ctx context.Context, opts Options) (credential.Client, error) {
	return opclient.New(ctx, opclient.Config{Account: opts.Account, Version: opts.Version})
}
