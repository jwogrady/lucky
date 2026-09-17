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
	"fmt"
	"os"

	"github.com/jwogrady/lucky/credential"
	"github.com/jwogrady/lucky/internal/opcli"
	"github.com/jwogrady/lucky/internal/opclient"
)

// Backend names how Lucky reaches 1Password.
//
// There are two, and neither is the product. 1Password is the system of record
// and the `op` CLI is a perfectly good way to talk to it; what Lucky adds is
// the same behaviour whichever one is underneath — secrets never in argv,
// never on disk, always redacted out of errors, always a reference back.
type Backend string

const (
	// BackendAuto prefers the SDK when a service account token is present and
	// falls back to the CLI, which is what works on a WSL machine where the
	// desktop app is on the Windows side and unreachable from a Linux binary.
	BackendAuto Backend = "auto"
	BackendSDK  Backend = "sdk"
	BackendCLI  Backend = "cli"
)

type Options struct {
	Account string
	Version string
	Backend Backend
	OpBin   string
}

// New returns a credential client using the requested backend.
func New(ctx context.Context, opts Options) (credential.Client, error) {
	switch opts.Backend {
	case BackendSDK:
		return opclient.New(ctx, opclient.Config{Account: opts.Account, Version: opts.Version})
	case BackendCLI:
		return newCLI(opts)
	case "", BackendAuto:
		if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "" {
			return opclient.New(ctx, opclient.Config{Account: opts.Account, Version: opts.Version})
		}
		if client, err := newCLI(opts); err == nil {
			return client, nil
		}
		// No CLI either: report the SDK's error, which names both auth paths.
		return opclient.New(ctx, opclient.Config{Account: opts.Account, Version: opts.Version})
	default:
		return nil, fmt.Errorf("unknown backend %q (auto, sdk, cli)", opts.Backend)
	}
}

func newCLI(opts Options) (credential.Client, error) {
	if opts.OpBin != "" {
		os.Setenv("LUCKY_OP_BIN", opts.OpBin)
	}
	return opcli.New()
}
