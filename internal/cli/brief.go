package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jwogrady/lucky/credential"
	"github.com/spf13/cobra"
)

// brief is what `lucky` with no arguments says.
//
// The default was cobra's help: thirteen commands in alphabetical order, which
// tells an operator what Lucky can do and nothing about what Lucky currently
// has. The first question on picking the tool up is not "what are the verbs",
// it is "are you working, what are you holding, and what needs doing" — and
// that gets asked far more often than any single command gets run.
//
// The voice is deliberate and it is confined to the labels. Every number stays
// a number and every next step stays a command you can paste; a briefing you
// have to translate is worse than a plain one, and the joke wears out long
// before the tool does.
//
// It stays cheap on purpose: one vault listing, no item reads, no vendor calls.
// A briefing that takes ten seconds is a briefing nobody waits for.
func (a App) brief(cmd *cobra.Command, account string) error {
	fmt.Fprintf(a.Out, "it's me, lucky. here's where we're at.\n\n")

	// Templates are local, so they are reported whether or not 1Password is
	// reachable. An overlay silently replaces a built-in by name, which is
	// worth seeing before it changes what `put` prompts for.
	builtin, _ := credential.LoadCatalog("")
	cat, err := a.catalog()
	if err != nil {
		fmt.Fprintf(a.Out, "the book\tcan't read it: %s\n", err)
		cat = builtin
	} else {
		extra := ""
		if n := len(cat) - len(builtin); n > 0 {
			extra = fmt.Sprintf(", %d of your own", n)
		}
		services := 0
		for _, name := range cat.Providers() {
			services += len(cat.Services(name))
		}
		fmt.Fprintf(a.Out, "the book\t%d people, %d services%s\n", len(cat), services, extra)
	}

	if v := a.defaultVault(); v != "" {
		fmt.Fprintf(a.Out, "usual spot\t%s\n", v)
	}

	client, err := a.NewClient(cmd.Context(), Config{Account: account})
	if err != nil {
		// Not being able to reach 1Password is a normal state to be in — the
		// vault is locked, or this is a new machine — and it is the operator's
		// next action rather than a crash.
		fmt.Fprintf(a.Out, "the door\tlocked: %s\n", err)
		fmt.Fprintf(a.Err, "\nsign in to 1Password and come find me.\n")
		return nil
	}
	vaults, err := client.Vaults(cmd.Context())
	if err != nil {
		fmt.Fprintf(a.Out, "the door\t%s, but nobody's listing: %s\n", client.AuthMode(), err)
		return nil
	}
	fmt.Fprintf(a.Out, "the door\t%s, %d vaults — we're in\n", client.AuthMode(), len(vaults))

	sort.Slice(vaults, func(i, j int) bool { return vaults[i].ItemCount > vaults[j].ItemCount })
	var held []credential.Vault
	var empty []string
	for _, v := range vaults {
		if v.ItemCount > 0 {
			held = append(held, v)
		} else {
			empty = append(empty, v.Title)
		}
	}
	if len(held) > 0 {
		fmt.Fprintf(a.Out, "\nwhat i'm holdin'\n")
		for i, v := range held {
			if i == 5 {
				fmt.Fprintf(a.Out, "  and %d more\n", len(held)-5)
				break
			}
			fmt.Fprintf(a.Out, "  %s\t%d items\n", v.Title, v.ItemCount)
		}
	}
	// An empty vault was invisible here, which got it exactly backwards: a
	// vault with nothing in it is usually one just created for a customer, and
	// it is the only thing on this screen with work outstanding.
	if len(empty) > 0 {
		sort.Strings(empty)
		fmt.Fprintf(a.Out, "\nnothin' in 'em yet\n  %s\n", strings.Join(empty, ", "))
	}

	// The suggested vault is the configured default or a placeholder, never a
	// guess. An earlier version picked whichever vault held the most items and
	// proposed running against "Personal" — Lucky has no way to tell a
	// customer's boundary from the operator's own drawer, and a confident wrong
	// suggestion is worse than an obvious blank.
	target := a.defaultVault()
	if target == "" {
		target = "<vault>"
	}
	next := []string{
		fmt.Sprintf("lucky inventory --vault %s\twhat they got, what they're missin'", target),
		fmt.Sprintf("lucky verify --vault %s\tsee which keys still turn", target),
	}
	next = append(next, "lucky new-customer \"Their Business\"\tbring somebody in")
	fmt.Fprintf(a.Err, "\nso whaddya need\n")
	for _, line := range next {
		fmt.Fprintf(a.Err, "  %s\n", line)
	}
	if install := completionHint(); install != "" {
		fmt.Fprintf(a.Err, "\nyou're typin' vault names by hand. let me finish 'em for you:\n  %s\n  then open a new shell.\n", install)
	}
	fmt.Fprintf(a.Err, "\nyou got any new connections for me?\n")
	return nil
}
