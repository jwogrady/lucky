package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Describe summarises a value the operator just pasted, WITHOUT revealing it,
// so a confirmation screen can be reviewed safely over a shared screen.
//
// This exists because of a failure this project has already been bitten by. The
// wtp .env.op records it:
//
//	inside a double-quoted value the parser ends the string at that first inner
//	quote and the variable resolves to one character: `{`. Nothing errors.
//
// A truncated service-account blob is indistinguishable from a good one if all
// you see is that "something was entered". Reporting the length, and for JSON
// the non-secret identifying fields, turns a silent corruption into an obvious
// one at the moment of entry rather than three systems downstream.
func Describe(label, value string) string {
	n := len([]rune(value))
	switch {
	case value == "":
		return fmt.Sprintf("%s: (empty)", label)
	case looksLikeJSON(value):
		return fmt.Sprintf("%s: %s", label, describeJSON(value, n))
	case strings.Contains(value, "PRIVATE KEY"):
		lines := strings.Count(value, "\n") + 1
		return fmt.Sprintf("%s: private key, %d lines, %d characters", label, lines, n)
	default:
		return fmt.Sprintf("%s: %d characters, ends %q", label, n, tail(value))
	}
}

func looksLikeJSON(v string) bool {
	return strings.HasPrefix(strings.TrimSpace(v), "{")
}

func describeJSON(value string, n int) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		if n <= 2 {
			return fmt.Sprintf("%d characters — this is a TRUNCATED JSON value, not a credential (check the quoting)", n)
		}
		return fmt.Sprintf("%d characters, but it does NOT parse as JSON (%v)", n, err)
	}
	// Identifying fields of a Google service account: not secret, and exactly
	// what tells you whether the right key was pasted.
	var parts []string
	for _, k := range []string{"type", "client_email", "project_id"} {
		if s, ok := parsed[k].(string); ok && s != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, s))
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("valid JSON, %d keys, %d characters", len(parsed), n)
	}
	return fmt.Sprintf("valid JSON, %d characters, %s", n, strings.Join(parts, " "))
}

func tail(v string) string {
	r := []rune(v)
	if len(r) <= 4 {
		return strings.Repeat("*", len(r))
	}
	return "…" + string(r[len(r)-4:])
}
