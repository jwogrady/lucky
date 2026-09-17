package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// completionHint tells an operator that tab completion exists, and only while
// it is missing.
//
// Cobra generates a completion script for free and almost nobody installs one,
// because nothing ever mentions it: `lucky completion` sits in the command list
// looking like plumbing. That matters more here than in most tools. The things
// a person types into Lucky are vault titles, provider names and service names
// — exactly the strings that are easy to get slightly wrong, and a wrong vault
// name is the difference between "not accessible" and filing a customer's
// credential in the wrong boundary.
//
// It goes quiet as soon as the file exists. A hint that keeps appearing after
// it has been acted on is a hint people learn to read past, and then the next
// thing Lucky says gets read past too.
func completionHint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	shell := filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}

	var paths []string
	var install string
	switch shell {
	case "bash":
		dir := filepath.Join(data, "bash-completion", "completions")
		paths = []string{
			filepath.Join(dir, "lucky"),
			"/etc/bash_completion.d/lucky",
			"/usr/share/bash-completion/completions/lucky",
		}
		install = fmt.Sprintf("mkdir -p %s && lucky completion bash > %s/lucky", dir, dir)
	case "zsh":
		dir := filepath.Join(home, ".zfunc")
		paths = []string{
			filepath.Join(dir, "_lucky"),
			"/usr/share/zsh/site-functions/_lucky",
			filepath.Join(data, "zsh", "site-functions", "_lucky"),
		}
		install = fmt.Sprintf("mkdir -p %s && lucky completion zsh > %s/_lucky", dir, dir)
	case "fish":
		dir := filepath.Join(home, ".config", "fish", "completions")
		paths = []string{filepath.Join(dir, "lucky.fish")}
		install = fmt.Sprintf("mkdir -p %s && lucky completion fish > %s/lucky.fish", dir, dir)
	default:
		// An unrecognised shell gets nothing rather than a guess that sends
		// someone to write a file their shell will never read.
		return ""
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return ""
		}
	}
	return install
}
