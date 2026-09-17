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
- **The boundary is the person, not the account.** Nobody hands Lucky an account. A *person* grants our team access to something they hold authority over, and that person is what the grant is attributable to, revocable by, and answerable to. An account is what the access happens to point at.
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

## Command surface: `lucky new <noun>`

The operator commands grew one verb at a time — `new-customer`, `profile`, `put` — and the names do not say that they are the same act at different scopes. The surface becomes:

```text
lucky new profile      a person: first name, last name, email, mobile
lucky new api          a vendor API credential
lucky new cred         a credential that is not an API key
```

`profile` currently means a *business* record — name, dba, domain, address, service area. Under the new surface a profile is a **person**, and the business record needs its own noun. Both are wanted; they are not the same thing and should not share a name.

---

## First use case: the transposing case

The phone rings. The customer says "log into my GoDaddy account and change a phone number on my website." The answer is: sure — who are you? I need to complete your Lucky profile so you can read me the information over the phone.

**The credential is transposed through the operator, verbally.** It is not emailed, not pasted, not linked. The customer says it and the operator types it.

This is the first case to satisfy because it needs nothing that does not exist: no magic link, no API, no service to send anything. It is also the case that actually happens.

### "Who is Lucky?"

It comes up on the call, and the answer is short:

> Lucky is our key manager. Anything you need from me, I ask Lucky for the keys.

That is a claim about how we work, and the system has to be built so it stays true. It says the operator does not keep the customer's keys to hand. They are in the customer's vault, and access is asked for at the moment it is needed — which is why credentials resolve to a reference everywhere except the one process that needs the value, and why `lucky run` puts a resolved secret into a child process and nowhere else.

It is also the sentence that makes the boundary legible to the person on the other end of the phone. They are not handing their password to a company. They are handing it to a custodian that answers for it, and they can take it back.

### What it demands

**The profile comes first, and it is the natural moment for it.** "Who are you" is not a formality on a phone call, it is the question you would ask anyway. The person on the line is the one granting access, so the profile is captured while they are there to answer — and the grant is attributable to them from the start rather than reconstructed later.

**The credential is usually a login, not an API key.** Nobody dictates an API key over the phone for GoDaddy; they read out the username and password they use. The catalog has `godaddy api` — a key and secret from the developer portal — and no login shape at all. This is what `lucky new cred` is for, and the transposing case is why it exists alongside `new api`.

**Transcription is the failure mode.** Every other intake path either pastes the exact bytes or has the owner type them. This one goes through a human ear and a human keyboard: `b`/`p`/`d`/`e` are the same sound, and `0`/`O` and `1`/`l`/`I` are the same character read aloud. A mistyped credential is indistinguishable from a revoked one at the moment of entry.

**So verification has to happen before the call ends.** This is the strongest argument for `lucky verify` existing at all. If the credential fails, the person who can read it again is still on the line — five minutes later they are not, and the failure becomes a callback, a second appointment, and a customer who has been asked twice. Verification is not a report in this case. It is the transcription check.

### Consequences

- A login shape in the catalog: username, password, and where to use them. Credentials people speak aloud, rather than credentials consoles issue.
- Verification for a login is not an API call. Proving a username and password means a sign-in, which is a different mechanism from the `verify` stanza and may not be available at all. Where it is not, the honest report is `unchecked` — and the operator needs to be told that while the customer is still on the phone, not afterwards.
- Read-back before writing. The confirmation screen already describes a value without revealing it; a dictated value wants the opposite — the operator reading it back to the customer to confirm, which is safe precisely because the customer already knows it.
- The profile fields need to be answerable aloud, in order, without the operator navigating a form while listening.

---

## The boundary is the person

An earlier version of this document said the customer's vault is the boundary, and scoped a Cosmic to it. That is still how storage is arranged, but it named the wrong thing as the boundary.

We are not given accounts. We are given access, by a person, to something they hold authority over. That person is the unit that matters:

- **Attribution.** "Who authorized this" has a person as its answer, never a business. A business cannot consent.
- **Revocation.** When a person leaves a business, everything they granted is in question — and nothing else is. That set has to be identifiable, which means the grant records who made it.
- **Consent.** A magic link goes to a person. What comes back is that person's grant, which is why the link is the mechanism and not a convenience.
- **Least privilege.** What a credential reaches was decided by the person who issued it, with whatever authority they personally had.

### Consequences to settle

- Does the vault stay per business with the grant attributing to a person, or does the person become the vault? The first keeps a customer's keys in one place; the second follows the boundary literally. These are not equivalent when one person serves several businesses, or several people grant for one.
- What happens to a credential when the person who granted it leaves. It has not been revoked by the vendor, and it probably still works — which is precisely the problem.
- One person granting across several customers: whose boundary holds that credential.
- The audit record of a grant is not secret, and needs to outlive the credential it authorized.

---

## Intake first, then permission

**The CLI does the intake. The link asks for permission afterwards.**

The order is the point, and it follows the call rather than fighting it:

1. `lucky new profile` — who is this. Asked on the call, because it is the question you would ask anyway.
2. **Secure intake, in the CLI.** The customer reads the credential; the operator enters it; it goes straight to the customer's vault and nowhere else.
3. `lucky verify` — while they are still on the line, because transcription is the failure mode.
4. **Then the link**, to the person whose profile was just captured:

   > Click on the link to authorize whatever it is we just talked about.

The credential is already safe at step 2. What step 4 adds is the record that the person agreed to it — and it is asked for *after*, because the customer called asking for work to be done and holding that work hostage to an email they have not opened yet serves nobody.

### Why permission needs its own artifact

Secure intake solves custody: the secret is in the right vault, never on disk, never in an inbox. It does not solve authority. The call ends with an operator holding a customer's password and nothing showing the customer agreed to that. Both people remember it and neither can produce it. A dispute months later — "I never authorized anyone to log into my GoDaddy" — has no answer, and the honest position is that the customer is right to ask.

A clicked link tied to a profile is the answer: a named person, contactable at an address they control, acting at a recorded time, against a scope written while the conversation was still happening.

It also protects the customer, which is the point. Access granted on a phone call with no record is access nobody can audit — including them.

### The first scope: general access, until revoked

Start with one scope, and the broadest one:

> You are authorized to log in and make changes until I tell Lucky otherwise.

Standing, not time-boxed. Covers logging in and changing things, not read-only. Granted once by a person, and good until that person withdraws it.

**Lucky, not us.** The customer withdraws by telling the custodian, not by telling the team whose access they are withdrawing. That is the whole difference between a promise and a control: nobody should have to ask the person they are revoking to process the revocation, or wonder whether the message was passed on. It also means the operator cannot quietly be the reason a withdrawal did not take effect.

This is the honest starting point because it is what actually happens on the call. A customer who rings up and asks for a phone number to be changed on their website is not granting a scoped, single-use permission — they are saying "you look after this for me." A consent record claiming anything narrower would be a record of something that did not occur.

**"Until I tell Lucky otherwise" is the obligation.** A grant that can be withdrawn only in principle has not really been given on those terms. Withdrawal needs to be as easy as the click that granted it, reachable by the customer without going through us, and it needs its own record — who withdrew, when — because the interesting question afterwards is not whether access exists now but what was authorized during the period it did.

This is the same gap as the link: the customer needs somewhere to reach Lucky that is not an operator's terminal. Consent and withdrawal are two ends of one customer-facing surface, and it is the piece the CLI cannot be.

Two things follow that are easy to get wrong:

- Withdrawal is not deletion. The grant happened, and the record of it outlives the access, for the same reason `archive` exists and `delete` does not.
- Withdrawn consent does not revoke the credential. The key still works; only the vendor can change that. Lucky can record that it is no longer authorized and stop resolving it — and the honest follow-up is telling the customer their key is still live and should be rotated.

Narrower scopes come later. They are a refinement of a working record, not a prerequisite for having one.

### What the record has to carry

- **Who**: the profile, which is why the profile is captured before anything else.
- **What**: the scope, in the words used on the call. "Whatever it is we just talked about" is what the customer heard; the record needs the operator's version of it, written during the call rather than reconstructed.
- **When**, and through which address or number the link was delivered and opened.
- Non-secret throughout, and it outlives the credential it authorized.

### Open questions

- What happens to a credential whose link is never clicked. It is already in the vault and already works, so this is not a gate — it is a state. Does it expire, archive itself, or simply show as unauthorized in `inventory` until someone chases it?
- Whether the operator may act on a credential before the click lands. In the transposing case they were asked to do the work by the person on the phone, so the answer is probably yes, with the record catching up — but that should be a decision rather than a default.
- Scope granularity beyond the general grant: per credential, or per engagement.
- Re-consent: when the work changes, when the credential rotates, or on a schedule.
- Email or SMS. The profile captures both, and a link to a mobile is a different assurance from a link to an inbox.

---

## Customer-entered credentials

**The customer enters their own credential. The operator never handles it.**

A profile — first name, last name, email, mobile — is what Lucky needs to send a magic link. The link opens a secure field where the customer enters and authorizes the credential themselves. It lands in their vault, and Lucky hands back a reference as it does today.

The profile is not contact details. It identifies the person whose grant this is, which is the record the boundary rests on.

This removes the operator from the secret's path entirely. Today a credential arrives by email or text, sits in an inbox, and gets pasted into a terminal by someone who is not its owner; `put` makes that moment as safe as it can be, but the copy in the inbox still exists and the operator still saw the value. A link the customer fills in has no such copy.

It also changes what "authorize" means. The customer is the party who actually holds authority with the vendor, so the credential arrives already granted by the person entitled to grant it, rather than relayed by someone acting on their behalf.

### Open questions

- What serves the link. Intake is the CLI, but a link has to point at something a customer can open, and an operator's terminal is not a URL. This is the one piece the CLI cannot be.
- Link lifetime, single use, and what happens to a link that is never opened.
- Whether the mobile number is a second factor on the link or only a contact route.
- What the customer sees: a bare field, or the template's `where` text guiding them to the right console page.
- Where the audit record of "who authorized what, when" lives, given it is not secret and the vault is the customer boundary.

### Relationship to v0.3

v0.3 is "Lucky can share" — handing a credential *out* securely. This is the same problem in the other direction: taking one *in* securely. They likely share a mechanism and should be designed together.

Consent belongs on the way out too: handing a credential to someone is an act a person authorized, and it wants the same record.

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
