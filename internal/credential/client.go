package credential

import "context"

type Vault struct {
	ID, Title string
	ItemCount uint32
}

type Item struct {
	ID, Title, Category string
}

// Client is the interface Lucky's command surface uses for credential custody.
// It deliberately contains no vendor-authority or business-data operations.
type Client interface {
	AuthMode() string
	Vaults(context.Context) ([]Vault, error)
	Items(context.Context, string) ([]Item, error)
	Resolve(context.Context, string) (string, error)
}
