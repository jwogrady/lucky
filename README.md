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

See [ROADMAP.md](ROADMAP.md) for scope and service boundaries.

Lucky owns credential custody only. Connections decides which vendor, service, and resource a credential authorizes; Collect retrieves authorized business data and writes it to Cosmic storage. Those services are deliberately outside this repository.

## Development

```bash
go test ./...
go vet ./...
```
