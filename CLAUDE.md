# kms-import

CLI tool and Go library that imports a GitHub App private key (PEM) into AWS KMS
as non-extractable key material, so JWT signing can be delegated to the KMS
`Sign` API. See `docs/prd-kms-import.md` and `docs/plan-kms-import.md`.

## Authoritative documentation (mandatory)

This project uses **Go 1.26** and SDK versions released after the AI knowledge
cutoff. Do not answer API questions from training data. Before using an
unfamiliar API, fetch current docs from these two sources — they are mandatory,
not optional:

- **Context7** — for language, SDK, and framework APIs (Go stdlib, AWS SDK,
  urfave/cli). Use the registered IDs below; resolve new libraries as needed.
- **AWS Knowledge Base MCP** (`aws___search_documentation` + `aws___read_documentation`)
  — for AWS *service* behaviour and procedures: KMS import/wrapping mechanics,
  exact wire formats, IAM policy shapes, CloudTrail. The crypto here is
  unforgiving; verify wire-format details against this source, not memory.

Registered Context7 IDs:

| Library | Context7 ID |
|---|---|
| Go 1.26 standard library (`crypto/*`, `encoding/pem`, …) | `/websites/pkg_go_dev_std` |
| `github.com/aws/aws-sdk-go-v2/service/kms` | `/websites/aws_amazon_sdk-for-go_v2_developer-guide` |
| `github.com/urfave/cli/v3` (Phase 4+) | `/urfave/cli` |

## Build and test

```
just verify    # fmt + build + lint + test (run before committing)
just build     # produces dist/kms-import
just test      # go test ./...
just fmt       # gofmt -w .
just lint      # golangci-lint run ./...
```

Toolchain versions (Go, golangci-lint, goreleaser, just) are pinned in
`mise.toml` and installed via mise. In Claude Code on the web, the
`.claude/hooks/session-start.sh` hook runs `mise install` and `just build` on
session start.

## Project layout

```
cmd/kms-import/main.go     entry point; thin wrapper over the CLI package
internal/buildinfo/        version string stamped at build time
pkg/kmsimport/             importable library (Import, KMSClient, wrapping crypto)
pkg/cli/                   CLI Command() (Phase 4+)
```

Package boundary (see the plan):

- `pkg/kmsimport/` — the importable library: `Import(ctx, opts...) (Result, error)`,
  functional options, the two-method `KMSClient` interface. **Never constructs an
  AWS SDK client and never calls `os.Exit`** — the caller injects the client.
- `pkg/cli/` — `Command()` returning a `urfave/cli` v3 `*cli.Command` that can be
  mounted as a subcommand of another app (e.g. `chinmina-bridge kms import`).
  Alias normalisation and SDK client construction live here, not in the library.

## Phasing

Implementation follows `docs/plan-kms-import.md` (Phases 1–10) with progress
tracked in `docs/progress-kms-import.md`. Phase 1 is a tracer-bullet build
(hello-world binary + CI + GoReleaser snapshot); product logic starts at
Phase 3.

## Commits and PR titles

Use Conventional Commits for all commit messages and PR titles.

| Type | When to use | Version bump |
|---|---|---|
| `feat: <description>` | new user-visible feature | minor |
| `fix: <description>` | bug fix | patch |
| `feat!:` or `BREAKING CHANGE:` in body | breaking change | major |
| `chore:`, `docs:`, `refactor:`, `test:` | maintenance, no behaviour change | none |

## Key conventions

- The library returns `(Result, error)`; only the CLI maps errors to exit codes.
- The wrapping algorithm is fixed: `RSA_AES_KEY_WRAP_SHA_256` + `RSA_4096`. Not
  configurable.
- Dependencies are injected (the `KMSClient` interface) — keep it that way for
  testability. No integration tests against real AWS live in the repo.
