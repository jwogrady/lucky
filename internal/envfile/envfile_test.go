package envfile

import (
	"strings"
	"testing"
)

func parse(t *testing.T, text string) []Entry {
	t.Helper()
	entries, err := Parse(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return entries
}

func TestParseSkipsCommentsAndBlankLines(t *testing.T) {
	entries := parse(t, "# a comment\n\n   \n  # indented comment\nA=1\n\nB=2\n")
	if len(entries) != 2 {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	if entries[0].Key != "A" || entries[0].Value != "1" || entries[0].Line != 5 {
		t.Fatalf("unexpected first entry %+v", entries[0])
	}
	if entries[1].Line != 7 {
		t.Fatalf("unexpected line %d", entries[1].Line)
	}
}

func TestParseQuoting(t *testing.T) {
	// Single quotes are load-bearing in the file of record: the service-account
	// value is JSON whose inner double quotes must survive intact.
	const jsonValue = `{"type":"service_account","project_id":"x"}`
	entries := parse(t, "D=\"double quoted\"\nS='"+jsonValue+"'\nU=unquoted value\nC=literal # trailing comment\nH=has#hash\n")
	want := map[string]string{
		"D": "double quoted",
		"S": jsonValue,
		"U": "unquoted value",
		"C": "literal",
		"H": "has#hash",
	}
	for _, e := range entries {
		if want[e.Key] != e.Value {
			t.Fatalf("%s = %q, want %q", e.Key, e.Value, want[e.Key])
		}
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries", len(entries))
	}
}

func TestParseDoubleQuotedJSONTruncatesAsTheRealParserDoes(t *testing.T) {
	// Documents why the file of record single-quotes that value: in double
	// quotes it ends at the first inner quote, yielding one character.
	entries := parse(t, `J="{"type":"service_account"}"`+"\n")
	if entries[0].Value != "{" {
		t.Fatalf("got %q, want %q", entries[0].Value, "{")
	}
}

func TestParseExportPrefix(t *testing.T) {
	if got := parse(t, "export A=1\n")[0]; got.Key != "A" || got.Value != "1" {
		t.Fatalf("unexpected entry %+v", got)
	}
}

func TestParseMalformed(t *testing.T) {
	for _, text := range []string{"NOT_AN_ASSIGNMENT\n", "=novalue\n", "BAD KEY=1\n", "1LEADINGDIGIT=1\n"} {
		if _, err := Parse(strings.NewReader(text)); err == nil {
			t.Fatalf("expected an error for %q", text)
		}
	}
}

func TestRefs(t *testing.T) {
	cases := []struct {
		name, value string
		want        []string
	}{
		{"whole value", "op://wtp/Supabase/secret key", []string{"op://wtp/Supabase/secret key"}},
		{"spaces in item and field", "op://wtp/Housecall Pro API key/credential", []string{"op://wtp/Housecall Pro API key/credential"}},
		{"literal", "https://sxzclqyavmfddfsaoutg.supabase.co", nil},
		{"literal email", "info@wetheplumberstx.com", nil},
		{"section form", "op://wtp/item/section/field", []string{"op://wtp/item/section/field"}},
		{
			"embedded in a postgres url",
			"postgresql://postgres.abc:op://wtp/Supabase/password@aws-0-us-east-1.pooler.supabase.com:5432/postgres",
			[]string{"op://wtp/Supabase/password"},
		},
		{"two in one value", "user=op://v/i/user;pass=op://v/i/pass", []string{"op://v/i/user", "op://v/i/pass"}},
		// Space continues a reference so that field labels may contain spaces.
		// The cost is that trailing prose after a reference is swallowed; the
		// file of record never does that, and `op inject` behaves the same way.
		{"space continues the reference", "op://v/i/field and more", []string{"op://v/i/field and more"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Refs(c.value)
			if len(got) != len(c.want) {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %q, want %q", got, c.want)
				}
			}
		})
	}
}

func TestExpand(t *testing.T) {
	resolved := map[string]string{
		"op://wtp/Supabase/password": "p4ss",
		"op://wtp/Supabase/secret":   "sk_live",
	}
	cases := []struct{ value, want string }{
		{"op://wtp/Supabase/secret", "sk_live"},
		{"https://example.supabase.co", "https://example.supabase.co"},
		{
			"postgresql://postgres.abc:op://wtp/Supabase/password@aws-0.pooler.supabase.com:5432/postgres",
			"postgresql://postgres.abc:p4ss@aws-0.pooler.supabase.com:5432/postgres",
		},
	}
	for _, c := range cases {
		if got := Expand(c.value, resolved); got != c.want {
			t.Fatalf("Expand(%q) = %q, want %q", c.value, got, c.want)
		}
	}
}
