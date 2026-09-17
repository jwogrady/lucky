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
// Keep this package free of business data. It says what a credential is, how to
// ask for one, and — since access is part of a key rather than a separate
// service's opinion about it — how to prove one works. It never carries
// collected data.
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
// both use for credential custody.
type Client interface {
	AuthMode() string
	Vaults(context.Context) ([]Vault, error)
	Items(context.Context, string) ([]Item, error)
	Resolve(context.Context, string) (string, error)
}

// Field is one field of a stored credential, without its secret.
//
// The value of a concealed field is deliberately absent here, even though the
// provider hands it over in the same response. An item's shape — which fields
// exist, which are secret, what each one's reference is — is not sensitive and
// is needed to build anything on top of a credential. The secret itself has one
// way out of this package, Client.Resolve, and keeping it to one path is what
// makes "never logged, never on disk" a property of the code rather than a
// promise repeated in each caller.
type Field struct {
	ID        string
	Label     string
	Type      string
	Reference string
	Secret    bool
	Value     string // empty for every secret field
}

// Inspector reads the shape of a stored credential.
//
// It is separate from Client for the reason Creator is: most consumers resolve
// a reference they were given and need none of this, and an interface they do
// not hold is an interface they cannot misuse.
type Inspector interface {
	ItemFields(ctx context.Context, vaultID, itemID string) ([]Field, error)
}
