# Lucky Roadmap

Lucky is the credential custodian for CosmOS.

**Lucky knows the keys. Connections knows the doors. Collect brings the data home.**

Lucky is implemented in Go and uses 1Password as the official credential provider. Lucky stores and manages secrets in 1Password; downstream services receive references and short-lived resolved values only when needed.

## Principles

- **1Password is the system of record for secrets.**
- **No secrets in Git, DuckDB, logs, config files, or application databases.**
- **Cosmic stores credential references and authority metadata, never secret values.**
- **One customer Cosmic should be scoped to that customer's vault/access boundary.**
- **CLI first. API later.** The Go core must not depend on either interface.
- **Least privilege by default.** A credential should expose only what the connection requires.
- **Lucky manages credentials, not business data.** Vendor data belongs to Collect and Cosmic storage.

## v0.1 — Lucky is born

Goal: prove Lucky can safely reach 1Password and resolve credentials.

### Deliverables

- Go module and `lucky` CLI.
- Pin the official `github.com/1password/onepassword-sdk-go` dependency.
- Support local authentication through the 1Password desktop app.
- Support unattended authentication through `OP_SERVICE_ACCOUNT_TOKEN`.
- Resolve an `op://vault/item/field` secret reference.
- List accessible vaults.
- List items in a selected vault.
- Redact secret values from normal command output and logs.

### CLI

```text
lucky status
lucky vaults
lucky items --vault wtp
lucky get op://wtp/<item>/<field>
```

### Acceptance

Lucky can authenticate, inspect the `wtp` vault, and resolve a known reference without writing the secret to disk or logs.

---

## v0.2 — Lucky takes the keys

Goal: make Lucky the standard path for entering and maintaining customer credentials.

### Deliverables

- Create credential items.
- Update credential items.
- Archive credentials without destroying history unnecessarily.
- Support common credential shapes:
  - API key
  - username/password
  - client ID/client secret
  - OAuth refresh token
  - service-account JSON
  - SSH key
  - certificate/file attachment
- Generate strong passwords when appropriate.
- Return stable 1Password references after writes.
- Define the customer → vendor → service naming convention.

### CLI

```text
lucky put --vault wtp --vendor google --service search-console
lucky put --vault wtp --vendor housecallpro --service api
lucky update <reference>
lucky archive <reference>
```

### Acceptance

An operator can enter a new vendor credential once, receive an `op://` reference, and never copy that secret into Cosmic configuration.

---

## v0.3 — Lucky can share

Goal: support secure handoff when another person must provide or receive credentials.

### Deliverables

- Share 1Password items using the SDK.
- Generate secure share links where supported.
- Record non-secret audit metadata for share operations.
- Expiration/default-sharing policy.
- Friendly operator output suitable for onboarding workflows.

### CLI

```text
lucky share <reference>
lucky share <reference> --expires 7d
```

### Acceptance

An operator can securely request or hand off a credential without email, chat, tickets, or plaintext documents carrying the secret.

---

## v0.4 — Lucky meets Connections

Goal: establish the contract between credential custody and vendor authority.

Lucky does **not** decide what a credential can access. Connections does.

### Deliverables

- Public Go interface for credential resolution.
- Typed credential reference model.
- Connection-safe error model:
  - credential missing
  - vault inaccessible
  - reference malformed
  - secret unavailable
  - authentication expired
- No 1Password-specific behavior leaks into vendor connectors beyond the reference abstraction.
- Integrate first with WTP Connections:
  - Google
  - Housecall Pro

### Contract

```text
Lucky
  ↓ credential reference / resolved secret
Connections
  ↓ verified vendor authority and discovered resources
Collect
```

### Acceptance

Connections can ask Lucky for a credential, validate it against a vendor, and persist only the credential reference plus discovered authority metadata.

---

## v0.5 — Lucky enables Collect

Goal: let verified Connections drive real collection without exposing secrets beyond the operation that needs them.

### Deliverables

- Scoped credential resolution for Gather/Collect jobs.
- Clear lifecycle around resolved values.
- Collector integration for the first WTP sources:
  - Google Search Console
  - Google Analytics
  - Google Ads
  - Google Business Profile
  - Housecall Pro
- Secret values excluded from collected payloads, checkpoints, and DuckDB.

### Acceptance

A verified WTP Connection can use Lucky to authenticate, Collect can retrieve vendor data, and the resulting customer data lands in Cosmic storage with no credential material persisted there.

---

## v0.6 — Lucky becomes provisionable

Goal: make Lucky repeatable for every Cosmic, not specific to WTP.

### Deliverables

- Customer-specific vault/service-account model.
- Least-privilege service-account provisioning guidance.
- Bootstrap configuration containing only what is necessary to reach 1Password.
- Vault naming and ownership policy.
- Health/status checks suitable for Cosmic provisioning.
- Rotation path for Lucky's own service-account credential.

### Acceptance

A newly provisioned Cosmic can be given its customer-specific 1Password access and immediately use Lucky without inheriting access to unrelated customer vaults.

---

## v0.7 — Lucky gets an API

Goal: expose Lucky to other CosmOS services only after the CLI and domain model are proven.

### Deliverables

- Thin API over the existing Go core.
- Authentication and authorization for callers.
- Resolve/store/share operations without returning more secret material than necessary.
- Audit metadata for machine access.
- CLI remains supported as the operator interface.

### Acceptance

Connections and other authorized CosmOS services can use Lucky remotely without embedding 1Password integration code themselves.

---

## v1.0 — Lucky is trusted with the keys

Goal: production-ready credential custody for CosmOS.

### Required characteristics

- Stable Go interfaces.
- Pinned and reviewed 1Password SDK dependency.
- Least-privilege access model.
- Secret-safe logging and error handling.
- Tested desktop and service-account authentication modes.
- Tested create/read/update/archive/share lifecycle.
- Auditable non-secret operational events.
- Documented recovery and credential-rotation procedures.
- Integration tests proving Lucky → Connections → Collect for a reference Cosmic.

## First reference implementation

`cosmic-wtp` is Lucky's first real consumer.

The initial path is:

```text
1Password vault: wtp
        ↓
      Lucky
        ↓
   Connections
   ├── Google
   └── Housecall Pro
        ↓
     Collect
        ↓
 cosmic-wtp storage
```

The first useful outcome is not a dashboard. It is proof that a customer-authorized credential can be safely managed by Lucky, verified through Connections, and used by Collect to bring the customer's vendor data into their Cosmic.
