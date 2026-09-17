// Package credential is Lucky's public boundary: the types and interface a
// consumer needs to obtain a credential without knowing that 1Password exists.
//
// This package was originally written as internal/credential, and that was a
// mistake with a traceable cost. Go forbids importing an internal/ package
// across module boundaries, so no other module could consume Lucky as a
// library — which is the whole point of a credential custodian. Lucky's own
// roadmap names "public Go interface for credential resolution" as the v0.4
// deliverable, so the placement contradicted the design from the first commit.
//
// The consequence was not hypothetical. cosmic-wtp, Lucky's first intended
// consumer, could not import this and grew its own copy in
// internal/credentials/op.go: thirty lines that shell out to the `op` CLI —
// which Lucky's README explicitly forbids — and interpolate the secret
// reference into unredacted error text. A duplicate is what a codebase builds
// when the thing it needs is sealed off, and the duplicate is always the weaker
// one, because the hardening lives on the side that got the attention.
//
// Keep this package free of vendor authority and business data. It says what a
// credential is and how to ask for one. It does not decide what a credential
// may reach — that is Connections — and it never carries collected data.
package credential

import "context"

type Vault struct {
	ID, Title string
	ItemCount uint32
}

type Item struct {
	ID, Title, Category string
}

// Client is the interface Lucky's command surface and its library consumers
// both use for credential custody. It deliberately contains no vendor-authority
// or business-data operations.
type Client interface {
	AuthMode() string
	Vaults(context.Context) ([]Vault, error)
	Items(context.Context, string) ([]Item, error)
	Resolve(context.Context, string) (string, error)
}
