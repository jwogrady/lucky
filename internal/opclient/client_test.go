package opclient

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeErrorRedactsSensitiveInputs(t *testing.T) {
	err := safeError("failed", errors.New("token abc and op://wtp/item/password"), "abc", "op://wtp/item/password")
	if strings.Contains(err.Error(), "abc") || strings.Contains(err.Error(), "op://") {
		t.Fatalf("unsafe error: %s", err)
	}
}
