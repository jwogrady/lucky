package lucky

import (
	"context"
	"strings"
	"testing"
)

// New must fail closed when neither authentication mode is available, and say
// which environment variables would fix it.
func TestNewRequiresAnAuthenticationMode(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")
	_, err := New(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected an error with no account and no service-account token")
	}
	if !strings.Contains(err.Error(), "LUCKY_OP_ACCOUNT") || !strings.Contains(err.Error(), "OP_SERVICE_ACCOUNT_TOKEN") {
		t.Fatalf("error should name both authentication paths: %s", err)
	}
}
