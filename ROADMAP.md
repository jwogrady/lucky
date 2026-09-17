# Lucky Roadmap

Lucky is the credential custodian for CosmOS.

**Lucky always has your back. Lucky hooks you up because he has all the connections. Lucky gets you in the door — he doesn't come in with you.**

Lucky is implemented in Go and uses 1Password as the official credential provider. Lucky stores and manages secrets in 1Password; downstream services receive references and short-lived resolved values only when needed.

## The platform is two services

An earlier version of this document described four: Lucky, Connections, Collect, and Cosmic. That has been reduced, and the reduction is not cosmetic — it moves work across a service boundary.

- **Lucky — keys and access.** Access is part of keys. A credential *is* an access grant: what it can reach was decided when it was issued, by whoever scoped it. A separate service asking a vendor "what may this key do" would be re-deriving something the key already carries, and would need the key to do it — which means holding it, which is Lucky's job. So vendor authority folds into Lucky.
- **Cosmic — runtime and storage.** The runtime handles input and output; storage handles persistence and logs.

**Collect is not a platform service. It is the first workload** that runs on those two.

The practical consequence for this repository: if Lucky owns access, Lucky owns *proving* access. That is `lucky verify`, and it is what makes "existing is not the same as working" a checkable claim about a customer's whole vault rather than a thing an operator finds out during an incident.

## Principles

- **1Password is the system of record for secrets.**
- **No secrets in Git, DuckDB, logs, config files, or application databases.**
- **Cosmic stores credential references and authority metadata, never secret values.**
- **One customer Cosmic should be scoped to that customer's vault/access boundary.**
- **CLI first. API later.** The Go core must not depend on either interface.
- **Least privilege by default.** A credential should expose only what the work requires.
- **Lucky manages credentials, not business data.** Vendor data belongs to the workload and to Cosmic storage.
- **Existing is not the same as working.** A credential Lucky holds but cannot prove is a credential Lucky reports as unproven.

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
lucky run --env-file .env.op -- <command>
```

`lucky run` is the "juggles keys between environments" half of Lucky's stated purpose: it resolves an env file's references and hands the values to one child process, and to nothing else.

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

## v0.4 — Lucky owns access

Goal: make Lucky consumable as a library, and make the access a credential carries something Lucky can demonstrate.

This section previously read "Lucky meets Connections" and said that Lucky does **not** decide what a credential can access — Connections does. Connections is no longer a separate service, and that sentence is withdrawn. What replaces it is narrower and more honest: Lucky does not *grant* access, because the vendor already did that when the key was issued. Lucky holds the key, and can therefore show what it reaches.

### Deliverables

- Public Go interface for credential resolution. *(done — the `credential` package and `lucky.New`)*
- Typed credential reference model. *(done — `credential.ValidateReference`, sections included)*
- A safe error model:
  - credential missing
  - vault inaccessible
  - reference malformed
  - secret unavailable
  - authentication expired
- No 1Password-specific behavior leaks into consumers beyond the reference abstraction.
- **Verification as template data, not vendor code.** A provider template carries the one call that proves a credential of that shape works. Adding a provider is an edit to `providers.json`; a customer with a non-standard endpoint is an overlay file. No vendor's API becomes a build-time dependency of the custodian.
- A check that cannot be expressed as one HTTP call gets no stanza and is reported as unchecked. Lucky does not guess.

### CLI

```text
lucky verify --vault wtp
lucky verify --vault wtp --provider google
```

### Acceptance

An operator can ask one question — "is this customer actually connected?" — and get a per-credential answer, with a non-zero exit when any credential fails, and no secret value anywhere in the output or in any error along the way.

---

## v0.5 — Lucky carries the first workload

Goal: let Collect, the first workload, run on proved credentials without exposing secrets beyond the operation that needs them.

### Deliverables

- Scoped credential resolution for Gather/Collect jobs.
- `lucky run` puts resolved values into a child process's environment and nowhere else — no file, no argv, no shell history.
- Clear lifecycle around resolved values.
- Collector integration for the first WTP sources:
  - Google Search Console
  - Google Analytics
  - Google Ads
  - Google Business Profile
  - Housecall Pro
- Secret values excluded from collected payloads, checkpoints, and DuckDB.

### Acceptance

A credential Lucky has proved can authenticate a Collect run, the workload retrieves vendor data, and the resulting customer data lands in Cosmic storage with no credential material persisted there.

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

Cosmic and other authorized CosmOS services can use Lucky remotely without embedding 1Password integration code themselves.

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
- Integration tests proving Lucky → Collect for a reference Cosmic.

## First reference implementation

`cosmic-wtp` is Lucky's first real consumer.

The initial path is:

```text
1Password vault: wtp
        ↓
      Lucky  — holds the keys, and proves what they reach
   ├── Google
   └── Housecall Pro
        ↓
     Collect  — the first workload, not a service
        ↓
 cosmic-wtp storage  — Cosmic runtime and storage
```

The first useful outcome is not a dashboard. It is proof that a customer-authorized credential can be safely managed by Lucky, proved by Lucky against the vendor, and used by Collect to bring the customer's vendor data into their Cosmic.
