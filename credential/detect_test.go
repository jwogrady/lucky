package credential

import (
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name, value, provider, service string
	}{
		{"google service account", `{"type":"service_account","client_email":"r@p.iam.gserviceaccount.com","project_id":"p"}`, "google", "service-account"},
		{"pem private key", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----", "cpanel", "ssh"},
		{"ssh public key", "ssh-ed25519 AAAAC3Nz user@host", "cpanel", "ssh"},
		{"google api key", "AIzaSyD-ExampleExampleExampleExample", "google", "maps-server"},
		{"postgres url", "postgresql://postgres.abc:pw@aws.pooler.supabase.com:5432/postgres", "supabase", "project"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, ok := Detect(c.value)
			if !ok {
				t.Fatal("not detected")
			}
			if g.Provider != c.provider || g.Service != c.service {
				t.Fatalf("got %s/%s want %s/%s", g.Provider, g.Service, c.provider, c.service)
			}
			if g.Reason == "" {
				t.Error("a guess with no stated reason cannot be judged by the operator")
			}
		})
	}
}

func TestDetectIdentifiesTheServiceAccountWithoutRevealingIt(t *testing.T) {
	g, _ := Detect(`{"type":"service_account","client_email":"reader@p.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----SECRET"}`)
	if !strings.Contains(g.Reason, "reader@p.iam.gserviceaccount.com") {
		t.Error("should name the client_email, which is how you tell which key it is")
	}
	if strings.Contains(g.Reason, "SECRET") || strings.Contains(g.Reason, "BEGIN PRIVATE KEY") {
		t.Fatalf("the reason leaked key material: %s", g.Reason)
	}
}

func TestDetectDeclinesRatherThanGuessing(t *testing.T) {
	for _, v := range []string{"", "   ", "hunter2", "just some words"} {
		if _, ok := Detect(v); ok {
			t.Errorf("%q should not produce a guess", v)
		}
	}
}
