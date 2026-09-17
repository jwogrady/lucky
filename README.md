# Lucky

Lucky is the CosmOS credential custodian: it manages credential access through 1Password while Connections owns vendor authority and Collect owns business-data retrieval.

## v0.1 CLI

Lucky uses Cobra for its command surface and the official 1Password Go SDK directly. It never shells out to the `op` CLI.

```bash
go build -o lucky ./cmd/lucky

# Operator authentication through the 1Password desktop app
export LUCKY_OP_ACCOUNT='your 1Password account name'
./lucky status

# Unattended authentication (do not place the token in a config file)
export OP_SERVICE_ACCOUNT_TOKEN='...'
./lucky status

./lucky vaults
./lucky items --vault wtp
./lucky get 'op://wtp/item/field'
```

`lucky get` intentionally writes the resolved value—and nothing else—to stdout so it can be consumed by a process. Diagnostics go to stderr and sensitive inputs are redacted. Avoid terminal history and command tracing when consuming secrets.

Desktop authentication requires the 1Password desktop app's SDK integration setting and a CGO-enabled build. Service-account authentication takes precedence when `OP_SERVICE_ACCOUNT_TOKEN` is present.

## Taking on a customer

```bash
lucky new-customer "We The Plumbers"   # the vault: their boundary
lucky profile --vault we-the-plumbers  # who they are; not secret, lives there anyway
lucky put --vault we-the-plumbers      # paste a credential, get a reference back
lucky inventory --vault we-the-plumbers # what is held, what is missing
lucky archive --vault we-the-plumbers --provider godaddy --service api
```

`put` reads the secret without echoing it, writes it straight to 1Password, and
prints the `op://` references plus the `.env.op` lines to paste them into. The
secret never touches disk and never appears in output.

`lucky providers` lists the templates — GoDaddy, Bluehost, cPanel, Cloudflare,
Google (service account, Ads, Maps), Housecall Pro, Yext, WordPress, Supabase,
Netlify, Vultr — with where in each console the credential comes from. Override
or add one by dropping a JSON file in `$LUCKY_TEMPLATES`.

There is no `delete`. `archive` retires a credential and keeps the record,
because a revoked key is still evidence of what was issued and when it stopped
being trusted.

## Use Lucky as a library

```go
import (
    "github.com/jwogrady/lucky"
    "github.com/jwogrady/lucky/credential"
)

client, err := lucky.New(ctx, lucky.Options{Account: os.Getenv("LUCKY_OP_ACCOUNT")})
secret, err := client.Resolve(ctx, "op://wtp/Housecall Pro API key/credential")
```

`credential.Client` and `lucky.New` are the entire public surface. The 1Password
SDK, the authentication modes, and the redaction of secrets out of error text
live in `internal/`, where a consumer can neither see them nor depend on them.

`credential.ValidateReference` checks the shape of an `op://` reference without
contacting any provider, so a consumer can reject a malformed reference that
came from configuration before it authenticates.

See [ROADMAP.md](ROADMAP.md) for scope and service boundaries.

Lucky owns credential custody only. Connections decides which vendor, service, and resource a credential authorizes; Collect retrieves authorized business data and writes it to Cosmic storage. Those services are deliberately outside this repository.

## Development

```bash
go test ./...
go vet ./...
```
