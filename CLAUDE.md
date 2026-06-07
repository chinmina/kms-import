# kms-import

CLI tool and Go library that imports a GitHub App private key (PEM) into AWS KMS
as non-extractable key material. See `docs/prd-kms-import.md` and
`docs/plan-kms-import.md` for the spec and phased plan;
`docs/progress-kms-import.md` tracks status.

## Authoritative documentation (mandatory)

Go 1.26 and the AWS SDK here postdate the training cutoff — do not answer API
questions from memory. Fetch current docs before using an unfamiliar API:

- **Context7** for language/SDK/framework APIs, using these IDs (resolve others
  as needed):
  - Go 1.26 stdlib (`crypto/*`, `encoding/pem`, …): `/websites/pkg_go_dev_std`
  - `aws-sdk-go-v2/service/kms`: `/websites/aws_amazon_sdk-for-go_v2_developer-guide`
  - `github.com/urfave/cli/v3` (Phase 4+): `/urfave/cli`
- **AWS Knowledge Base MCP** (`aws___search_documentation` /
  `aws___read_documentation`) for AWS *service* behaviour: KMS import/wrapping
  mechanics, exact wire formats, IAM policy shapes. The crypto is unforgiving —
  verify wire formats here, not from memory.

## Build and test

Run `just verify` (fmt + build + lint + test) before committing; `just build`
produces `dist/kms-import`. Toolchain versions are pinned in `mise.toml`; the
Claude Code web session-start hook installs them.

## Architecture

```
cmd/kms-import/      thin binary wrapper
pkg/kmsimport/       importable library: Import, KMSClient, wrapping crypto
pkg/cli/             CLI Command() (Phase 4+)
internal/buildinfo/  build-time version string
```

- `pkg/kmsimport/` — `Import(ctx, opts...) (Result, error)` with functional
  options and the two-method `KMSClient` interface. **Never constructs an SDK
  client and never calls `os.Exit`** — the caller injects the client.
- `pkg/cli/` — `Command()` returns a `urfave/cli` v3 `*cli.Command` (mountable as
  a subcommand). SDK client construction, alias normalisation, and exit-code
  mapping live here, not in the library.
- Wrapping algorithm is fixed at `RSA_AES_KEY_WRAP_SHA_256` + `RSA_4096` — not
  configurable. No integration tests against real AWS live in the repo.

## Conventions

Conventional Commits for commit messages and PR titles (`feat`, `fix`, `chore`,
`docs`, `refactor`, `test`; `feat!` / `BREAKING CHANGE:` for breaking changes).
