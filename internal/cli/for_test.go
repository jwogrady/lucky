package cli

import (
	"strings"
	"testing"
)

// `lucky for` is the intake path — the one that takes a credential down while
// somebody is reading it out, before anyone has decided what shape it is. It
// was the only command in the CLI with no tests, which is the wrong one to
// leave unproven: put walks a template and can only store what the catalog
// already describes, so everything that arrives from a vendor nobody
// anticipated arrives through here.

// person answers requirePerson's four questions. Every vault must name
// somebody before it holds a credential, so every intake test pays this cost.
const person = "Hank\nBlare\nhank@blare.example\n940-555-0100\n"

func TestForFilesAVendorNoTemplateDescribes(t *testing.T) {
	// acme-crm is in no template and never will be. That is the case this
	// command exists for: an intake that demands the shape first is an intake
	// that does not happen.
	app, out, _, c := custodianApp(person + "acme-secret-value\n\ny\n")
	if err := app.Run(t.Context(), []string{"for", "wtp", "acme-crm", "api", "key"}); err != nil {
		t.Fatalf("filing an untemplated vendor failed: %v", err)
	}
	if c.created.Name != "acme-crm" {
		t.Errorf("credential name = %q, want acme-crm", c.created.Name)
	}
	if c.created.Values["api key"] != "acme-secret-value" {
		t.Errorf("stored values = %v, want the value under %q", c.created.Values, "api key")
	}
	if !strings.Contains(out.String(), "op://") {
		t.Errorf("no reference returned on stdout, got %q", out.String())
	}
}

func TestForConcealsEveryKeyUnlessPlain(t *testing.T) {
	// Secrecy is a rule, not an inference. Reading the key's name to decide
	// would mean the operator cannot predict what Lucky does with a name it
	// has not seen, and a key left readable because nobody anticipated its
	// name is a secret on a shared screen.
	app, _, _, c := custodianApp(person + "https://api.acme.test\n\ny\n")
	if err := app.Run(t.Context(), []string{"for", "wtp", "acme-crm", "endpoint"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range c.created.Fields {
		if !f.Secret {
			t.Errorf("%q stored readable; every key is concealed unless --plain", f.Label)
		}
	}

	app, _, _, c = custodianApp(person + "https://api.acme.test\n\ny\n")
	if err := app.Run(t.Context(), []string{"for", "wtp", "acme-crm", "endpoint", "--plain"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range c.created.Fields {
		if f.Secret {
			t.Errorf("%q concealed despite --plain", f.Label)
		}
	}
}

func TestForRequiresAPersonBeforeItHoldsACredential(t *testing.T) {
	// Every credential in a vault was granted by a person, and the day a grant
	// is questioned the answer has to be a name. Blank means no.
	app, _, _, c := custodianApp("\n")
	err := app.Run(t.Context(), []string{"for", "wtp", "acme-crm", "api", "key"})
	if err == nil {
		t.Fatal("a credential was accepted into a vault that names nobody")
	}
	if !strings.Contains(err.Error(), "first name is required") {
		t.Errorf("error = %v, want it to name the missing field", err)
	}
	if c.created.Name != "" {
		t.Errorf("wrote %q despite having no person", c.created.Name)
	}
}

func TestForNeverPrintsTheSecret(t *testing.T) {
	const secret = "acme-live-key-do-not-print"
	app, out, errOut, _ := custodianApp(person + secret + "\n\ny\n")
	if err := app.Run(t.Context(), []string{"for", "wtp", "acme-crm", "api", "key"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), secret) {
		t.Errorf("secret reached stdout: %q", out.String())
	}
	if strings.Contains(errOut.String(), secret) {
		t.Errorf("secret reached stderr: %q", errOut.String())
	}
}

func TestForMatchesNamesExactlyAndNotByOverlap(t *testing.T) {
	// The fake vault holds "Google". A near-match here would hand over the
	// wrong secret, and nobody corrects that because it looks like an answer.
	// "goog" must therefore be a new credential, not a hit on Google.
	app, _, errOut, c := custodianApp(person + "some-value\n\ny\n")
	if err := app.Run(t.Context(), []string{"for", "wtp", "goog", "api", "key"}); err != nil {
		t.Fatal(err)
	}
	if c.created.Name != "goog" {
		t.Fatalf("created %q; a partial name matched an existing credential", c.created.Name)
	}
	// And it says what is already on file, so the duplicate is visible.
	if !strings.Contains(errOut.String(), "Google") {
		t.Errorf("did not show what is already filed, got %q", errOut.String())
	}
}

func TestForDeclinedWritesNothing(t *testing.T) {
	app, _, _, c := custodianApp(person + "acme-secret-value\n\nn\n")
	if err := app.Run(t.Context(), []string{"for", "wtp", "acme-crm", "api", "key"}); err != nil {
		t.Fatal(err)
	}
	if c.created.Name == "acme-crm" {
		t.Error("declining the confirmation still wrote the credential")
	}
}
