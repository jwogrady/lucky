package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeClient struct {
	resolved   string
	resolveErr error
}

func (fakeClient) AuthMode() string { return "service-account" }
func (fakeClient) Vaults(context.Context) ([]Vault, error) {
	return []Vault{{ID: "v1", Title: "wtp", ItemCount: 2}}, nil
}
func (fakeClient) Items(context.Context, string) ([]Item, error) {
	return []Item{{ID: "i1", Title: "Google", Category: "ApiCredential"}}, nil
}
func (f fakeClient) Resolve(context.Context, string) (string, error) { return f.resolved, f.resolveErr }

func appFor(f fakeClient) (*App, *bytes.Buffer, *bytes.Buffer) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	return &App{Out: out, Err: errOut, NewClient: func(context.Context, Config) (Client, error) { return f, nil }}, out, errOut
}

func TestStatus(t *testing.T) {
	app, out, _ := appFor(fakeClient{})
	if err := app.Run(context.Background(), []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "authenticated\tservice-account\nvaults\t1\n" {
		t.Fatalf("unexpected output %q", got)
	}
}
func TestItemsResolvesVaultName(t *testing.T) {
	app, out, _ := appFor(fakeClient{})
	if err := app.Run(context.Background(), []string{"items", "--vault", "wtp"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Google") {
		t.Fatalf("unexpected output %q", out.String())
	}
}
func TestGetWritesOnlySecretToStdout(t *testing.T) {
	app, out, errOut := appFor(fakeClient{resolved: "s3cr3t"})
	if err := app.Run(context.Background(), []string{"get", "op://wtp/item/password"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "s3cr3t" || strings.Contains(errOut.String(), "s3cr3t") {
		t.Fatalf("unsafe output stdout=%q stderr=%q", out.String(), errOut.String())
	}
}
func TestGetFailureDoesNotPrintSecret(t *testing.T) {
	app, out, _ := appFor(fakeClient{resolveErr: errors.New("nope")})
	if err := app.Run(context.Background(), []string{"get", "op://wtp/item/password"}); err == nil {
		t.Fatal("expected error")
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output %q", out.String())
	}
}
