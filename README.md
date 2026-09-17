# Lucky

Lucky is the CosmOS credential custodian: it holds a customer's keys in 1Password, puts them where work needs them, and proves they still reach the systems they are for. Cosmic is the runtime and the storage; Collect is a workload, not a service.

## v0.1 CLI

Lucky uses Cobra for its command surface and viper for configuration, and reaches
1Password through one of two backends:

- **sdk** — the official 1Password Go SDK, for a service account.
- **cli** — the `op` CLI, which is what works on a WSL machine where the desktop
  app is on the Windows side and unreachable from a Linux binary.

`auto`, the default, prefers the SDK when `OP_SERVICE_ACCOUNT_TOKEN` is set and
falls back to the CLI. An earlier version of this file said Lucky never shells
out to `op`; that was true of the SDK-only v0.1 and is no longer. What Lucky
guarantees is the behaviour, not the transport: a secret never appears in argv,
in shell history, or in `ps` output on either path — the CLI backend always
writes items through a JSON template on stdin, never assignment arguments.

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

Run a command with a whole env file resolved into its environment:

```bash
./lucky run --env-file .env.op -- bun run dev
```

`lucky run` reads `KEY=op://vault/item/field` lines, resolves every reference
before starting anything, and passes the values to the child process through its
environment only. Values that are not references pass through literally, and a
file containing no references never opens a 1Password session. Every reference
that cannot be resolved is reported at once, by key, with the reference and the
resolved values redacted; nothing is executed unless all of them resolve. The
child's exit status becomes Lucky's. Unlike `lucky get`, `run` writes no secret
to stdout, stderr, a log, or disk.

References may be embedded in a larger value, which `op inject` supports and
`op run` does not:

```
SUPABASE_DB_URL="postgresql://postgres.ref:op://wtp/Supabase/password@host:5432/postgres"
```

`lucky get` intentionally writes the resolved value—and nothing else—to stdout so it can be consumed by a process. Diagnostics go to stderr and sensitive inputs are redacted. Avoid terminal history and command tracing when consuming secrets.

Desktop authentication requires the 1Password desktop app's SDK integration setting and a CGO-enabled build. Service-account authentication takes precedence when `OP_SERVICE_ACCOUNT_TOKEN` is present.

## Taking on a customer

```bash
lucky new-customer "We The Plumbers"   # the vault: their boundary
lucky profile --vault we-the-plumbers  # who they are; not secret, lives there anyway
lucky put --vault we-the-plumbers      # paste a credential, get a reference back
lucky inventory --vault we-the-plumbers # what is held, what is missing
lucky verify --vault we-the-plumbers   # which of them actually still work
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

## Proving a credential works

Existing is not the same as working. A key can resolve from 1Password, be
perfectly well-formed, and still be revoked, scoped to the wrong account, or
pointed at a different customer's tenant.

```bash
lucky verify --vault wtp
lucky verify --vault wtp --provider google
```

`verify` makes one read-only authenticated call per credential and reports what
came back, exiting non-zero if any credential failed. No secret value appears in
the output, in an error, or in a URL echoed back by a transport failure.

The call is **data, not code**. Each provider template carries a `verify` stanza:

```json
"verify": {"method": "GET", "path": "/user/tokens/verify", "expect": 200,
           "contains": "\"success\"", "proves": "the token is active"}
```

So adding a provider is an edit to `providers.json`, and no vendor's API becomes
a build-time dependency of the thing that holds the keys.

Two stanza fields exist because a status code is not always the truth:

- `contains` asserts something about the body. Google's Geocoding API answers
  HTTP 200 to a key it rejected, so a status-only check reports a dead key as
  working.
- `refusal` declares a rejection that *confirms* the credential — a
  referrer-restricted browser key is supposed to be refused server-side. It
  matches on the reason text, not just the status, because an invalid key and a
  correctly-restricted one are both `REQUEST_DENIED` and differ only in the
  message.

A check that cannot be expressed as one HTTP call gets no stanza, and the
credential is reported as `unchecked` rather than guessed at.

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

Lucky owns credential custody and the proof that a credential works. It does not grant access — the vendor did that when the key was issued — and it never carries business data: Collect retrieves that and writes it to Cosmic storage, deliberately outside this repository.

## Development

```bash
go test ./...
go vet ./...
```
