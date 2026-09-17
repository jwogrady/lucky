package credential

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinCatalogLoadsAndCoversTheMilestoneProviders(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	// Every provider named in the MILESTONES 000 credential list, plus the
	// ones added since, must have at least one service with a secret field.
	for _, p := range []string{"google", "housecallpro", "cpanel", "wordpress", "yext", "godaddy", "bluehost", "cloudflare"} {
		services := cat.Services(p)
		if len(services) == 0 {
			t.Fatalf("%s has no services", p)
		}
		for _, s := range services {
			fields, err := cat.Fields(p, s)
			if err != nil {
				t.Fatal(err)
			}
			if len(fields) == 0 {
				t.Fatalf("%s/%s has no fields", p, s)
			}
			var secret bool
			for _, f := range fields {
				if f.Secret {
					secret = true
				}
			}
			if !secret {
				t.Errorf("%s/%s stores nothing secret", p, s)
			}
		}
	}
}

func TestFieldsRejectsUnknownProviderAndService(t *testing.T) {
	cat, _ := LoadCatalog("")
	if _, err := cat.Fields("nope", "api"); err == nil {
		t.Error("expected unknown provider to fail")
	}
	if _, err := cat.Fields("google", "nope"); err == nil {
		t.Error("expected unknown service to fail")
	}
}

func TestOverrideDirectoryReplacesAProvider(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "custom.json"), []byte(
		`{"housecallpro":{"title":"HCP","services":{"api":{"fields":[{"label":"token","secret":true}]}}}}`), 0o600)
	cat, err := LoadCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := cat.Fields("housecallpro", "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Label != "token" {
		t.Fatalf("override did not replace the built-in: %+v", fields)
	}
	if len(cat.Services("google")) == 0 {
		t.Error("override wiped unrelated providers")
	}
}

func TestMissingOverrideDirectoryIsNotAnError(t *testing.T) {
	if _, err := LoadCatalog(filepath.Join(t.TempDir(), "absent")); err != nil {
		t.Fatal(err)
	}
}

func TestTitleAndReference(t *testing.T) {
	item := NewItem{Provider: "Housecall Pro", Service: "API"}
	if item.Title() != "housecall pro api" {
		t.Fatalf("title %q", item.Title())
	}
	c := Created{Title: "housecallpro api", VaultName: "wtp"}
	if got := c.Reference("base url"); got != "op://wtp/housecallpro api/base url" {
		t.Fatalf("reference %q", got)
	}
	// The references this produces must survive the project's own validator.
	if err := ValidateReference(c.Reference("base url")); err != nil {
		t.Fatalf("a reference Lucky itself produced is invalid: %v", err)
	}
}

func TestValidateRequiresNonOptionalFields(t *testing.T) {
	cat, _ := LoadCatalog("")
	fields, _ := cat.Fields("housecallpro", "api")
	item := NewItem{VaultID: "v", Provider: "housecallpro", Service: "api", Fields: fields, Values: map[string]string{}}
	if err := item.Validate(); err == nil {
		t.Fatal("expected a missing required field to fail")
	}
	item.Values["credential"] = "abc"
	if err := item.Validate(); err != nil {
		t.Fatalf("optional fields should not be required: %v", err)
	}
}

func TestProfileItemIsNotSecretAndTitlesConsistently(t *testing.T) {
	item := ProfileItem("v1", "wtp", map[string]string{"business name": "We The Plumbers", "domain": "x.com", "phone": "1", "email": "a@b.c", "city": "Conroe", "state": "TX", "postal code": "77301"})
	if item.Title() != "cosmic profile" {
		t.Fatalf("title %q", item.Title())
	}
	for _, f := range item.Fields {
		if f.Secret {
			t.Errorf("profile field %q is marked secret; the profile is not secret", f.Label)
		}
	}
	if err := item.Validate(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(item.Notes, "customer boundary") {
		t.Error("profile should record why it lives in the vault")
	}
}

func TestVaultTitleIsASafeReferenceSegment(t *testing.T) {
	for in, want := range map[string]string{
		"We The Plumbers":     "we-the-plumbers",
		"  Smith & Sons, LLC": "smith-sons-llc",
		"ACME":                "acme",
	} {
		if got := (NewVault{Customer: in}).Title(); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	if err := (NewVault{Customer: "  "}).Validate(); err == nil {
		t.Error("a blank customer name must be rejected")
	}
	// Whatever it produces has to survive the reference validator, because it
	// becomes the first segment of every op:// reference for that customer.
	v := NewVault{Customer: "Smith & Sons, LLC"}
	if err := ValidateReference("op://" + v.Title() + "/item/field"); err != nil {
		t.Fatalf("vault title produces an invalid reference: %v", err)
	}
}
