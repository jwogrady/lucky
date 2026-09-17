# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com/) and Semantic Versioning.

## [Unreleased]

### Added

- Initial Cobra-based Go CLI with desktop-app and service-account authentication.
- Status, vault listing, item listing, and safe `op://` resolution commands.
- Secret-safe output boundaries and unit tests.
- `lucky verify` proves each credential in a vault against the system it is for,
  with a non-zero exit when any credential fails.
- Provider templates carry an optional `verify` stanza, so checking a new
  provider is an edit to `providers.json` rather than vendor code in the
  custodian.
- `lucky run` resolves an env file's `op://` references and hands the values to
  one child process through its environment — never stdout, stderr, a log or
  disk. Resolution is all-or-nothing, every unresolvable reference is reported
  at once, and the child's exit status becomes Lucky's. The grammar follows
  `op inject`, so a reference embedded in a larger value resolves.
- `credential.Inspector` reads an item's field shape — labels, types and
  references — without its secret values.

### Changed

- The platform is two services, Lucky and Cosmic, rather than four. Vendor
  authority folds into Lucky: access is part of a key. Collect is a workload.
  ROADMAP v0.4 no longer says Connections decides what a credential can access.
- README no longer claims Lucky never shells out to `op`; it describes the SDK
  and CLI backends and what is guaranteed on both.
