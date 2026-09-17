package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionHintAppearsOnlyWhileMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("SHELL", "/bin/bash")

	hint := completionHint()
	if !strings.Contains(hint, "lucky completion bash") {
		t.Fatalf("expected a bash install line, got %q", hint)
	}

	dir := filepath.Join(home, ".local", "share", "bash-completion", "completions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lucky"), []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Installed: it has to go quiet, or it trains the operator to skip
	// everything Lucky says.
	if hint := completionHint(); hint != "" {
		t.Fatalf("hint should stop once installed, got %q", hint)
	}
}

// A shell Lucky does not know gets silence, not a file its shell will never
// read.
func TestCompletionHintSkipsUnknownShells(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHELL", "/usr/bin/nushell")
	if hint := completionHint(); hint != "" {
		t.Fatalf("expected no hint, got %q", hint)
	}
}

func TestCompletionHintKnowsZshAndFish(t *testing.T) {
	for shell, want := range map[string]string{
		"/bin/zsh":  "_lucky",
		"/bin/fish": "lucky.fish",
	} {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("SHELL", shell)
		if hint := completionHint(); !strings.Contains(hint, want) {
			t.Fatalf("%s: expected %q in %q", shell, want, hint)
		}
	}
}
