package lucky

import (
	"context"
	"strings"
	"testing"
)

// With the SDK backend and neither authentication mode available, New must
// fail closed and name both ways out.
func TestSDKBackendRequiresAnAuthenticationMode(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")
	_, err := New(context.Background(), Options{Backend: BackendSDK})
	if err == nil {
		t.Fatal("expected an error with no account and no service-account token")
	}
	if !strings.Contains(err.Error(), "LUCKY_OP_ACCOUNT") || !strings.Contains(err.Error(), "OP_SERVICE_ACCOUNT_TOKEN") {
		t.Fatalf("error should name both authentication paths: %s", err)
	}
}

func TestUnknownBackendIsRejected(t *testing.T) {
	if _, err := New(context.Background(), Options{Backend: "carrier-pigeon"}); err == nil {
		t.Fatal("expected an unknown backend to fail")
	}
}

// auto must reach the CLI when no service-account token is present, which is
// the WSL case: the desktop app is unreachable from a Linux binary but op is
// installed and signed in.
func TestAutoFallsBackToTheCLIWhenThereIsNoToken(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")
	stub := writeStubOp(t)
	client, err := New(context.Background(), Options{OpBin: stub})
	if err != nil {
		t.Fatal(err)
	}
	if client.AuthMode() != "op-cli" {
		t.Fatalf("auto should have chosen the CLI, got %q", client.AuthMode())
	}
}
