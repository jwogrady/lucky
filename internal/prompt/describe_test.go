package prompt

import (
	"strings"
	"testing"
)

// The failure this exists to catch, quoted from the wtp .env.op: a
// double-quoted JSON value resolves to one character, `{`, and nothing errors.
func TestDescribeCallsOutTheTruncatedJSONValue(t *testing.T) {
	got := Describe("credential", "{")
	if !strings.Contains(got, "TRUNCATED") {
		t.Fatalf("a one-character JSON value must be called out, got %q", got)
	}
}

func TestDescribeIdentifiesAGoodServiceAccountWithoutRevealingIt(t *testing.T) {
	blob := `{"type":"service_account","project_id":"wtp","client_email":"reader@wtp.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----MIIEvQIBADAN"}`
	got := Describe("credential", blob)
	if !strings.Contains(got, "valid JSON") || !strings.Contains(got, "reader@wtp.iam.gserviceaccount.com") {
		t.Fatalf("should confirm it parses and name the identity: %q", got)
	}
	if strings.Contains(got, "MIIEvQIBADAN") || strings.Contains(got, "BEGIN PRIVATE KEY") {
		t.Fatalf("leaked key material: %q", got)
	}
}

func TestDescribeNeverPrintsMoreThanTheLastFourCharacters(t *testing.T) {
	secret := "super-secret-api-key-value-9876"
	got := Describe("credential", secret)
	if strings.Contains(got, "super-secret") {
		t.Fatalf("leaked the secret: %q", got)
	}
	if !strings.Contains(got, "9876") || !strings.Contains(got, "31 characters") {
		t.Fatalf("should give length and a short tail to verify a paste: %q", got)
	}
}

func TestDescribePrivateKeyReportsShapeOnly(t *testing.T) {
	key := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\n-----END OPENSSH PRIVATE KEY-----"
	got := Describe("private key", key)
	if strings.Contains(got, "b3Blbn") {
		t.Fatalf("leaked key material: %q", got)
	}
	if !strings.Contains(got, "3 lines") {
		t.Fatalf("should report line count: %q", got)
	}
}

func TestDescribeEmpty(t *testing.T) {
	if got := Describe("x", ""); !strings.Contains(got, "empty") {
		t.Fatalf("got %q", got)
	}
}
