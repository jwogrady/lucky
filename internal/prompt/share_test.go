package prompt_test

import (
	"io"
	"strings"
	"testing"

	"github.com/jwogrady/lucky/internal/prompt"
)

// A Prompter buffers, so it reads ahead past the line it was asked for. That is
// invisible at a terminal, where a person types one line at a time and there is
// nothing to read ahead into — and it is why a command that asks in two places
// worked by hand and dropped everything after the first question when the
// answers arrived down a pipe.
//
// The package promises the opposite in its doc comment: piped, scripted and
// test input are meant to work through the same code path as a terminal. So a
// run shares one Prompter, and this is the test that says why.
func TestOnePrompterReadsTheWholeStream(t *testing.T) {
	in := strings.NewReader("one\ntwo\nthree\nfour\n")
	p := prompt.New(in, io.Discard)

	var got []string
	for range 4 {
		v, err := p.Line("line", "", true)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}

	want := []string{"one", "two", "three", "four"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("read %d = %q, want %q (full read: %q)", i, got[i], want[i], got)
		}
	}
}

// The defect, stated directly: two Prompters over one reader starve the second.
// Nothing in the CLI may do this, which is what App.prompter() exists to
// prevent. If this ever stops failing, bufio is no longer reading ahead and the
// sharing in App could be reconsidered — until then it is load-bearing.
func TestSecondPrompterOverTheSameReaderIsStarved(t *testing.T) {
	in := strings.NewReader("one\ntwo\nthree\nfour\n")

	first := prompt.New(in, io.Discard)
	if _, err := first.Line("first", "", true); err != nil {
		t.Fatal(err)
	}

	second := prompt.New(in, io.Discard)
	v, err := second.Line("second", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if v != "" {
		t.Skipf("a second prompter now reads %q; read-ahead may have changed", v)
	}
}
