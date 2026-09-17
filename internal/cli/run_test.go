package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The test binary doubles as the child process so the exec path is real.
func TestMain(m *testing.M) {
	switch os.Getenv("LUCKY_TEST_HELPER") {
	case "":
		os.Exit(m.Run())
	case "print":
		fmt.Print(os.Getenv(os.Getenv("LUCKY_TEST_VAR")))
	case "exit":
		code := 0
		fmt.Sscanf(os.Getenv("LUCKY_TEST_CODE"), "%d", &code)
		os.Exit(code)
	case "touch":
		os.WriteFile(os.Getenv("LUCKY_TEST_MARKER"), []byte("ran"), 0o600)
	case "silent":
	}
	os.Exit(0)
}

// refClient resolves from a table so a test can fail selected references.
type refClient struct {
	secrets map[string]string
	newErr  error
}

func (refClient) AuthMode() string                        { return "service-account" }
func (refClient) Vaults(context.Context) ([]Vault, error) { return nil, nil }
func (refClient) Items(context.Context, string) ([]Item, error) {
	return nil, nil
}
func (c refClient) Resolve(_ context.Context, ref string) (string, error) {
	if secret, ok := c.secrets[ref]; ok {
		return secret, nil
	}
	// Mirrors opclient: the vendor message quotes the reference back.
	return "", fmt.Errorf("could not resolve secret reference: %q is not an item", ref)
}

func runApp(t *testing.T, c refClient, envFileBody string, args ...string) (*App, *bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.op")
	if err := os.WriteFile(path, []byte(envFileBody), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := &App{In: strings.NewReader(""), Out: out, Err: errOut, NewClient: func(context.Context, Config) (Client, error) {
		if c.newErr != nil {
			return nil, c.newErr
		}
		return c, nil
	}}
	full := append([]string{"run", "--env-file", path, "--"}, args...)
	return app, out, errOut, app.Run(context.Background(), full)
}

func TestRunResolvesReferencesIntoTheChildEnvironment(t *testing.T) {
	c := refClient{secrets: map[string]string{
		"op://wtp/Supabase/secret key":              "sk_live_value",
		"op://wtp/Housecall Pro API key/credential": "hcp_value",
		"op://wtp/Supabase/password":                "p4ss",
	}}
	body := `# comment
PUBLIC_SUPABASE_URL="https://example.supabase.co"
SUPABASE_SECRET_KEY="op://wtp/Supabase/secret key"
HCP_API_KEY="op://wtp/Housecall Pro API key/credential"
SUPABASE_DB_URL="postgresql://postgres.abc:op://wtp/Supabase/password@aws-0.pooler.supabase.com:5432/postgres"
`
	for _, tc := range []struct{ variable, want string }{
		{"PUBLIC_SUPABASE_URL", "https://example.supabase.co"},
		{"SUPABASE_SECRET_KEY", "sk_live_value"},
		{"HCP_API_KEY", "hcp_value"},
		{"SUPABASE_DB_URL", "postgresql://postgres.abc:p4ss@aws-0.pooler.supabase.com:5432/postgres"},
	} {
		t.Setenv("LUCKY_TEST_HELPER", "print")
		t.Setenv("LUCKY_TEST_VAR", tc.variable)
		_, out, _, err := runApp(t, c, body, os.Args[0])
		if err != nil {
			t.Fatalf("%s: %v", tc.variable, err)
		}
		if out.String() != tc.want {
			t.Fatalf("%s = %q, want %q", tc.variable, out.String(), tc.want)
		}
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	t.Setenv("LUCKY_TEST_HELPER", "exit")
	t.Setenv("LUCKY_TEST_CODE", "7")
	_, _, _, err := runApp(t, refClient{}, "A=literal\n", os.Args[0])
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected an ExitError, got %v", err)
	}
	if exit.Code != 7 {
		t.Fatalf("exit code %d, want 7", exit.Code)
	}
}

func TestRunSucceedsWithZeroExit(t *testing.T) {
	t.Setenv("LUCKY_TEST_HELPER", "silent")
	if _, _, _, err := runApp(t, refClient{}, "A=literal\n", os.Args[0]); err != nil {
		t.Fatal(err)
	}
}

func TestRunDoesNotExecWhenAReferenceFails(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	t.Setenv("LUCKY_TEST_HELPER", "touch")
	t.Setenv("LUCKY_TEST_MARKER", marker)
	c := refClient{secrets: map[string]string{"op://wtp/good/field": "ok"}}
	body := "GOOD=\"op://wtp/good/field\"\nBAD=\"op://wtp/stale/field\"\n"
	_, _, _, err := runApp(t, c, body, os.Args[0])
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the child ran despite an unresolvable reference")
	}
}

func TestRunReportsEveryUnresolvableReference(t *testing.T) {
	t.Setenv("LUCKY_TEST_HELPER", "silent")
	c := refClient{secrets: map[string]string{"op://wtp/good/field": "s3cr3t_value"}}
	body := `GOOD="op://wtp/good/field"
STALE_ONE="op://wtp/stale-one/field"
STALE_TWO="op://wtp/stale-two/field"
STALE_THREE="op://wtp/stale-three/field"
`
	_, _, _, err := runApp(t, c, body, os.Args[0])
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	for _, key := range []string{"STALE_ONE", "STALE_TWO", "STALE_THREE"} {
		if !strings.Contains(message, key) {
			t.Fatalf("failure report omits %s: %s", key, message)
		}
	}
	if strings.Contains(message, "GOOD") {
		t.Fatalf("failure report names a key that resolved: %s", message)
	}
	if strings.Contains(message, "s3cr3t_value") {
		t.Fatalf("failure report leaked a resolved value: %s", message)
	}
	for _, ref := range []string{"op://wtp/stale-one/field", "op://wtp/stale-two/field"} {
		if strings.Contains(message, ref) {
			t.Fatalf("failure report leaked a reference: %s", message)
		}
	}
	if !strings.Contains(message, "[REDACTED]") {
		t.Fatalf("expected redaction in: %s", message)
	}
}

func TestRunWritesNoSecretToLuckyOutput(t *testing.T) {
	t.Setenv("LUCKY_TEST_HELPER", "silent")
	c := refClient{secrets: map[string]string{"op://wtp/item/field": "t0p-s3cr3t"}}
	_, out, errOut, err := runApp(t, c, "SECRET=\"op://wtp/item/field\"\n", os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "t0p-s3cr3t") || strings.Contains(errOut.String(), "t0p-s3cr3t") {
		t.Fatalf("secret reached Lucky's own output stdout=%q stderr=%q", out.String(), errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("run wrote to stdout: %q", out.String())
	}
}

// A file of literals is a credential-free operation, so Lucky must not open a
// 1Password session for it.
func TestRunWithOnlyLiteralsNeedsNoClient(t *testing.T) {
	t.Setenv("LUCKY_TEST_HELPER", "print")
	t.Setenv("LUCKY_TEST_VAR", "BOOKING_NOTIFY_FROM")
	c := refClient{newErr: errors.New("1Password must not be contacted here")}
	body := "BOOKING_NOTIFY_FROM=\"info@wetheplumberstx.com\"\n"
	_, out, _, err := runApp(t, c, body, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "info@wetheplumberstx.com" {
		t.Fatalf("got %q", out.String())
	}
}

func TestRunEnvFileOverridesTheParentEnvironment(t *testing.T) {
	t.Setenv("LUCKY_TEST_HELPER", "print")
	t.Setenv("LUCKY_TEST_VAR", "LUCKY_OVERRIDE_ME")
	t.Setenv("LUCKY_OVERRIDE_ME", "from-parent")
	c := refClient{secrets: map[string]string{"op://wtp/item/field": "from-env-file"}}
	_, out, _, err := runApp(t, c, "LUCKY_OVERRIDE_ME=\"op://wtp/item/field\"\n", os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "from-env-file" {
		t.Fatalf("got %q, want the env-file value to win", out.String())
	}
}

func TestRunRequiresEnvFileAndCommand(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := &App{Out: out, Err: errOut, NewClient: func(context.Context, Config) (Client, error) { return refClient{}, nil }}
	if err := app.Run(context.Background(), []string{"run", "--", "true"}); err == nil {
		t.Fatal("expected --env-file to be required")
	}
	if err := app.Run(context.Background(), []string{"run", "--env-file", "x"}); err == nil {
		t.Fatal("expected a command to be required")
	}
}

// TestRunBinaryEndToEnd exercises the real lucky binary. A literals-only env
// file needs no 1Password, so the whole path -- flag parsing, parsing, exec,
// exit status -- runs against the shipped artifact.
func TestRunBinaryEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "lucky")
	build := exec.Command("go", "build", "-o", binary, "github.com/jwogrady/lucky/cmd/lucky")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}
	envFile := filepath.Join(dir, ".env.op")
	body := "# literals only\nBOOKING_NOTIFY_TO=\"john@status26.com\"\nPUBLIC_SUPABASE_URL=\"https://example.supabase.co\"\n"
	if err := os.WriteFile(envFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	script := `printf '%s|%s' "$BOOKING_NOTIFY_TO" "$PUBLIC_SUPABASE_URL"; exit 3`
	cmd := exec.Command(binary, "run", "--env-file", envFile, "--", "sh", "-c", script)
	cmd.Env = append(os.Environ(), "OP_SERVICE_ACCOUNT_TOKEN=", "LUCKY_OP_ACCOUNT=")
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("exit status = %v (stderr %q), want 3", err, stderr.String())
	}
	if got := stdout.String(); got != "john@status26.com|https://example.supabase.co" {
		t.Fatalf("child environment = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("lucky wrote to stderr: %q", stderr.String())
	}
}
