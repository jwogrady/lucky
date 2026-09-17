// Package prompt reads credentials from a terminal without echoing them and
// without ever writing them anywhere but memory.
//
// It degrades deliberately: when stdin is not a terminal — a pipe, a test, CI —
// every read still works, echo suppression simply has nothing to suppress. That
// keeps `lucky put` scriptable without a second code path.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type Prompter struct {
	In  io.Reader
	Out io.Writer
	r   *bufio.Reader
}

func New(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{In: in, Out: out, r: bufio.NewReader(in)}
}

// tty reports the file descriptor to suppress echo on, and whether there is
// one. A piped or in-test stdin is not a terminal and needs no suppression.
func (p *Prompter) tty() (int, bool) {
	f, ok := p.In.(*os.File)
	if !ok {
		return 0, false
	}
	fd := int(f.Fd())
	return fd, term.IsTerminal(fd)
}

func (p *Prompter) label(label, help string, optional bool) {
	suffix := ""
	if optional {
		suffix = " (optional, Enter to skip)"
	}
	if help != "" {
		fmt.Fprintf(p.Out, "  %s%s\n    %s\n  > ", label, suffix, help)
		return
	}
	fmt.Fprintf(p.Out, "  %s%s\n  > ", label, suffix)
}

// Line reads one visible line.
func (p *Prompter) Line(label, help string, optional bool) (string, error) {
	p.label(label, help, optional)
	s, err := p.r.ReadString('\n')
	if err != nil && (err != io.EOF || s == "") {
		if err == io.EOF {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

// Secret reads one line without echoing it.
func (p *Prompter) Secret(label, help string, optional bool) (string, error) {
	p.label(label, help, optional)
	fd, isTTY := p.tty()
	if !isTTY {
		s, err := p.r.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return strings.TrimRight(s, "\r\n"), nil
	}
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(p.Out)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

// Multiline reads until EOF. Terminal echo stays ON here: a pasted private key
// or service-account blob is many lines, and a person pasting into a silent
// terminal cannot tell whether anything arrived. The value is confirmed by
// Describe afterwards rather than by hiding it during entry.
func (p *Prompter) Multiline(label, help string, optional bool) (string, error) {
	if help == "" {
		help = "paste the value, then press Ctrl-D on a new line"
	}
	p.label(label, help, optional)
	fmt.Fprintln(p.Out)
	var b strings.Builder
	if _, err := io.Copy(&b, p.r); err != nil {
		return "", err
	}
	return strings.TrimRight(b.String(), "\r\n"), nil
}

// Confirm asks a yes/no question, defaulting to no.
func (p *Prompter) Confirm(question string) (bool, error) {
	fmt.Fprintf(p.Out, "%s [y/N] ", question)
	s, err := p.r.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes", nil
}

// Choose presents a numbered menu and returns the chosen index.
func (p *Prompter) Choose(title string, options []string) (int, error) {
	fmt.Fprintf(p.Out, "%s\n", title)
	for i, o := range options {
		fmt.Fprintf(p.Out, "  %d) %s\n", i+1, o)
	}
	for {
		fmt.Fprint(p.Out, "  > ")
		s, err := p.r.ReadString('\n')
		if err != nil && err != io.EOF {
			return 0, err
		}
		s = strings.TrimSpace(s)
		for i, o := range options {
			if s == o {
				return i, nil
			}
			if s == fmt.Sprint(i+1) {
				return i, nil
			}
		}
		if err == io.EOF {
			return 0, fmt.Errorf("no selection")
		}
		fmt.Fprintf(p.Out, "  not one of the options\n")
	}
}
