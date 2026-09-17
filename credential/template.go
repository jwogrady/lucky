package credential

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed providers/providers.json
var builtinProviders []byte

// A Template is what Lucky knows about one provider: which fields a credential
// from them actually has, which of those are secret, what the endpoint and auth
// header look like, and where in their console a person goes to get one.
//
// This is the part of Lucky that replaces a person. Knowing that a GoDaddy key
// comes in two halves, that Cloudflare wants a scoped token rather than the
// global key, and that a Google service account is a JSON file you must then
// grant access to in four separate products — that knowledge currently lives in
// one operator's head, and every credential that gets pasted into the wrong
// field or stored under a name nobody recognises is that knowledge failing to
// scale. A template writes it down once.
type Template struct {
	Title    string             `json:"title"`
	Docs     string             `json:"docs,omitempty"`
	Where    string             `json:"where,omitempty"`
	Services map[string]Service `json:"services"`
}

type Service struct {
	Category string      `json:"category,omitempty"`
	Fields   []FieldSpec `json:"fields"`

	// Verify is the one call that proves a credential of this shape works.
	// Optional: a service with no stanza is reported as unchecked rather than
	// guessed at.
	Verify *Verify `json:"verify,omitempty"`
}

// FieldSpec describes one prompt.
type FieldSpec struct {
	Label     string `json:"label"`
	Secret    bool   `json:"secret,omitempty"`
	Multiline bool   `json:"multiline,omitempty"`
	Optional  bool   `json:"optional,omitempty"`
	Default   string `json:"default,omitempty"`
	Help      string `json:"help,omitempty"`
}

// Catalog is the set of provider templates in effect.
type Catalog map[string]Template

// LoadCatalog returns the built-in templates, overlaid with any JSON files
// found in dir. An override file replaces the provider it names, so a customer
// with a non-standard endpoint is a file, not a code change.
func LoadCatalog(dir string) (Catalog, error) {
	c := Catalog{}
	if err := json.Unmarshal(builtinProviders, &c); err != nil {
		return nil, fmt.Errorf("built-in provider catalog is corrupt: %w", err)
	}
	if strings.TrimSpace(dir) == "" {
		return c, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		overlay := Catalog{}
		if err := json.Unmarshal(raw, &overlay); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		for name, t := range overlay {
			c[name] = t
		}
	}
	return c, nil
}

func (c Catalog) Providers() []string {
	out := make([]string, 0, len(c))
	for name := range c {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (c Catalog) Services(provider string) []string {
	t, ok := c[provider]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(t.Services))
	for name := range t.Services {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Verification returns the check for one provider/service pair, if the
// template declares one.
func (c Catalog) Verification(provider, service string) (*Verify, bool) {
	t, ok := c[provider]
	if !ok {
		return nil, false
	}
	s, ok := t.Services[service]
	if !ok || s.Verify == nil {
		return nil, false
	}
	return s.Verify, true
}

// Fields returns the prompts for one provider/service pair.
func (c Catalog) Fields(provider, service string) ([]FieldSpec, error) {
	t, ok := c[provider]
	if !ok {
		return nil, fmt.Errorf("no template for provider %q (known: %s)", provider, strings.Join(c.Providers(), ", "))
	}
	s, ok := t.Services[service]
	if !ok {
		return nil, fmt.Errorf("%s has no service %q (known: %s)", provider, service, strings.Join(c.Services(provider), ", "))
	}
	return s.Fields, nil
}
