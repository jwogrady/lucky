package credential

import (
	"encoding/json"
	"strings"
)

// Guess is what Lucky thinks a pasted value is.
type Guess struct {
	Provider string
	Service  string
	Field    string
	Reason   string // shown to the operator, so the guess can be rejected on sight
}

// Detect inspects a value pasted from an email, a text message, or a console
// and reports what it appears to be.
//
// It is a suggestion, never an action: the operator confirms before anything is
// written. The point is not cleverness, it is that a person holding a blob of
// characters from a client's email should not also have to remember which of
// eleven providers it belongs to and which field it goes in.
//
// Detection reads structure only — prefixes, JSON keys, PEM headers. It never
// contacts a network and never logs what it saw.
func Detect(value string) (Guess, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return Guess{}, false
	}
	switch {
	case strings.HasPrefix(v, "{"):
		var parsed map[string]any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			if t, _ := parsed["type"].(string); t == "service_account" {
				reason := "JSON with type=service_account"
				if email, ok := parsed["client_email"].(string); ok {
					reason += ", client_email=" + email
				}
				return Guess{Provider: "google", Service: "service-account", Field: "credential", Reason: reason}, true
			}
		}
	case strings.Contains(v, "PRIVATE KEY-----"):
		kind := "a private key"
		if strings.Contains(v, "OPENSSH") {
			kind = "an OpenSSH private key"
		}
		return Guess{Provider: "cpanel", Service: "ssh", Field: "private key", Reason: kind + " (PEM header)"}, true
	case strings.HasPrefix(v, "ssh-rsa ") || strings.HasPrefix(v, "ssh-ed25519 "):
		return Guess{Provider: "cpanel", Service: "ssh", Field: "public key", Reason: "an SSH public key"}, true
	case strings.HasPrefix(v, "sbp_") || strings.HasPrefix(v, "sb_secret_"):
		return Guess{Provider: "supabase", Service: "project", Field: "secret key", Reason: "a Supabase secret key prefix"}, true
	case strings.HasPrefix(v, "eyJ") && strings.Count(v, ".") == 2:
		return Guess{Provider: "supabase", Service: "project", Field: "publishable key", Reason: "a JWT, which is how Supabase anon/publishable keys look"}, true
	case strings.HasPrefix(v, "AIza"):
		return Guess{Provider: "google", Service: "maps-server", Field: "credential", Reason: "an AIza… Google API key — confirm whether it is the server or browser key, they are restricted differently"}, true
	case strings.HasPrefix(v, "postgres://") || strings.HasPrefix(v, "postgresql://"):
		return Guess{Provider: "supabase", Service: "project", Field: "password", Reason: "a Postgres connection string — store the PASSWORD as the secret, not the whole URL"}, true
	}
	return Guess{}, false
}
