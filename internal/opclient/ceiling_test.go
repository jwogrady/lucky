package opclient_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	onepassword "github.com/1password/onepassword-sdk-go"
)

// The plan ceiling, measured one SDK call at a time.
//
// v1 runs on an individual account and upgrades only when the API is genuinely
// exhausted, so where the API stops is a fact the plan depends on.
//
// Two different things stop a call, and only one of them is documented.
// 1password.dev/sdks/functionality lists what the SDK implements — sharing,
// password generation and group vault permissions are in; listing or creating
// groups, user vault permissions and everything about users are out. What it
// does not say is what any given account is *permitted* to do, and the pricing
// pages are no help because they describe an organization sharing credentials
// internally, which is not the shape being built here.
//
// So this measures the second axis only: given that the SDK implements a call,
// does this token get to make it. Anything the SDK does not implement has no
// test here, because a compile error is a clearer answer than a probe.
//
// Two kinds of test live here, and the difference is the point:
//
//   - Required. v1 cannot work without it, so a refusal fails. Intake, resolve,
//     archive, list.
//   - Exploratory. The answer is unknown and either answer is useful, so it is
//     reported and never fails. Groups, sharing, vault creation.
//
// Nothing runs by accident. Both variables must be set:
//
//	OP_SERVICE_ACCOUNT_TOKEN=ops_... LUCKY_CEILING_VAULT=ceiling \
//	  go test ./internal/opclient/ -run TestCeiling -v
//
// LUCKY_CEILING_VAULT must be a throwaway vault and is the only one touched. It
// has to be a vault somebody created: 1Password documents that a service
// account cannot reach Personal, Private or Employee vaults at all, which on an
// individual account rules out the default one.
//
// No test calls Delete. Lucky archives and does not delete — `lucky` has no
// delete command and a test asserts it never gains one — and a probe is a bad
// reason to make this package the first caller.

const (
	tokenEnv = "OP_SERVICE_ACCOUNT_TOKEN"
	vaultEnv = "LUCKY_CEILING_VAULT"
)

// ceiling authenticates and finds the throwaway vault, skipping the test
// entirely when this machine has no token to measure.
func ceiling(t *testing.T) (*onepassword.Client, string, string) {
	t.Helper()
	token, title := os.Getenv(tokenEnv), os.Getenv(vaultEnv)
	if token == "" || title == "" {
		t.Skipf("set %s and %s to measure the ceiling", tokenEnv, vaultEnv)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("Lucky ceiling probe", "dev"),
	)
	if err != nil {
		t.Fatalf("authentication failed: %s", scrub(err, token))
	}

	vaults, err := client.Vaults().List(ctx)
	if err != nil {
		t.Fatalf("could not list vaults: %s", scrub(err, token))
	}
	for _, v := range vaults {
		if strings.EqualFold(v.Title, title) {
			return client, v.ID, v.Title
		}
	}
	t.Fatalf("no vault titled %q is reachable by this token (%d reachable). "+
		"Create it, then grant the service account access to it.", title, len(vaults))
	return nil, "", ""
}

// probeItem files a throwaway credential and archives it when the test ends, so
// each test stands alone rather than depending on what ran before it.
func probeItem(t *testing.T, client *onepassword.Client, vaultID string) onepassword.Item {
	t.Helper()
	ctx := t.Context()
	item, err := client.Items().Create(ctx, onepassword.ItemCreateParams{
		VaultID:  vaultID,
		Title:    fmt.Sprintf("lucky ceiling probe %d", time.Now().UnixNano()),
		Category: onepassword.ItemCategoryAPICredentials,
		Fields: []onepassword.ItemField{{
			ID:        "credential",
			Title:     "credential",
			FieldType: onepassword.ItemFieldTypeConcealed,
			Value:     "ceiling-probe-not-a-real-secret",
		}},
	})
	if err != nil {
		t.Fatalf("items.Create: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	t.Cleanup(func() {
		if err := client.Items().Archive(context.WithoutCancel(ctx), vaultID, item.ID); err != nil {
			t.Logf("could not archive probe item %s: %s", item.ID, scrub(err, os.Getenv(tokenEnv)))
		}
	})
	return item
}

// scrub keeps the token out of any message, the rule the rest of this package
// follows: a report that prints a credential is a report nobody can paste.
func scrub(err error, token string) string {
	if err == nil {
		return "<nil>"
	}
	if token == "" {
		return err.Error()
	}
	return strings.ReplaceAll(err.Error(), token, "[REDACTED]")
}

// reportf records an exploratory answer. It never fails: not being allowed to
// do something is the measurement, not a bug.
func reportf(t *testing.T, call string, err error) bool {
	t.Helper()
	if err != nil {
		t.Logf("CEILING  %-26s refused: %s", call, scrub(err, os.Getenv(tokenEnv)))
		return false
	}
	t.Logf("ALLOWED  %-26s", call)
	return true
}

// ---------------------------------------------------------------------------
// Required: v1 does not work without these.
// ---------------------------------------------------------------------------

func TestCeilingVaultsList(t *testing.T) {
	client, _, _ := ceiling(t)
	vaults, err := client.Vaults().List(t.Context())
	if err != nil {
		t.Fatalf("vaults.List: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	t.Logf("ALLOWED  vaults.List                %d reachable", len(vaults))
}

func TestCeilingItemsList(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	items, err := client.Items().List(t.Context(), vaultID)
	if err != nil {
		t.Fatalf("items.List: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	t.Logf("ALLOWED  items.List                 %d item(s)", len(items))
}

// Intake. Everything else in Lucky is downstream of being able to file a
// credential nobody templated.
func TestCeilingItemsCreate(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	item := probeItem(t, client, vaultID)
	t.Logf("ALLOWED  items.Create               %s", item.ID)
}

func TestCeilingItemsGet(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	created := probeItem(t, client, vaultID)
	got, err := client.Items().Get(t.Context(), vaultID, created.ID)
	if err != nil {
		t.Fatalf("items.Get: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	t.Logf("ALLOWED  items.Get                  %d field(s)", len(got.Fields))
}

// Resolve is the single way a secret leaves the vault, so if it is refused
// there is no Lucky at all.
func TestCeilingSecretsResolve(t *testing.T) {
	client, vaultID, vaultTitle := ceiling(t)
	item := probeItem(t, client, vaultID)
	ref := fmt.Sprintf("op://%s/%s/credential", vaultTitle, item.Title)
	got, err := client.Secrets().Resolve(t.Context(), ref)
	if err != nil {
		t.Fatalf("secrets.Resolve: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	if got != "ceiling-probe-not-a-real-secret" {
		t.Fatalf("resolved value did not round-trip")
	}
	t.Logf("ALLOWED  secrets.Resolve")
}

// Adding a key to a credential that already exists — `lucky for` does this
// whenever one credential turns out to carry more than one pair.
func TestCeilingItemsPut(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	item := probeItem(t, client, vaultID)
	item.Fields = append(item.Fields, onepassword.ItemField{
		ID: "endpoint", Title: "endpoint", FieldType: onepassword.ItemFieldTypeText,
		Value: "https://api.example.test",
	})
	updated, err := client.Items().Put(t.Context(), item)
	if err != nil {
		t.Fatalf("items.Put: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	t.Logf("ALLOWED  items.Put                  %d field(s) after add", len(updated.Fields))
}

// Archive, not delete. `lucky archive` retires a credential and keeps the
// record, which is the only retirement Lucky offers.
func TestCeilingItemsArchive(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	item, err := client.Items().Create(t.Context(), onepassword.ItemCreateParams{
		VaultID:  vaultID,
		Title:    fmt.Sprintf("lucky ceiling archive %d", time.Now().UnixNano()),
		Category: onepassword.ItemCategoryAPICredentials,
		Fields: []onepassword.ItemField{{
			ID: "credential", Title: "credential",
			FieldType: onepassword.ItemFieldTypeConcealed, Value: "archive-me",
		}},
	})
	if err != nil {
		t.Fatalf("items.Create: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	if err := client.Items().Archive(t.Context(), vaultID, item.ID); err != nil {
		t.Fatalf("items.Archive: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	t.Logf("ALLOWED  items.Archive")
}

// ---------------------------------------------------------------------------
// Exploratory: the answer decides how much of the roadmap survives v1.
// ---------------------------------------------------------------------------

// Groups are what ROADMAP "Two RBAC surfaces, and only one is ours" wants for
// scoping a member of staff to one job. Groups are an organization concept, so
// this is the likeliest wall on an individual account — and the error text is
// the finding, because a plan-level refusal reads differently from "no such
// group".
//
// Retrieve is all the SDK offers: listing groups, creating them and changing
// membership are documented as unimplemented. So even where this is allowed,
// the groups themselves get made by hand in the web UI and Lucky only grants
// them vault permissions afterwards.
func TestCeilingGroupsGet(t *testing.T) {
	client, _, _ := ceiling(t)
	_, err := client.Groups().Get(t.Context(), "00000000000000000000000000", onepassword.GroupGetParams{})
	reportf(t, "groups.Get", err)
}

// Vault permissions for a group: the mechanism Lucky would drive rather than
// invent. Meaningless without groups, so it is measured second.
func TestCeilingVaultsGrantGroupPermissions(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	err := client.Vaults().GrantGroupPermissions(t.Context(), vaultID, []onepassword.GroupAccess{{
		GroupID: "00000000000000000000000000", Permissions: onepassword.ReadItems,
	}})
	reportf(t, "vaults.GrantGroupPermissions", err)
}

// v0.3 is "Lucky can share". The SDK has the API; whether this account may use
// it decides whether that version is wiring or a plan upgrade.
func TestCeilingSharesGetAccountPolicy(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	item := probeItem(t, client, vaultID)
	policy, err := client.Items().Shares().GetAccountPolicy(t.Context(), vaultID, item.ID)
	if reportf(t, "shares.GetAccountPolicy", err) {
		t.Logf("         account share policy: %+v", policy)
	}
}

// Handing a credential to somebody outside the account, which is the half of
// the customer-facing surface that points outward.
func TestCeilingSharesCreate(t *testing.T) {
	client, vaultID, _ := ceiling(t)
	item := probeItem(t, client, vaultID)
	policy, err := client.Items().Shares().GetAccountPolicy(t.Context(), vaultID, item.ID)
	if err != nil {
		t.Skipf("no share policy to work from: %s", scrub(err, os.Getenv(tokenEnv)))
	}
	link, err := client.Items().Shares().Create(t.Context(), item, policy, onepassword.ItemShareParams{
		OneTimeOnly: true,
	})
	if reportf(t, "shares.Create", err) {
		t.Logf("         a share link was issued (%d chars); it expires on its own", len(link))
	}
}

// new-customer creates a vault today. Under customer-owned accounts that
// inverts — the customer provisions and hands over a token — so whether a
// service account may create a vault at all decides how much of the current
// flow survives the change.
func TestCeilingVaultsCreate(t *testing.T) {
	client, _, _ := ceiling(t)
	desc := "created by Lucky's ceiling probe; safe to remove"
	v, err := client.Vaults().Create(t.Context(), onepassword.VaultCreateParams{
		Title:       fmt.Sprintf("lucky-ceiling-%d", time.Now().Unix()),
		Description: &desc,
	})
	if reportf(t, "vaults.Create", err) {
		t.Logf("         created %q (%s) — remove it by hand; Lucky does not delete", v.Title, v.ID)
	}
}
