# Lucky Roadmap

Lucky is the credential custodian for CosmOS.

**Lucky always has your back. Lucky hooks you up because he has all the connections. Lucky gets you in the door — he doesn't come in with you.**

Lucky is implemented in Go and uses 1Password as the official credential provider. Lucky stores and manages secrets in 1Password; downstream services receive references and short-lived resolved values only when needed.

## Why credentials-first makes an identity provider

A conventional identity provider asserts that somebody controls an email address. That is what a magic link proves, and it is a thin claim: an inbox.

Working credentials-first produces a thicker one. By the time Lucky has a person on file, that person has:

- been named and reached at an address or number they control,
- granted access to systems they demonstrably hold authority over,
- and had that access *proved against the vendor* by `lucky verify` — not once, but every time it runs.

"This person controls this inbox" and "this person controls this GoDaddy account, this Supabase project and this cPanel, and we confirmed it on Tuesday" are different assertions. The second is closer to what anyone relying on an identity actually wants to know, and it falls out of doing credential custody properly rather than being a separate product.

The consent record is the other half. An identity provider's real output is not "who is this" but "what did they authorize" — and that artifact already has to exist here for reasons that have nothing to do with identity.

### The tension, and the answer to it

It appears to pull against the differentiator. The promise is that a customer keeps their keys when they leave, and becoming the thing that vouches for who they are looks like the lock-in that promise rejects.

It is not, because leaving is settled: **they get a printout, by certified mail.**

That is a stronger answer than an export button. The test of "you keep your keys" is not whether we hand them back, it is whether the customer still has them when we do not exist — and paper needs no vendor, no account, no software and no us. Certified mail adds the delivery receipt, which gives the handover the same evidentiary standing as the consent that started it. The relationship opens with a recorded grant and closes with a recorded return.

It also concentrates risk. Being an identity provider means inheriting obligations that credential custody alone does not carry: revocation has to propagate, sessions have to end, recovery has to work for somebody who has lost the address the whole thing is anchored to, and a compromise stops being one customer's problem.

Neither is a reason not to do it. They are reasons the person boundary, the consent record and withdrawal have to be right first — an identity provider built on a custody model that cannot cleanly revoke is worse than no identity provider.

---

## Structure instead of a naming convention

The `wtp` vault records an attempt to find a naming convention, and what it actually shows is why one was needed:

```text
Housecall Pro API key             vendor + what it is
Google Maps browser key           vendor + which key
mandrill: cosmic-wtp              vendor : which project
Service Account Auth Token: wtp   what it is : what it is for
GitHub CLI - jwogrady             what it is - whose
cpanel_titan                      what it is _ which server
```

Five separators — space, colon, dash, underscore — in twelve items. That is not carelessness. Each title is a hierarchy flattened into one string because there was nowhere else to put it, and the separator changes because the relationship being expressed changes.

**A naming convention is what you need when the structure has nowhere to live.** 1Password provides four levels and Lucky's taxonomy uses them:

| level | 1Password | what it holds |
|---|---|---|
| account | vault | `status26` — who owns these |
| credential | item | `blare` — the system |
| group | section | `prod` — the variant, where there is one |
| key | field | `user`, `api key`, `endpoint` |

`mandrill: cosmic-wtp` is an item and a section. `cpanel_titan` is an item and a section. `GitHub CLI - jwogrady` is an item whose "whose" belongs to the person controlling the vault, not the title.

Put the structure in the structure and the name is just a name — one word, the system as a person would say it, no separator to choose.

### What follows

- No convention to enforce, document, or explain to the next operator.
- References read as what they are: `op://status26/blare/prod/api key`.
- `inventory` matches loosely today precisely because titles are unpredictable. As structure replaces convention, matching can tighten instead of getting cleverer.
- Existing items are not wrong and do not need rewriting to be usable. Lucky can point out where a title is carrying structure and suggest the split, but it cannot make that call — only a person knows whether `titan` is a server, a customer or a nickname.

---

## Owned by the account, controlled by people

Ownership and control are separate, and conflating them was the mistake in "the boundary is the person".

- **The account owns the vault.** A company's credentials belong to the company. People come and go; the GoDaddy account does not stop being the business's when the office manager leaves.
- **People control it.** Access is exercised by named individuals, and every grant, handover and withdrawal is attributable to one of them.
- **One of them is prime.** The person who can speak for the account — who can grant what others cannot, and who is asked when a grant needs authority behind it.
- **Rules decide who does what.** Control is not a flat list of people with equal reach.

This keeps what person-as-boundary was actually for. The reason that principle exists is that a business cannot consent — only a person can, and the record has to name them. That stays true. What changes is that consenting on behalf of an account is not the same as owning the account, and the vault follows ownership rather than consent.

It also answers the awkward cases the earlier framing could not. One person serving several businesses controls several vaults without their credentials being commingled. Several people granting for one business are all recorded against the one vault they share. And a person leaving does not orphan a vault — it changes who controls it, which is a rules question rather than a data-migration one.

### For now: one person per account

Every account is linked to one person. That is the working assumption, and it is worth stating because of how much it defers.

With one person per account there is no prime to designate, no rules deciding who may grant what, and no ambiguity about whose authority a consent record rests on — there is only one candidate. The person controlling `agds` is Hank Paulsen; the person is the account's controller by construction.

It also needs no code. A `person` item in the vault, holding name, email and mobile, is the link. The structure already supports it.

What will break the assumption, when it does:

- One person with two businesses — the same human controlling two accounts, which the model already handles since the vault is per account.
- One account with several people — an office manager and an owner, which is where prime becomes necessary and the rules below start to matter.
- A person leaving, when nobody else is attached to the account.

None of that is today's problem. The assumption is recorded so its expiry is recognisable rather than discovered.

### What prime means in practice

- Some grants require prime, not merely a person. The office manager reading out a login and the owner authorizing bank access are different acts.
- Prime is who Lucky asks when authority is in question, and who a consent record points at when a grant is challenged.
- Prime can change without the vault changing, which is the point of separating ownership from control.
- An account with no prime is an account nobody can speak for, and Lucky should say so rather than discover it during an incident.

---

## Identity, authentication, authorization

These are three things and the roadmap has been treating them as one. Separating them shows what is actually missing.

**Identity** — who this person is. The profile: name, email, mobile. Lucky has this.

**Authentication** — proof that it is them, *now*. The magic-link click is one moment of it, and one moment is all it is: there is no session, no expiry, no re-authentication when the work changes, and no second factor. A click six months ago proving somebody read an email is not the same as knowing who is on the phone today.

**Authorization** — what they may do. **This is missing entirely**, on both sides.

### Two RBAC surfaces, and only one is ours

**Our team, over customer vaults.** Today anyone who can reach a vault can do everything in it: read every secret, add credentials, archive them, and print the offboarding sheet. Those are not the same act and should not carry the same permission.

1Password already solves this and Lucky should drive it rather than invent it. The SDK exposes `Client.Groups()`, and on a vault: `GrantGroupPermissions`, `RevokeGroupPermissions`, `UpdateGroupPermissions`. Permissions arrive as a bitmask on `GroupAccess` — a `uint32` with no named constants in the Go types, so the bit meanings have to be established before anything is built on them.

This is the same argument as the storage model: the primitives exist, and Lucky's job is the experience of using them, not a parallel permission system that can disagree with the real one.

**The customer's own authority.** Not a 1Password concept, and this is the one that is genuinely ours.

A grant is only as good as the granter's right to give it. The office manager who reads out the GoDaddy login and the owner who reads out the bank details are not making the same kind of grant, and the consent record currently cannot tell them apart — it records that *a person agreed*, not that they were *entitled to agree*.

That gap matters most exactly where the stakes are highest. It is also the difference between a consent record that would survive being questioned and one that would not: "she clicked the link" is a weaker answer than "she clicked the link, and she is the person at that business who can grant this."

### What this changes

- The profile needs the person's role at the business, and it is not a free-text note — it is what bounds their grants.
- Consent records the granter's authority alongside the grant, because authority at the time is what the record has to preserve.
- Some credentials should require an owner, not merely a person.
- Authentication needs to be more than one historical click before any of this can be leaned on.

---

## Leaving: the printout

A customer leaves with their credentials on paper, sent by certified mail.

This is the load-bearing end of "they keep their keys". Every other form of return depends on something: an export file needs a device, a vault transfer needs an account, a link needs us to still be running. Paper in a hand needs nothing, which is the only version of the promise that survives the company disappearing.

Certified mail is doing real work too, not just formality. The grant was recorded — a named person, a timestamp, a delivery address. The return should be recorded to the same standard, and a delivery receipt is the paper equivalent of the click.

### What it demands

**This is the one command that deliberately emits secrets in bulk**, and it should be built like it. Everything else in Lucky exists to keep values from being seen, pasted, logged, or written down; this prints all of them at once.

- Nothing to disk. Straight to a print stream, or a document the operator is told to destroy after posting.
- Explicit, unmissable confirmation naming the person and the number of credentials.
- Its own record: who asked, which credentials, when, which address. The handover is an event in the customer's history, not a quiet read.
- The printed sheet needs the vendor and what each credential is for, not just values — a page of secrets nobody can attribute is not a handover.
- An obvious statement on the page that these are live credentials and should be rotated, because from the moment it is posted the customer is the only one who controls them.

**After it, say so.** What Lucky then holds is a copy, not the only copy, and if the relationship is ending the credentials should be removed from the working set and the consent withdrawn. Archived, not deleted — the record of what was held outlives the holding.

---

## What the CLI is

**Lucky's CLI is the input experience around 1Password. It assumes human hands: typing, copying, pasting.**

1Password already has the model. A vault holds items, an item holds fields, a field has a type and a value — which is exactly *person > credential > key > value*, and `Concealed` is a field type rather than a thing Lucky invents. The SDK already puts an item back with a key appended, already shares, already takes file attachments, already archives. Lucky should not re-derive any of it.

What the SDK has no opinion about is the part that involves a person: a value read down a phone line, a photograph of a sticky note, a password pasted out of an email. Capturing that without echoing it, without putting it in argv, without it touching disk, and without making someone choose a template before they can write anything down — that is the product.

### The split this creates

| | human present | no human |
|---|---|---|
| surface | `lucky` CLI | library, later the API |
| backend | `op` CLI, desktop unlock | SDK, service account |
| work | intake, lookup, handover | Collect runs, scheduled verification |

The CLI backend is not a fallback. It is the right backend for the case the CLI is for: a person is sitting there, their 1Password is unlocked, and they are typing. A service account would be the wrong credential for that — it would let the tool reach a customer's vault when nobody is at the keyboard.

It also means the existing `auto` rule is right for a reason worth stating: `OP_SERVICE_ACCOUNT_TOKEN` being set *is* the signal that no human is present, so preferring the SDK when it exists and the CLI when it does not is the human/unattended split expressing itself.

`lucky run` is the seam between the two. It is the moment a value captured by hand is handed to a process, and after it no human is involved.

---

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

- ~~Does the vault stay per business, or does the person become the vault?~~ **Settled: the vault is the company.** See "Owned by the account, controlled by people" below.
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
