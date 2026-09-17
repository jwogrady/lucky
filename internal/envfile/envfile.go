// Package envfile parses the env files that carry op:// secret references and
// locates the references inside their values.
//
// The format of record is cosmic-wtp's .env.op. It is an `op inject` template,
// not merely an `op run --env-file` file: one of its values embeds a reference
// inside a larger string (a Postgres URL). `op run` resolves whole-value
// references only and passes an embedded one through untouched, so matching
// `op run` alone would hand that consumer a literal "op://..." inside its
// connection string and fail quietly. This package follows `op inject`, which
// is a superset: a whole-value reference behaves exactly as `op run` resolves it.
package envfile

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Entry is one KEY=VALUE assignment. Line is retained so a failure can name the
// line the operator has to go and fix.
type Entry struct {
	Key   string
	Value string
	Line  int
}

const scheme = "op://"

// terminators end a reference. Space is deliberately absent: 1Password item and
// field labels contain spaces ("op://wtp/Housecall Pro API key/credential") and
// the file of record depends on that. The set was established against
// `op inject` rather than assumed; '/' separates segments and never terminates.
const terminators = "@:,;%&(){}[]<>|!*+~^#`\\\"'\t\n"

// Parse reads assignments, skipping blank lines and whole-line comments.
func Parse(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(text, "export "), "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", line)
		}
		key = strings.TrimSpace(key)
		if !validKey(key) {
			return nil, fmt.Errorf("line %d: %q is not a valid variable name", line, key)
		}
		entries = append(entries, Entry{Key: key, Value: unquote(value), Line: line})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		digit := r >= '0' && r <= '9'
		if !(r == '_' || digit || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
		if i == 0 && digit {
			return false
		}
	}
	return true
}

// unquote strips one layer of quoting. A quoted value ends at the next matching
// quote with no escape processing, which is why the file of record wraps the
// service-account JSON in single quotes: inside double quotes its first inner
// quote would end the value one character in, silently.
func unquote(raw string) string {
	value := strings.TrimLeft(raw, " \t")
	if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
		if end := strings.IndexByte(value[1:], value[0]); end >= 0 {
			return value[1 : 1+end]
		}
		return value[1:]
	}
	if hash := strings.Index(value, " #"); hash >= 0 { // inline comment
		value = value[:hash]
	}
	return strings.TrimRight(value, " \t")
}

// span locates the next reference at or after from.
func span(value string, from int) (start, end int, ok bool) {
	i := strings.Index(value[from:], scheme)
	if i < 0 {
		return 0, 0, false
	}
	start = from + i
	rest := value[start+len(scheme):]
	end = len(value)
	if j := strings.IndexAny(rest, terminators); j >= 0 {
		end = start + len(scheme) + j
	}
	for end > start && value[end-1] == ' ' { // trailing space is surrounding text
		end--
	}
	return start, end, true
}

// Refs returns every reference in value, in order, including duplicates.
func Refs(value string) []string {
	var refs []string
	for i := 0; ; {
		start, end, ok := span(value, i)
		if !ok {
			return refs
		}
		refs = append(refs, value[start:end])
		i = end
	}
}

// Expand substitutes each reference with its resolved secret. A reference
// missing from resolved would expand to nothing, so callers must resolve every
// reference Refs reports before calling this.
func Expand(value string, resolved map[string]string) string {
	var b strings.Builder
	for i := 0; ; {
		start, end, ok := span(value, i)
		if !ok {
			b.WriteString(value[i:])
			return b.String()
		}
		b.WriteString(value[i:start])
		b.WriteString(resolved[value[start:end]])
		i = end
	}
}
