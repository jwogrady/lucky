package credential

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// NewItem is a credential about to be stored. Values is keyed by FieldSpec
// label. It is passed by value, never logged, and never written to disk.
type NewItem struct {
	VaultID   string
	VaultName string
	Provider  string
	Service   string
	Fields    []FieldSpec
	Values    map[string]string
	Category  string
	Notes     string

	// Name titles the item directly, for a credential captured before anybody
	// knows what it is.
	//
	// Provider and service are the right title when a template was chosen, and
	// the wrong requirement when something has just arrived by voice, email,
	// photograph or paper and needs to be somewhere safe before the call ends.
	// Forcing a provider at that moment means either a wrong guess stored as
	// fact, or an operator stopping to classify while a customer waits. The
	// name they already used for it is better than both.
	Name string
}

// Title is the item's name in the vault, and the middle segment of every
// reference it yields. Provider then service, lowercase, because a vault read
// by a person groups better by who owns the door than by what one consumer's
// configuration happens to call the variable.
func (n NewItem) Title() string {
	if t := strings.TrimSpace(n.Name); t != "" {
		return strings.ToLower(t)
	}
	return strings.TrimSpace(strings.ToLower(n.Provider + " " + n.Service))
}

func (n NewItem) Validate() error {
	if strings.TrimSpace(n.VaultID) == "" {
		return fmt.Errorf("vault is required")
	}
	if strings.TrimSpace(n.Name) == "" && (strings.TrimSpace(n.Provider) == "" || strings.TrimSpace(n.Service) == "") {
		return fmt.Errorf("provider and service are required: together they are the item title")
	}
	if len(n.Fields) == 0 {
		return fmt.Errorf("no fields: %s/%s has no template", n.Provider, n.Service)
	}
	for _, f := range n.Fields {
		// A field carrying a default can always be satisfied, so it is never
		// missing — only a field with no value and no fallback is.
		if f.Optional || f.Default != "" {
			continue
		}
		if strings.TrimSpace(n.Values[f.Label]) == "" {
			return fmt.Errorf("%q is required for %s %s", f.Label, n.Provider, n.Service)
		}
	}
	return nil
}

// Created is what a vault write returns: enough to hand back references, and
// deliberately not the values that were written.
type Created struct {
	ID        string
	Title     string
	VaultName string
	Fields    []string
}

// Reference builds the op:// reference for one stored field. This is the
// product of a put: a secret is pasted once, and thereafter the operator holds
// only this string, which is safe in configuration, in Git, and in a ticket.
func (c Created) Reference(field string) string {
	return fmt.Sprintf("op://%s/%s/%s", c.VaultName, c.Title, field)
}

// Creator writes credentials. It is separate from Client so that a consumer
// which only needs to resolve secrets — which is most of them, and all of
// Collect — depends on a read-only interface and cannot write to a customer's
// vault even by mistake.
type Creator interface {
	Create(context.Context, NewItem) (Created, error)
}

// Custodian is the full surface, for operator tooling that needs all of it.
//
// Every backend implements the whole of it, asserted at compile time in each
// package. That is the normalisation Lucky exists to provide: which transport
// reached 1Password is an authentication detail, and it must not change what a
// credential does. A capability present on one backend and missing on another
// turns "can Lucky add a key to this?" into a question about the operator's
// environment, answerable only by trying it.
type Custodian interface {
	Client
	Creator
	Archiver
	Provisioner
	Inspector
	Updater
}

// ProfileFields are the customer and business details captured before any
// credential exists.
//
// This is the first thing Lucky collects, because it is what a vault is FOR. A
// vault named after a customer, holding that customer's profile alongside that
// customer's keys, is what makes "one customer Cosmic scoped to that customer's
// vault" a fact rather than a naming convention. Everything downstream — which
// vault a credential lands in, which Cosmic a collector writes to, whose data
// it is — resolves through this record.
//
// None of it is secret. It lives in the vault anyway, because the vault is the
// customer's boundary and splitting identity from authority across two systems
// is how they drift.
func ProfileFields() []FieldSpec {
	return []FieldSpec{
		// The person in charge of the vault comes first, because they are who
		// the work is actually with. Every credential in here was granted by
		// somebody, and on the day a grant is questioned the answer has to be a
		// name, an address and a number — not a business.
		//
		// Their email and mobile are separate from the business's on purpose.
		// AG Danforth Solutions publishes contactagdanforthsolutions@gmail.com
		// and its owner sends from hcpaulsen4@gmail.com; the grant arrived from
		// the personal address, so that is the address that authenticated him.
		// Collapsing the two would lose exactly the one that matters.
		{Label: "first name", Help: "the person in charge of this vault"},
		{Label: "last name"},
		{Label: "email", Help: "the address they actually write from"},
		{Label: "mobile", Help: "where a text reaches them"},

		{Label: "business name", Help: "the legal name"},
		{Label: "dba", Optional: true, Help: "trading name, if different"},
		{Label: "domain", Help: "e.g. wetheplumberstx.com"},
		{Label: "business phone", Optional: true},
		{Label: "business email", Optional: true, Help: "the published address, if different from theirs"},
		{Label: "street", Optional: true},
		{Label: "city"},
		{Label: "state"},
		{Label: "postal code"},
		{Label: "service area", Optional: true, Help: "counties or cities served, comma separated"},
		{Label: "google place id", Optional: true, Help: "from the Business Profile listing"},
		{Label: "timezone", Default: "America/Chicago", Optional: true},
	}
}

// PersonFields are the four things Lucky needs before it will hold a credential
// for somebody: who to name when a grant is questioned, and how to reach them.
//
// It is deliberately short. The full profile asks sixteen questions, which is
// the right number when somebody is being taken on and the wrong number when
// they are halfway through reading out a password. These four can be answered
// from the top of an email.
func PersonFields() []FieldSpec {
	return []FieldSpec{
		{Label: "first name", Help: "the person in charge of this vault"},
		{Label: "last name"},
		{Label: "email", Help: "the address they actually write from"},
		{Label: "mobile", Help: "where a text reaches them"},
	}
}

// ProfileTitle is the item every vault must hold before it holds a credential.
const ProfileTitle = "cosmic profile"

// ProfileItem builds the vault item holding a customer's profile.
func ProfileItem(vaultID, vaultName string, values map[string]string) NewItem {
	return NewItem{
		VaultID: vaultID, VaultName: vaultName,
		Provider: "cosmic", Service: "profile",
		Fields: ProfileFields(), Values: values,
		Notes: "Customer and business profile, and the person in charge of this vault. Non-secret; the account owns the vault and this person controls it.",
	}
}

// Archiver retires a credential without destroying it.
//
// Deletion is not offered anywhere in Lucky, deliberately. A revoked key is
// still evidence: of what was issued, to whom, and when it stopped being
// trusted. Rotation, an incident review, and a customer asking what a vendor
// ever had access to all need the record of a credential that no longer works.
// Archived items stay recoverable in 1Password; they simply leave the working
// set.
type Archiver interface {
	Archive(ctx context.Context, vaultID, itemID string) error
}

// NewVault is a customer boundary about to be created.
//
// A vault per customer is what makes "one customer Cosmic scoped to that
// customer's vault" true rather than aspirational. It is also what makes the
// claim a customer cares about — that they keep their keys when they leave —
// something you can demonstrate instead of promise.
type NewVault struct {
	Customer    string
	Description string
}

// Title is the vault name. Lowercase and hyphenated, because it becomes the
// first segment of every op:// reference for this customer and those are typed
// by hand, pasted into config, and read in error messages.
func (v NewVault) Title() string {
	t := strings.ToLower(strings.TrimSpace(v.Customer))
	t = strings.Join(strings.Fields(t), "-")
	return strings.Trim(nonSlug.ReplaceAllString(t, "-"), "-")
}

func (v NewVault) Validate() error {
	if v.Title() == "" {
		return fmt.Errorf("a customer name is required")
	}
	return nil
}

// Provisioner creates customer boundaries.
type Provisioner interface {
	CreateVault(context.Context, NewVault) (Vault, error)
}
