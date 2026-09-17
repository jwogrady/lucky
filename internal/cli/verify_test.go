package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jwogrady/lucky/credential"
)

const probeSecret = "hcp-live-key-do-not-print"

// fakeVault is a vault holding one real-shaped Housecall Pro credential: the
// title a person typed, the field labels 1Password gives an API credential, and
// the auth header form the wtp vault actually contains.
type fakeVault struct {
	resolveErr error
	fieldsErr  error
}

func (fakeVault) AuthMode() string { return "fake" }
func (fakeVault) Vaults(context.Context) ([]Vault, error) {
	return []Vault{{ID: "v1", Title: "wtp"}}, nil
}
func (fakeVault) Items(context.Context, string) ([]Item, error) {
	return []Item{{ID: "i1", Title: "Housecall Pro API key", Category: "ApiCredential"}}, nil
}
func (f fakeVault) Resolve(_ context.Context, ref string) (string, error) {
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	if ref != "op://wtp/Housecall Pro API key/credential" {
		return "", fmt.Errorf("unexpected reference %q", ref)
	}
	return probeSecret, nil
}
func (f fakeVault) ItemFields(context.Context, string, string) ([]credential.Field, error) {
	if f.fieldsErr != nil {
		return nil, f.fieldsErr
	}
	return []credential.Field{
		{Label: "credential", Type: "CONCEALED", Secret: true, Reference: "op://wtp/Housecall Pro API key/credential"},
		{Label: "base url", Type: "URL", Value: "https://api.housecallpro.com"},
		{Label: "auth header", Type: "STRING", Value: "Authorization: Token <key>"},
	}, nil
}

type fakeDoer struct {
	status int
	body   string
	err    error
	seen   *http.Request
}

func (d *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	d.seen = req
	if d.err != nil {
		return nil, d.err
	}
	return &http.Response{
		StatusCode: d.status,
		Body:       io.NopCloser(strings.NewReader(d.body)),
		Header:     http.Header{},
	}, nil
}

func verifyApp(c Client, d Doer) (*App, *bytes.Buffer, *bytes.Buffer) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	return &App{
		Out: out, Err: errOut, HTTP: d,
		NewClient: func(context.Context, Config) (Client, error) { return c, nil },
	}, out, errOut
}

func TestVerifyPassesAndSendsTheResolvedSecret(t *testing.T) {
	doer := &fakeDoer{status: 200, body: `{"total_count":134}`}
	app, out, errOut := verifyApp(fakeVault{}, doer)
	if err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "ok\thousecallpro api\tHousecall Pro API key\t") {
		t.Fatalf("unexpected output %q", out.String())
	}
	if got := doer.seen.Header.Get("Authorization"); got != "Token "+probeSecret {
		t.Fatalf("the resolved secret did not reach the request: %q", got)
	}
	if got := doer.seen.URL.String(); got != "https://api.housecallpro.com/api/price_book/services?page_size=1" {
		t.Fatalf("unexpected url %q", got)
	}
	if strings.Contains(out.String()+errOut.String(), probeSecret) {
		t.Fatal("the secret was printed")
	}
}

// A rejected credential has to exit non-zero: that is what makes this usable in
// a pipeline rather than something a person reads and forgets.
func TestVerifyFailsAndExitsNonZero(t *testing.T) {
	app, out, errOut := verifyApp(fakeVault{}, &fakeDoer{status: 401, body: `{"message":"Unauthorized"}`})
	err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"})
	if err == nil {
		t.Fatal("a failed verification must return an error")
	}
	if !strings.Contains(out.String(), "failed\thousecallpro api") || !strings.Contains(out.String(), "401") {
		t.Fatalf("unexpected output %q", out.String())
	}
	if strings.Contains(out.String()+errOut.String()+err.Error(), probeSecret) {
		t.Fatal("the secret was printed")
	}
}

// A transport error quotes the URL, and for a provider that takes its key in
// the query string that URL is a secret.
func TestVerifyRedactsTheSecretOutOfATransportError(t *testing.T) {
	doer := &fakeDoer{err: errors.New(`Get "https://api.housecallpro.com/x?key=` + probeSecret + `": timeout`)}
	app, out, errOut := verifyApp(fakeVault{}, doer)
	if err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"}); err == nil {
		t.Fatal("expected failure")
	}
	if strings.Contains(out.String()+errOut.String(), probeSecret) {
		t.Fatalf("secret leaked through the transport error: %q", out.String())
	}
	if !strings.Contains(out.String(), "[REDACTED]") {
		t.Fatalf("expected a redaction marker: %q", out.String())
	}
}

// A response body can echo what was sent.
func TestVerifyRedactsTheSecretOutOfTheResponseBody(t *testing.T) {
	app, out, errOut := verifyApp(fakeVault{}, &fakeDoer{status: 403, body: `{"rejected":"` + probeSecret + `"}`})
	if err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"}); err == nil {
		t.Fatal("expected failure")
	}
	if strings.Contains(out.String()+errOut.String(), probeSecret) {
		t.Fatalf("secret leaked through the response body: %q", out.String())
	}
}

func TestVerifyReportsWhatItCouldNotCheck(t *testing.T) {
	// A vault item that matches a service with no stanza.
	app, out, _ := verifyApp(unstanzaedVault{}, &fakeDoer{status: 200})
	if err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unchecked\tgoogle service-account") {
		t.Fatalf("an unverifiable credential must still be reported: %q", out.String())
	}
}

type unstanzaedVault struct{ fakeVault }

func (unstanzaedVault) Items(context.Context, string) ([]Item, error) {
	return []Item{{ID: "i2", Title: "Google service account wtp-analytics-reader"}}, nil
}

func TestVerifyNeedsAnInspector(t *testing.T) {
	app, _, _ := verifyApp(fakeClient{}, &fakeDoer{status: 200})
	err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"})
	if err == nil || !strings.Contains(err.Error(), "item fields") {
		t.Fatalf("expected a clear capability error, got %v", err)
	}
}

func TestVerifyFailureToResolveIsNotAPass(t *testing.T) {
	app, out, _ := verifyApp(fakeVault{resolveErr: errors.New("vault locked")}, &fakeDoer{status: 200})
	if err := app.Run(context.Background(), []string{"verify", "--vault", "wtp"}); err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(out.String(), "failed") || !strings.Contains(out.String(), "vault locked") {
		t.Fatalf("unexpected output %q", out.String())
	}
}

// `lucky` with no arguments is the most-run invocation there is, so what it
// says matters more than any single command's output.
func TestBriefReportsWithoutGuessingAVault(t *testing.T) {
	app, out, errOut := verifyApp(fakeVault{}, &fakeDoer{status: 200})
	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	both := out.String() + errOut.String()
	if !strings.Contains(out.String(), "it's me, lucky") {
		t.Fatalf("unexpected briefing %q", out.String())
	}
	if !strings.Contains(out.String(), "the book") || !strings.Contains(out.String(), "the door") {
		t.Fatalf("briefing lost its substance %q", out.String())
	}
	// No default vault is configured, so nothing may be proposed by name.
	if !strings.Contains(both, "--vault <vault>") {
		t.Fatalf("expected a placeholder, not a guess: %q", both)
	}
	if strings.Contains(both, "--vault wtp") {
		t.Fatal("briefing guessed a vault from the vault list")
	}
}

// A locked vault or a new machine is a normal state, not a crash.
func TestBriefSurvivesNoAuth(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := &App{Out: out, Err: errOut, NewClient: func(context.Context, Config) (Client, error) {
		return nil, errors.New("no 1Password CLI found")
	}}
	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatalf("a locked vault must not be an error: %v", err)
	}
	if !strings.Contains(out.String(), "locked") || !strings.Contains(errOut.String(), "sign in") {
		t.Fatalf("unhelpful output %q / %q", out.String(), errOut.String())
	}
	// The local half still reports: templates do not need 1Password.
	if !strings.Contains(out.String(), "the book") {
		t.Fatalf("templates should report without auth: %q", out.String())
	}
}
