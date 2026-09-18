package credential

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// normalize reduces a title to comparable letters and digits.
func normalize(s string) string {
	return nonAlnum.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "")
}

// Match is what the vault appears to hold for one templated service.
type Match struct {
	Provider string
	Service  string
	ItemID   string
	ItemName string // the real title in the vault, not the convention
	Exact    bool   // true when the title already follows provider+service
}

// MatchItems pairs templated services against the items actually in a vault.
//
// It matches loosely on purpose. A vault that predates any convention names
// things the way a person named them — "Housecall Pro API key", not
// "housecallpro api" — and the wtp .env.op is explicit about this:
//
//	They do not follow a single convention because the vault groups by system
//	rather than by variable; follow the vault, not a guess.
//
// An earlier version of this compared exact titles and reported eight existing
// credentials as missing, which is the worst failure available to this command:
// it would send an operator to reissue keys that already work. Reporting a
// loose match that a person can reject costs nothing. Reporting a false absence
// costs a rotation.
func (c Catalog) MatchItems(items []Item) (matched []Match, unmatched []Item) {
	claimed := map[string]bool{}
	for _, provider := range c.Providers() {
		template := c[provider]
		services := c.Services(provider)
		providerKeys := []string{normalize(provider), normalize(template.Title)}
		for _, service := range services {
			want := normalize(provider + " " + service)
			serviceTokens := strings.FieldsFunc(service, func(r rune) bool { return r == '-' || r == '_' })
			var best *Match
			for i := range items {
				title := normalize(items[i].Title)
				if title == "" {
					continue
				}
				var providerHit bool
				for _, key := range providerKeys {
					if key != "" && strings.Contains(title, key) {
						providerHit = true
					}
				}
				if !providerHit {
					continue
				}
				exact := title == want
				serviceHit := exact || len(services) == 1
				if !serviceHit {
					serviceHit = true
					for _, tok := range serviceTokens {
						if !strings.Contains(title, normalize(tok)) {
							serviceHit = false
						}
					}
				}
				if !serviceHit {
					continue
				}
				m := Match{Provider: provider, Service: service, ItemID: items[i].ID, ItemName: items[i].Title, Exact: exact}
				if best == nil || (m.Exact && !best.Exact) {
					best = &m
				}
			}
			if best != nil {
				matched = append(matched, *best)
				claimed[best.ItemID] = true
			}
		}
	}
	for _, item := range items {
		if !claimed[item.ID] {
			unmatched = append(unmatched, item)
		}
	}
	return matched, unmatched
}

// Missing lists templated services with nothing resembling them in the vault.
func (c Catalog) Missing(matched []Match) []string {
	have := map[string]bool{}
	for _, m := range matched {
		have[m.Provider+" "+m.Service] = true
	}
	var out []string
	for _, provider := range c.Providers() {
		for _, service := range c.Services(provider) {
			if !have[provider+" "+service] {
				out = append(out, provider+" "+service)
			}
		}
	}
	return out
}
