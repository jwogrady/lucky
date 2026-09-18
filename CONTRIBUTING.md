# Contributing

Use `master` as the release branch. Work on a focused branch, keep commits small and problem-oriented, and open one pull request per GitHub issue. Use Conventional Commits and include tests for behavior changes.

Before opening a pull request, run `go test ./...` and `go vet ./...`. Never commit credentials, service-account tokens, resolved values, or test fixtures containing real secrets.
