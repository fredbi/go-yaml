# Contributing

Thanks for considering a contribution.

Before you start, please read [the fork's README](../README.md): this repository is a soft fork of
[`goccy/go-yaml`](https://github.com/goccy/go-yaml), and for a bug that also exists upstream, **the pull request
is often better sent upstream** — everyone benefits, and we track upstream closely.

Work that belongs here rather than upstream is anything tied to what the fork is for: the token/AST surface,
source positions, streaming, and the scanner/parser performance work.

## Problem statement

Describe the problem the pull request solves, or reference an existing issue. "Fix bug" is not a description —
explain *what* was wrong and *why* the change is correct.

## Tests are mandatory

Every bug fix and every feature must come with tests that demonstrate the problem and verify the fix. The only
exceptions are documentation changes and typo fixes. Aim for at least 80% coverage of your patch.

```sh
go test ./...
```

Test code that needs third-party libraries goes under `internal/testdata/`, which carries a `go_test.mod` rather
than a `go.mod` so those dependencies never reach the published `go.mod`. `-modfile` cannot be combined with a
workspace, so run it with the workspace off:

```sh
cd internal/testdata && GOWORK=off go test -modfile=go_test.mod ./...
```

Measurement modules (`internal/analysis`, `internal/benchmarks`) are in `go.work`, so `go test work ./...` covers
them along with the library.

Conformance matters here more than in most libraries. If your change affects what the parser accepts or rejects,
say so explicitly in the pull request — a change in acceptance is a behaviour change even when it is a fix.

## Linting

```sh
golangci-lint run --new-from-rev master
```

Install the latest version with:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

Every `//nolint` directive must carry an inline comment explaining why. Prefer disabling a linter in
`.golangci.yml` over scattering `//nolint` across the codebase.

Format with `golangci-lint fmt` (not `gofmt` or `gofumpt` directly).

## Commits

- Every commit must be DCO signed off (`git commit -s`) with a real email address. PGP signatures are appreciated
  but not required.
- Agents may be listed as co-authors (`Co-Authored-By:`), but the author of a commit must be a human.
- Squash into logical units of work before requesting review (`git rebase -i`).

## Supported Go versions

The two most recent stable Go minor versions.
