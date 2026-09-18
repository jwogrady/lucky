package credential

import (
	"strings"
	"testing"
)

// Google answers 200 to a key it rejected, so these two bodies are the whole
// reason Verify has a Contains field and a Refusal that matches on text.
const (
	geocodeOK = `{ "results" : [ { "formatted_address" : "New York, NY 10001, USA" } ], "status" : "OK" }`

	geocodeReferrer = `{ "error_message" : "API keys with referer restrictions cannot be used with this API.",` +
		` "results" : [], "status" : "REQUEST_DENIED" }`

	geocodeInvalid = `{ "error_message" : "The provided API key is invalid.",` +
		` "results" : [], "status" : "REQUEST_DENIED" }`
)

func mapsVerify() Verify {
	return Verify{
		Method: "GET", Path: "/geocode/json?address=10001&key=<credential>", Expect: 200,
		Contains: `"status" : "OK"`,
		Refusal: &Refusal{
			Status: 200, Contains: "referer restrictions",
			Means: "refused server-side, as a referrer-restricted browser key should be",
		},
	}
}

// A dead key and a correctly-restricted key produce the same HTTP status and
// the same "status" value, and differ only in the reason text. Matching on
// REQUEST_DENIED alone reports a revoked key as working, which is the one
// failure this command exists to prevent.
func TestRefusalDistinguishesRestrictionFromRevocation(t *testing.T) {
	v := mapsVerify()

	if got := v.Judge(200, geocodeReferrer); !got.OK {
		t.Fatalf("a referrer-restricted key should pass: %+v", got)
	}
	got := v.Judge(200, geocodeInvalid)
	if got.OK {
		t.Fatal("an invalid key was accepted as a correct refusal")
	}
	if !strings.Contains(got.Detail, "does not confirm") {
		t.Fatalf("unhelpful detail %q", got.Detail)
	}
}

func TestJudgeBodyMustConfirmTheStatus(t *testing.T) {
	v := Verify{Expect: 200, Contains: `"status" : "OK"`, Proves: "geocoding accepted the key"}
	if got := v.Judge(200, geocodeOK); !got.OK || got.Detail != "geocoding accepted the key" {
		t.Fatalf("unexpected %+v", got)
	}
	if got := v.Judge(200, geocodeInvalid); got.OK {
		t.Fatal("200 with a rejecting body must not pass")
	}
	if got := v.Judge(401, `{"message":"Unauthorized"}`); got.OK || !strings.Contains(got.Detail, "401") {
		t.Fatalf("unexpected %+v", got)
	}
}

// An empty Refusal must not swallow every response.
func TestEmptyRefusalDoesNotMatchEverything(t *testing.T) {
	v := Verify{Expect: 200, Refusal: &Refusal{}}
	if got := v.Judge(500, "boom"); got.OK {
		t.Fatalf("a bare refusal stanza passed a 500: %+v", got)
	}
}

func housecall() Values {
	return Values{
		{Label: "credential", Value: "hcp-secret", Secret: true},
		{Label: "base url", Value: "https://api.housecallpro.com"},
		// The real wtp item says <key>; the field is called "credential".
		{Label: "auth header", Value: "Authorization: Token <key>"},
	}
}

// The placeholder in a real vault does not match the field label. This is taken
// from the wtp vault, not invented.
func TestPlaceholderFallsBackToTheOnlySecret(t *testing.T) {
	p, err := BuildProbe(Verify{Path: "/api/price_book/services?page_size=1"}, housecall())
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://api.housecallpro.com/api/price_book/services?page_size=1" {
		t.Fatalf("unexpected url %q", p.URL)
	}
	if len(p.Headers) != 1 || p.Headers[0].Name != "Authorization" || p.Headers[0].Value != "Token hcp-secret" {
		t.Fatalf("unexpected headers %+v", p.Headers)
	}
	if p.Method != "GET" {
		t.Fatalf("method should default to GET, got %q", p.Method)
	}
}

func TestUnresolvablePlaceholderNamesTheFieldsWithoutValues(t *testing.T) {
	vals := Values{
		{Label: "base url", Value: "https://example.test"},
		{Label: "key", Value: "aaa", Secret: true},
		{Label: "secret", Value: "bbb", Secret: true},
	}
	_, err := BuildProbe(Verify{Path: "/x", Method: "GET"}, Values{vals[0]}.WithDefaults(nil))
	if err != nil {
		t.Fatalf("a path with no placeholder should build: %v", err)
	}
	_, err = BuildProbe(Verify{Path: "/x?t=<totally unknown>"}, vals)
	if err == nil {
		t.Fatal("expected an error naming the missing placeholder")
	}
	if !strings.Contains(err.Error(), "<totally unknown>") || !strings.Contains(err.Error(), "base url") {
		t.Fatalf("error should name the placeholder and the available labels: %v", err)
	}
	for _, secret := range []string{"aaa", "bbb"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked a secret value: %v", err)
		}
	}
}

// A secret in a query string is a secret in every error that quotes the URL.
func TestSecretInTheURLIsEscapedAndRedacted(t *testing.T) {
	vals := Values{
		{Label: "credential", Value: "a key/with+specials", Secret: true},
		{Label: "base url", Value: "https://maps.googleapis.com/maps/api/"},
	}
	p, err := BuildProbe(mapsVerify(), vals)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p.URL, "a key/with+specials") {
		t.Fatalf("secret was not percent-encoded into the url: %q", p.URL)
	}
	if !strings.Contains(p.URL, "a+key%2Fwith%2Bspecials") {
		t.Fatalf("unexpected url %q", p.URL)
	}
	if strings.Contains(p.URL, "api//geocode") {
		t.Fatalf("base url and path double-slashed: %q", p.URL)
	}
	got := p.Redact("Get " + strings.ReplaceAll(p.URL, "a+key%2Fwith%2Bspecials", "a key/with+specials") + ": timeout")
	if strings.Contains(got, "a key/with+specials") {
		t.Fatalf("redaction missed the secret: %q", got)
	}
}

// Supabase needs two headers, which is why one field holds more than one line.
func TestMultipleAuthHeaders(t *testing.T) {
	vals := Values{
		{Label: "url", Value: "https://project.supabase.co"},
		{Label: "publishable key", Value: "pub", Secret: true},
		{Label: "secret key", Value: "srv", Secret: true},
		{Label: "auth header", Value: "apikey: <secret key>\nAuthorization: Bearer <secret key>"},
	}
	p, err := BuildProbe(Verify{Path: "/rest/v1/"}, vals)
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://project.supabase.co/rest/v1/" {
		t.Fatalf("base url should come from the \"url\" field: %q", p.URL)
	}
	if len(p.Headers) != 2 {
		t.Fatalf("expected two headers, got %+v", p.Headers)
	}
	// Two secrets are present, so the single-secret fallback must not fire;
	// the exact label has to win.
	for _, h := range p.Headers {
		if !strings.Contains(h.Value, "srv") || strings.Contains(h.Value, "pub") {
			t.Fatalf("wrong secret chosen for %s: %+v", h.Name, p.Headers)
		}
	}
	if got := p.Redact("failed for srv and pub"); strings.Contains(got, "srv") || strings.Contains(got, "pub") {
		t.Fatalf("both secrets must be redacted: %q", got)
	}
}

// A hand-made vault item has no endpoint field; the template knows it.
func TestDefaultsFillWhatTheItemLacks(t *testing.T) {
	item := Values{{Label: "credential", Value: "k", Secret: true}}
	spec := []FieldSpec{
		{Label: "credential", Secret: true},
		{Label: "base url", Default: "https://maps.googleapis.com/maps/api"},
	}
	p, err := BuildProbe(mapsVerify(), item.WithDefaults(spec))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.URL, "https://maps.googleapis.com/maps/api/geocode/json") {
		t.Fatalf("unexpected url %q", p.URL)
	}
}

// An item's own value outranks the template's default: the customer's endpoint
// is the real one.
func TestItemValueBeatsTemplateDefault(t *testing.T) {
	item := Values{{Label: "base url", Value: "https://box42.example.test:2083"}}
	got := item.WithDefaults([]FieldSpec{{Label: "base url", Default: "https://ignored.test"}})
	f, ok := got.lookup("base url")
	if !ok || f.Value != "https://box42.example.test:2083" {
		t.Fatalf("template default overwrote the item: %+v", got)
	}
}

func TestMissingEndpointIsExplained(t *testing.T) {
	_, err := BuildProbe(Verify{Path: "/x"}, Values{{Label: "credential", Value: "k", Secret: true}})
	if err == nil || !strings.Contains(err.Error(), "no endpoint") {
		t.Fatalf("expected an endpoint error, got %v", err)
	}
	if strings.Contains(err.Error(), "k") && strings.Contains(err.Error(), "\"k\"") {
		t.Fatalf("error leaked the secret: %v", err)
	}
}

// Every stanza shipped in the catalog has to be usable by the code that reads
// it: a typo in providers.json should fail here, not in front of a customer.
func TestBuiltinStanzasAreWellFormed(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, provider := range cat.Providers() {
		for _, service := range cat.Services(provider) {
			v, ok := cat.Verification(provider, service)
			if !ok {
				continue
			}
			found++
			if v.Path == "" {
				t.Errorf("%s %s: stanza has no path", provider, service)
			}
			if v.Proves == "" {
				t.Errorf("%s %s: stanza does not say what a pass proves", provider, service)
			}
			fields, err := cat.Fields(provider, service)
			if err != nil {
				t.Fatal(err)
			}
			// Build with a stand-in for every templated field, which proves the
			// placeholders in the stanza resolve against the fields the
			// template actually defines.
			var vals Values
			for _, f := range fields {
				// A field's default is its real content — the auth header form
				// especially — so only fields a person fills get a stand-in.
				value := f.Default
				if value == "" {
					value = "x-" + f.Label
				}
				vals = append(vals, FieldValue{Label: f.Label, Value: value, Secret: f.Secret})
			}
			vals = vals.WithDefaults(fields)
			if _, err := BuildProbe(*v, vals); err != nil {
				t.Errorf("%s %s: %v", provider, service, err)
			}
		}
	}
	if found == 0 {
		t.Fatal("no verify stanzas in the built-in catalog")
	}
}
