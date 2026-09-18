package credential

import (
	"errors"
	"strings"
)

// ValidateReference checks the shape of an op:// secret reference before any
// credential provider is contacted.
//
// This moved out of internal/opclient for the same reason the interface did: a
// consumer holding a reference from configuration should be able to reject a
// malformed one without importing a 1Password client, or reimplementing the
// check and getting the ".." case wrong.
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
