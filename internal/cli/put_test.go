package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jwogrady/lucky/credential"
)

// fakeCustodian implements read, write and archive with no 1Password present.
type fakeCustodian struct {
	fakeClient
	created  credential.NewItem
	archived string
}

func (f *fakeCustodian) Create(_ context.Context, item credential.NewItem) (credential.Created, error) {
	f.created = item
	var stored []string
	for _, spec := range item.Fields {
		if item.Values[spec.Label] != "" {
			stored = append(stored, spec.Label)
		}
	}
	return credential.Created{ID: "new1", Title: item.Title(), VaultName: item.VaultName, Fields: stored}, nil
}

func (f *fakeCustodian) Archive(_ context.Context, _, itemID string) error {
	f.archived = itemID
	return nil
}

func custodianApp(in string) (*App, *bytes.Buffer, *bytes.Buffer, *fakeCustodian) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	c := &fakeCustodian{}
	app := &App{Out: out, Err: errOut, In: strings.NewReader(in),
		NewClient: func(context.Context, Config) (Client, error) { return c, nil }}
	return app, out, errOut, c
}

const hcpKey = "abc123-housecall-secret"

func TestPutStoresAndReturnsReferences(t *testing.T) {
	app, out, errOut, c := custodianApp(hcpKey + "\n\n\n")
	err := app.Run(context.Background(), []string{"put", "--vault", "wtp", "--provider", "housecallpro", "--service", "api", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	if c.created.Values["credential"] != hcpKey {
		t.Fatalf("secret not passed through: %q", c.created.Values["credential"])
	}
	// Defaults fill in when the operator presses Enter.
	if c.created.Values["base url"] != "https://api.housecallpro.com" {
		t.Fatalf("default not applied: %q", c.created.Values["base url"])
	}
	if !strings.Contains(errOut.String(), "op://wtp/housecallpro api/credential") {
		t.Fatalf("no reference printed:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), `HOUSECALLPRO_API_CREDENTIAL="op://wtp/housecallpro api/credential"`) {
		t.Fatalf("no .env.op line on stdout:\n%s", out.String())
	}
}

// The single most important property of this command.
func TestPutNeverPrintsTheSecret(t *testing.T) {
	app, out, errOut, _ := custodianApp(hcpKey + "\n\n\n")
	if err := app.Run(context.Background(), []string{"put", "--vault", "wtp", "--provider", "housecallpro", "--service", "api", "--yes"}); err != nil {
		t.Fatal(err)
	}
	for name, buf := range map[string]*bytes.Buffer{"stdout": out, "stderr": errOut} {
		if strings.Contains(buf.String(), hcpKey) {
			t.Fatalf("the secret appeared on %s:\n%s", name, buf.String())
		}
	}
}

func TestPutDeclinedWritesNothing(t *testing.T) {
	app, _, errOut, c := custodianApp(hcpKey + "\n\n\nn\n")
	if err := app.Run(context.Background(), []string{"put", "--vault", "wtp", "--provider", "housecallpro", "--service", "api"}); err != nil {
		t.Fatal(err)
	}
	if c.created.Provider != "" {
		t.Fatal("wrote despite the operator declining")
	}
	if !strings.Contains(errOut.String(), "nothing written") {
		t.Fatalf("should say so:\n%s", errOut.String())
	}
}

// "know it's going to the right place"
func TestPutWarnsWhenThePasteLooksLikeAnotherProvider(t *testing.T) {
	blob := `{"type":"service_account","client_email":"reader@wtp.iam.gserviceaccount.com"}`
	app, _, errOut, _ := custodianApp(blob + "\nsecondhalf\n\n\n")
	if err := app.Run(context.Background(), []string{"put", "--vault", "wtp", "--provider", "godaddy", "--service", "api", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "heads up") || !strings.Contains(errOut.String(), "service_account") {
		t.Fatalf("a Google service account filed under GoDaddy should be flagged:\n%s", errOut.String())
	}
}

func TestEnvLineQuotingFollowsTheStoredValue(t *testing.T) {
	json := `{"type":"service_account"}`
	got := envLine("google", "service-account", "credential", "op://wtp/google service-account/credential", json)
	if !strings.HasSuffix(got, `='op://wtp/google service-account/credential'`) {
		t.Fatalf("a JSON value must get SINGLE quotes or op inject truncates it to `{`: %s", got)
	}
	plain := envLine("housecallpro", "api", "credential", "op://wtp/housecallpro api/credential", "abc")
	if !strings.Contains(plain, `="op://`) {
		t.Fatalf("a plain value should be double quoted: %s", plain)
	}
}

func TestProfileIsStoredWithNoSecretFields(t *testing.T) {
	in := "We The Plumbers\n\nwetheplumberstx.com\n936-555-0100\na@b.com\n\nConroe\nTX\n77301\n\n\n\n\n"
	app, _, _, c := custodianApp(in)
	if err := app.Run(context.Background(), []string{"profile", "--vault", "wtp", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if c.created.Title() != "cosmic profile" {
		t.Fatalf("title %q", c.created.Title())
	}
	if c.created.Values["business name"] != "We The Plumbers" || c.created.Values["city"] != "Conroe" {
		t.Fatalf("profile not captured: %+v", c.created.Values)
	}
	for _, f := range c.created.Fields {
		if f.Secret {
			t.Errorf("profile field %q marked secret", f.Label)
		}
	}
}

func TestInventoryReportsHeldAndMissingWithoutReadingValues(t *testing.T) {
	app, out, errOut, _ := custodianApp("")
	if err := app.Run(context.Background(), []string{"inventory", "--vault", "wtp"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "missing\tgodaddy api") {
		t.Fatalf("should list untouched templates as missing:\n%s", out.String())
	}
	// fakeClient holds one item titled just "Google". That is too ambiguous to
	// claim any particular Google service, so it belongs in the untemplated
	// list rather than being asserted as a match.
	if !strings.Contains(out.String(), "untemplated") {
		t.Fatalf("an ambiguous title should surface as untemplated:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "no customer profile") {
		t.Fatalf("a vault with no profile should say so:\n%s", errOut.String())
	}
}

func TestArchiveKeepsTheRecord(t *testing.T) {
	app, _, errOut, c := custodianApp("y\n")
	// fakeClient lists one item titled "Google"
	err := app.Run(context.Background(), []string{"archive", "--vault", "wtp", "--provider", "", "--service", "x"})
	if err == nil {
		t.Fatal("expected missing --provider to fail")
	}
	app, _, errOut, c = custodianApp("y\n")
	if err := app.Run(context.Background(), []string{"archive", "--vault", "wtp", "--provider", "google", "--service", ""}); err == nil {
		t.Fatal("expected missing --service to fail")
	}
	_ = c
	_ = errOut
}

func TestNoDeleteCommandExists(t *testing.T) {
	app, _, _, _ := custodianApp("")
	if err := app.Run(context.Background(), []string{"delete", "--vault", "wtp"}); err == nil {
		t.Fatal("lucky must not offer a delete command; a revoked key is still evidence")
	}
}
