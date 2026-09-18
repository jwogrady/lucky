package credential

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Verify is how a provider template says "here is one call that proves this
// credential works."
//
// The alternative was a function per vendor, and it was rejected on a specific
// ground: it makes every vendor's API a build-time dependency of the credential
// custodian. Lucky would then need a release to check a new provider, and a
// vendor's client library breaking would break the thing that holds the keys.
// A stanza is data. Adding a provider is an edit to providers.json, and a
// customer with a non-standard endpoint is an overlay file, exactly as it
// already is for the fields themselves.
//
// What a stanza cannot express, Lucky does not check. That is the honest
// boundary of this design and it is worth stating: proving a Google service
// account can actually reach Search Console means minting a JWT, and that is
// not one HTTP call. Those credentials get no stanza rather than a stanza that
// pretends.
type Verify struct {
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
	Expect int    `json:"expect,omitempty"`

	// Contains is a positive assertion about the body, for the APIs that
	// answer 200 to a credential they rejected.
	//
	// Google's Geocoding API is the reason this exists. A revoked key gets
	// HTTP 200 with {"status": "REQUEST_DENIED"} in the body, so a status-only
	// check reports a dead key as working — the exact failure this command is
	// meant to catch.
	Contains string `json:"contains,omitempty"`

	// Refusal declares a rejection that means the credential is correct.
	//
	// A referrer-restricted browser key is SUPPOSED to be refused when called
	// from a server. Reporting that as a failure trains an operator to ignore
	// the report, and an ignored report is worse than no report.
	Refusal *Refusal `json:"refusal,omitempty"`

	// Proves is what a pass demonstrates, in an operator's words.
	Proves string `json:"proves,omitempty"`
}

// Refusal is a rejection that confirms the credential rather than condemning it.
type Refusal struct {
	Status   int    `json:"status,omitempty"`
	Contains string `json:"contains,omitempty"`
	Means    string `json:"means,omitempty"`
}

// FieldValue is one field of a credential as verification sees it.
type FieldValue struct {
	Label  string
	Value  string
	Secret bool
}

// Values are the fields available when building one probe: what the vault item
// holds, with the template's defaults filling anything the item omits.
type Values []FieldValue

// WithDefaults returns v extended with any templated field that carries a
// default and that the item does not already provide.
//
// The Google Maps server key in the wtp vault has no "base url" field — it was
// created by a person, before any template existed. The template knows the
// endpoint. Without this, every hand-made item in a real vault is unverifiable,
// which is most of them.
func (v Values) WithDefaults(fields []FieldSpec) Values {
	out := append(Values(nil), v...)
	for _, spec := range fields {
		if spec.Default == "" {
			continue
		}
		if _, ok := out.lookup(spec.Label); ok {
			continue
		}
		out = append(out, FieldValue{Label: spec.Label, Value: spec.Default, Secret: spec.Secret})
	}
	return out
}

// baseLabels are the field names that can carry a credential's endpoint, in
// order. There is more than one because the vault already disagrees with
// itself: the Housecall Pro item calls it "base url" and the Supabase item
// calls it "url", both written before any template existed. Renaming a field
// in a customer's live vault to satisfy a matcher is the wrong direction.
var baseLabels = []string{"base url", "url", "endpoint", "host"}

func (v Values) base() (FieldValue, bool) {
	for _, label := range baseLabels {
		if f, ok := v.lookup(label); ok {
			return f, true
		}
	}
	return FieldValue{}, false
}

func (v Values) lookup(name string) (FieldValue, bool) {
	want := normalize(name)
	for _, f := range v {
		if normalize(f.Label) == want && strings.TrimSpace(f.Value) != "" {
			return f, true
		}
	}
	return FieldValue{}, false
}

// resolve finds the value for one <placeholder>.
//
// Placeholder names do not match field labels in practice, and the wtp vault is
// the proof: the Housecall Pro item's auth header reads
//
//	Authorization: Token <key>
//
// while the field holding the secret is labelled "credential". The header was
// written by a person describing the vendor's documentation; the label came
// from 1Password's item category. Requiring them to agree would fail on the
// first real credential, so this matches in three widening steps and, as a last
// resort, falls back to the item's only secret — which is unambiguous whenever
// a credential has exactly one.
func (v Values) resolve(name string) (FieldValue, bool) {
	if f, ok := v.lookup(name); ok {
		return f, true
	}
	want := normalize(name)
	if want == "" {
		return FieldValue{}, false
	}
	var partial []FieldValue
	for _, f := range v {
		if strings.TrimSpace(f.Value) == "" {
			continue
		}
		label := normalize(f.Label)
		if strings.Contains(label, want) || strings.Contains(want, label) {
			partial = append(partial, f)
		}
	}
	if len(partial) == 1 {
		return partial[0], true
	}
	var secrets []FieldValue
	for _, f := range v {
		if f.Secret && strings.TrimSpace(f.Value) != "" {
			secrets = append(secrets, f)
		}
	}
	if len(secrets) == 1 {
		return secrets[0], true
	}
	return FieldValue{}, false
}

// Labels lists the field names available, for an error message. Labels are not
// secret; values are, and none is returned here.
func (v Values) Labels() []string {
	out := make([]string, 0, len(v))
	for _, f := range v {
		if strings.TrimSpace(f.Value) != "" {
			out = append(out, f.Label)
		}
	}
	sort.Strings(out)
	return out
}

// secrets returns every secret value, for redaction.
func (v Values) secrets() []string {
	var out []string
	for _, f := range v {
		if f.Secret && strings.TrimSpace(f.Value) != "" {
			out = append(out, f.Value)
		}
	}
	return out
}

var placeholder = regexp.MustCompile(`<([^<>]+)>`)

// Header is one request header, already resolved.
type Header struct{ Name, Value string }

// Probe is one resolved HTTP request, and the secrets that must never be shown
// with it. The URL can carry a secret — Google puts the key in the query string
// — so the URL is as sensitive as the headers and is redacted the same way.
type Probe struct {
	Method  string
	URL     string
	Headers []Header

	secrets []string
}

// Redact removes every secret this probe carries from a string. Nothing derived
// from a probe — a URL in an error, a response body, a transport failure — goes
// anywhere near an operator's terminal without passing through here.
func (p Probe) Redact(s string) string {
	for _, secret := range p.secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[REDACTED]")
		}
	}
	return s
}

// BuildProbe turns a stanza plus a credential's fields into one request.
func BuildProbe(v Verify, vals Values) (Probe, error) {
	base, ok := vals.base()
	if !ok {
		return Probe{}, fmt.Errorf("no endpoint: the item has no %s field and the template supplies no default (fields: %s)",
			strings.Join(quoteEach(baseLabels), " or "), strings.Join(vals.Labels(), ", "))
	}
	p := Probe{Method: strings.ToUpper(strings.TrimSpace(v.Method)), secrets: vals.secrets()}
	if p.Method == "" {
		p.Method = "GET"
	}

	path, err := expand(v.Path, vals, true)
	if err != nil {
		return Probe{}, err
	}
	p.URL = strings.TrimRight(strings.TrimSpace(base.Value), "/") + path

	if header, ok := vals.lookup("auth header"); ok {
		form, err := expand(header.Value, vals, false)
		if err != nil {
			return Probe{}, err
		}
		// One header per line. Supabase needs two — apikey and Authorization —
		// and a field that can only hold one would have forced a Supabase
		// special case into the code, which is the thing this design exists to
		// avoid.
		for _, line := range strings.Split(form, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			name, value, found := strings.Cut(line, ":")
			if !found {
				return Probe{}, fmt.Errorf("auth header line %q is not in \"Name: value\" form", strings.TrimSpace(line))
			}
			p.Headers = append(p.Headers, Header{Name: strings.TrimSpace(name), Value: strings.TrimSpace(value)})
		}
	}
	return p, nil
}

// expand substitutes <placeholder> against the credential's fields. A value
// going into a URL is percent-encoded; one going into a header is not.
func expand(text string, vals Values, escape bool) (string, error) {
	var missing []string
	out := placeholder.ReplaceAllStringFunc(text, func(match string) string {
		name := strings.Trim(match, "<>")
		f, ok := vals.resolve(name)
		if !ok {
			missing = append(missing, name)
			return match
		}
		if escape {
			return url.QueryEscape(f.Value)
		}
		return f.Value
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("nothing in the item matches %s (fields: %s)",
			strings.Join(quoteAll(missing), ", "), strings.Join(vals.Labels(), ", "))
	}
	return out, nil
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = fmt.Sprintf("<%s>", s)
	}
	return out
}

func quoteEach(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

// Outcome is the verdict on one credential.
type Outcome struct {
	OK     bool
	Status int
	Detail string
}

// Judge decides what a response says about the credential.
//
// Order matters. An expected refusal is checked first, because it is a
// deliberate override of what the status would otherwise mean.
func (v Verify) Judge(status int, body string) Outcome {
	if r := v.Refusal; r != nil {
		statusHit := r.Status == 0 || r.Status == status
		bodyHit := r.Contains == "" || strings.Contains(body, r.Contains)
		if statusHit && bodyHit && (r.Status != 0 || r.Contains != "") {
			detail := r.Means
			if detail == "" {
				detail = "refused, as this credential should be"
			}
			return Outcome{OK: true, Status: status, Detail: detail}
		}
	}
	expect := v.Expect
	if expect == 0 {
		expect = 200
	}
	if status != expect {
		return Outcome{Status: status, Detail: fmt.Sprintf("HTTP %d: %s", status, excerpt(body))}
	}
	if v.Contains != "" && !strings.Contains(body, v.Contains) {
		// The status was right and the body says otherwise. This is the case a
		// status-only check gets wrong, so it is worth naming precisely.
		return Outcome{Status: status, Detail: fmt.Sprintf("HTTP %d but the body does not confirm it: %s", status, excerpt(body))}
	}
	detail := v.Proves
	if detail == "" {
		detail = fmt.Sprintf("HTTP %d", status)
	}
	return Outcome{OK: true, Status: status, Detail: detail}
}

// excerpt collapses a response body to one short line. A body can contain
// anything, including an echo of what was sent, so callers redact the result.
func excerpt(body string) string {
	s := strings.Join(strings.Fields(body), " ")
	if s == "" {
		return "empty response"
	}
	if len(s) > 180 {
		return s[:180] + "…"
	}
	return s
}
