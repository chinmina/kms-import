# Plan: kms-import

> Source PRD: `prd-kms-import.md` (chinmina/kms-import) — Go binary + library that imports a GitHub App private key into AWS KMS.

## Architectural decisions

Durable decisions that apply across all phases. Lock these; don’t relitigate per phase.

- **Module path**: `github.com/chinmina/kms-import`. **Binary name**: `kms-import`.
- **Repo layout**: standalone binary in `cmd/kms-import/` (thin wrapper). The importable library (`Import`, functional options, `KMSClient`, result type) and the CLI `Command()` package both live under `pkg/` — e.g. `pkg/kmsimport/` (library) and `pkg/cli/` (Command). Exact subpackage names are flex; the `pkg/` location is locked.
- **CLI framework**: urfave/cli **v3** (stable — v3.4.1, ~20k importers). `Command()` returns `*cli.Command` so it can be mounted as a subcommand of another urfave/cli app (e.g. `chinmina-bridge kms import`).
- **Go version**: **1.26** mandated (current patch 1.26.4, 2 Jun 2026). `go.mod` declares `go 1.26`.
- **Wrapping algorithm**: `RSA_AES_KEY_WRAP_SHA_256` + wrapping key spec `RSA_4096`, **hard-coded, not configurable**. Mechanics: ephemeral AES-256 key → AES key-wrap (RFC 3394) over the key material → RSA-OAEP-SHA-256 encrypt of the AES key with the wrapping public key → concatenate `(wrapped AES key || wrapped key material)` for submission.
- **Key material conversion**: PEM type detected after `pem.Decode`. PKCS#1 (`BEGIN RSA PRIVATE KEY`, GitHub’s format) and PKCS#8 (`BEGIN PRIVATE KEY`) both normalised to PKCS#8 DER before encryption. Only RSA 2048 supported (the GitHub App key spec).
- **`KMSClient` interface**: defined in the library, exactly two methods (`GetParametersForImport`, `ImportKeyMaterial`). The AWS SDK `*kms.Client` satisfies it. **The library never constructs or configures an SDK client** — the caller injects it.
- **Alias normalisation**: lives in the CLI layer, not the library. Library accepts any valid KMS key identifier string verbatim.
- **Library contract**: returns `(result, error)`; never calls `os.Exit`. Exit-code handling is the CLI’s job only.
- **Expiry format**: RFC 3339 / ISO 8601 (e.g. `2027-01-01T00:00:00Z`), AWS-CLI-consistent.
- **Tooling**: **mise** manages toolchain versions (Go, goreleaser, golangci-lint, cosign, **just**). **just** is the command runner — build/test/lint/verify tasks are defined in a `justfile` and invoked as `just <task>`. CI installs the toolchain (including `just`) via `jdx/mise-action`, then runs `just` tasks. GitHub Actions pinned to latest **by commit SHA** (not floating tags).
- **Release**: GoReleaser v2. Split across two phases — snapshot build in CI (Phase 1, Linux only) and full tagged release with signing (Phase 9). Cosign **keyless** signing (Sigstore OIDC) via `cosign sign-blob` in the GoReleaser GHA workflow.
- **Testing boundary**: no integration tests against real AWS in the repo. Core logic is exercised against a mock `KMSClient`; real-KMS verification is a manual operational smoke.
- **Template source**: `.github/` workflows, `mise.toml`, `justfile`, and `.goreleaser.yaml` shapes are derived from `jamestelfer/dollop`. That repo was not readable during planning (private/unindexed) — the implementer copies its structure rather than reconstructing from this plan.

### Optional consideration (not locked)

- Go 1.26’s experimental opt-in `runtime/secret` package can securely wipe in-memory secret temporaries. Relevant to handling the decoded private key bytes. Evaluate during Phase 3; do **not** treat as a requirement.

## Normalization notes

The PRD requirements are already in EARS form with stable numeric IDs 1–44. No re-normalization needed. This plan refers to them as **R1–R44**, where `R{n}` is PRD requirement `{n}` verbatim. Phases 1 and 2 are infrastructure (toolchain, CI, dev environment) and map to no PRD requirement; that is intentional and approved.

## P0 baseline and standard quality gate

**Phase 1 is the P0 baseline.** The repo is greenfield, so there is no pre-existing state to stabilise — Phase 1 *establishes* the gate. The gate must be green at the end of Phase 1 and re-run before every subsequent phase completes.

Standard commands (`just` tasks; exact names may vary, behaviour is locked):

- [x] `just fmt` — `gofmt`/`go fmt ./...`, zero diff
- [x] `just lint` — `golangci-lint run`, zero findings
- [x] `just test` — `go test ./...`, all pass
- [x] `just build` — `goreleaser build --snapshot --clean --single-target` (linux/amd64) succeeds
- [x] `just verify` — composite gate (fmt-check + `go vet` + lint + test) used as the pass/fail gate per phase

If the gate fails at the end of Phase 1, fix before starting Phase 3. Do not advance phases while the gate is red.

-----

## Phase 1: Hello-world binary + CI + GoReleaser snapshot (tracer bullet / P0)

**EARS requirements**: none directly; foundation for R35 and R36 (Linux snapshot only).

### Why this phase exists

Prove the build-and-release pipeline end to end before any product logic exists. A failing release pipeline discovered late is expensive; a working snapshot build on day one de-risks everything downstream and gives every later phase a green baseline to build on.

### Locked decisions (non-negotiable)

- Module path `github.com/chinmina/kms-import`; `go 1.26` in `go.mod`.
- `main.go` lives in `cmd/kms-import/`.
- mise owns tool versions (including `just`); `just` is the command runner with tasks in a `justfile`; CI installs the toolchain via `jdx/mise-action`.
- GitHub Actions pinned by commit SHA, latest available.
- GoReleaser snapshot build in CI targets **linux/amd64 only**. No tags, no signing, no GitHub Release published this phase.

### Flex zone (implementation choice allowed)

- Exact `just` task names, `justfile`, and `mise.toml` structure (copy dollop).
- Workflow file names/structure, job matrix shape, caching.
- What the hello-world binary prints (version + usage is enough).
- Linter config (`.golangci.yaml`) ruleset.

### End-to-end behaviour to implement

`go build ./cmd/kms-import` produces a binary that runs and prints its version/usage. Pushing to a branch triggers CI: install toolchain via mise → run `just` tasks (fmt-check → vet → lint → test) → `goreleaser build --snapshot --clean` for linux/amd64. CI is green and produces a snapshot artifact.

### Acceptance criteria

- [x] `[observable]` `just build` produces a runnable linux/amd64 binary that prints version/usage.
- [x] `[observable]` CI workflow runs on push and completes green (lint + test + snapshot build).
- [x] `[observable]` `goreleaser build --snapshot --clean` succeeds locally and in CI.
- [x] `[structural]` `go.mod` declares `go 1.26`; module path is correct.
- [x] `[structural]` All GitHub Actions are pinned by commit SHA; mise installs the toolchain (including `just`).

### Verification

Push a branch; observe the CI run go green. Download the snapshot artifact from the run and execute it locally to confirm it runs. Run `just verify` locally and confirm zero findings.

### Replan triggers

- GoReleaser v2 snapshot config can’t satisfy the Linux-only build cleanly.
- `jdx/mise-action` or the SHA-pinned actions are unavailable/incompatible with Go 1.26.

-----

## Phase 2: CLAUDE.md + Claude Code web start hook

**EARS requirements**: none (dev-environment infra).

**Carry-forward**: re-run `just verify`; confirm Phase 1 CI is still green.

### Why this phase exists

Make the repo productive for agent-assisted development. A `CLAUDE.md` plus a session start hook means Claude Code web bootstraps the toolchain and knows the build/test conventions without rediscovery each session.

### Locked decisions (non-negotiable)

- `CLAUDE.md` documents the `just` commands from Phase 1, the module/binary names, and the library-vs-CLI boundary.
- A start hook bootstraps the toolchain (`mise install`, which provides `just`) on Claude Code web session start.

### Flex zone (implementation choice allowed)

- Exact hook mechanism and file location (follow current Claude Code web conventions / dollop).
- Depth and phrasing of `CLAUDE.md`.

### End-to-end behaviour to implement

Opening a Claude Code web session on the repo runs the start hook, which installs the pinned toolchain so build/test/lint work immediately. `CLAUDE.md` is present and accurate.

### Acceptance criteria

- [ ] `[observable]` A fresh Claude Code web session bootstraps the toolchain via the start hook with no manual steps.
- [ ] `[observable]` `just verify` succeeds in that bootstrapped session.
- [ ] `[structural]` `CLAUDE.md` lists the standard commands and the module/binary/layout conventions.

### Verification

Start a fresh Claude Code web session; confirm the hook runs and `just test` works without manual tool installation.

### Replan triggers

- Claude Code web start-hook mechanism differs materially from the dollop pattern.

-----

## Phase 3: Crypto + library import core

**EARS requirements**: R12, R13, R14, R15, R16, R28, R29, R30, R31, R32.

**Carry-forward**: re-run `just verify`; Phase 1 CI green.

### Why this phase exists

This is the highest-risk slice — the wrapping cryptography is unforgiving and is the whole reason the tool exists. Proving it against a mock client (and one manual real-KMS import) retires the core risk before any flag plumbing is built.

### Locked decisions (non-negotiable)

- `Import(ctx, opts...) (Result, error)` — functional options, structured `Result`, never `os.Exit` (R28, R29, R32). Lives under `pkg/` (e.g. `pkg/kmsimport/`).
- `KMSClient` interface with exactly `GetParametersForImport` + `ImportKeyMaterial` (R30); library does not build an SDK client (R31).
- `GetParametersForImport` uses `RSA_AES_KEY_WRAP_SHA_256` + `RSA_4096` (R12); import token and wrapping public key come from the **same** GPI response (R15).
- Encryption follows the locked RSA_AES_KEY_WRAP_SHA_256 mechanics (R13); `ImportKeyMaterial` submits encrypted material + that import token (R14).
- Default `ExpirationModel = KEY_MATERIAL_DOES_NOT_EXPIRE`, `ValidTo` omitted (R16).
- Library accepts key material as PKCS#8 DER (conversion from PEM is Phase 4/5’s input concern; core takes DER).

### Flex zone (implementation choice allowed)

- Internal package structure, option names, `Result` field naming.
- Whether to adopt `runtime/secret` for wiping key bytes (evaluate; optional).

### Open questions / risk burn-down

- Confirm the exact byte concatenation order and OAEP hash AWS expects for `RSA_AES_KEY_WRAP_SHA_256` against a **real** KMS key early — a mock can’t catch a wrong wire format. Do the manual real-KMS import as soon as the happy path compiles.

### End-to-end behaviour to implement

Given PKCS#8 DER key material and a `KMSClient`, `Import` performs GPI → wrap → IKM and returns a `Result` carrying key ID and resulting state. Against a real EXTERNAL-origin KMS key, the import succeeds and the key reaches `Enabled`.

### Acceptance criteria

- [ ] `[observable]` Mock-client unit test: successful import returns the expected `Result`; GPI failure and IKM failure surface as errors (no `os.Exit`).
- [ ] `[observable]` Mismatched token/public-key case returns an error.
- [ ] `[observable]` Manual real-KMS import of a test RSA-2048 key reaches state `Enabled`.
- [ ] `[structural]` `KMSClient` has exactly two methods; `*kms.Client` satisfies it; library imports no SDK-config packages.
- [ ] `[structural]` Default path sets `KEY_MATERIAL_DOES_NOT_EXPIRE` and omits `ValidTo`.

### Verification

Run the mock-client test suite. Then run a one-off real import against a throwaway EXTERNAL-origin KMS key and confirm `Enabled` via `aws kms describe-key`.

### Regression watchpoints

- N/A (first product phase) — but the wire format established here is depended on by every later phase.

### Replan triggers

- Real-KMS import rejects the encrypted material (wire-format assumption wrong) → pause and fix the crypto contract before proceeding.
- `runtime/secret` adoption turns out to materially complicate the build → drop it.

-----

## Phase 4: Minimal CLI end-to-end

**EARS requirements**: R1, R2, R6, R20, R23, R24, R33, R34.

**Carry-forward**: re-run `just verify`; re-run Phase 3 mock tests; CI green.

### Why this phase exists

Turn the library into a runnable operator tool. This is the first point the binary does a real end-to-end import from a PEM file on disk — the actual user-facing value.

### Locked decisions (non-negotiable)

- `Command()` returns a urfave/cli v3 `*cli.Command` (R33), defined in a `pkg/` package (e.g. `pkg/cli/`); `cmd/kms-import` is a thin wrapper constructing a `cli.Command`/app around it (R34).
- `--key-file` accepts a PEM path (R1); PKCS#1 detected and converted to PKCS#8 DER (R2); undecodable PEM errors out (R6).
- AWS credentials via the standard SDK chain (R20); absent `--profile`/`--region`, defer to SDK defaults (R23). The CLI constructs the SDK client and injects it into the library.
- Successful import prints a human-readable confirmation with resolved key ID + key state (R24).

### Flex zone (implementation choice allowed)

- Confirmation message wording.
- Internal wiring between `cmd/` and `Command()`.

### End-to-end behaviour to implement

`kms-import --key-file app.pem --key-id <id>` reads a PKCS#1 PEM, converts it, imports it via the Phase 3 library using SDK-default credentials, and prints a confirmation line with the key ID and `Enabled`.

### Acceptance criteria

- [ ] `[observable]` Running the binary against a real KMS key + PKCS#1 PEM imports successfully and prints key ID + state.
- [ ] `[observable]` Undecodable/garbage PEM exits non-zero with a clear error.
- [ ] `[observable]` `kms-import --help` shows the command (proves `Command()` mounts).
- [ ] `[structural]` `Command()` returns `*cli.Command` from a `pkg/` package; `cmd/` is a thin wrapper only.
- [ ] `[structural]` CLI builds the SDK client; library still receives it via injection.

### Verification

Run the binary end to end against a throwaway KMS key with a real PKCS#1 PEM; confirm output and key state. Run `--help`. Feed a malformed PEM and confirm the error + non-zero exit.

### Regression watchpoints

- Phase 3 library contract (`Import` signature, `Result`) — changing it here ripples.

### Replan triggers

- urfave/cli v3 subcommand mounting can’t satisfy the `chinmina-bridge` embedding requirement cleanly.

-----

## Phase 5: PEM format coverage + input errors

**EARS requirements**: R3, R4, R5.

**Carry-forward**: re-verify Phase 4 happy-path import + Phase 3 tests.

### Why this phase exists

Harden the input path so the tool fails clearly instead of mysteriously on the formats and errors operators will actually hit.

### Locked decisions (non-negotiable)

- `BEGIN PRIVATE KEY` treated as PKCS#8, converted to DER (R3).
- Any other PEM header → error naming the unsupported format (R4).
- Unreadable `--key-file` → error identifying file + reason (R5).

### Flex zone

- Error message phrasing.

### End-to-end behaviour to implement

PKCS#8 PEM imports identically to PKCS#1. Unsupported headers and unreadable files produce specific, actionable errors and non-zero exits.

### Acceptance criteria

- [ ] `[observable]` PKCS#8 PEM imports successfully (parity with PKCS#1).
- [ ] `[observable]` Unsupported PEM header exits non-zero, error names the format.
- [ ] `[observable]` Missing/unreadable file exits non-zero, error names file + reason.
- [ ] `[structural]` Format detection keys off the `pem.Decode` `Type` field; no format flag exists.

### Verification

Run the binary (or unit tests on the parse function) across: PKCS#8, unsupported header, missing file. Confirm exit codes and messages.

### Replan triggers

- A real GitHub App key turns out to use a header neither path handles.

-----

## Phase 6: KMS key identification + AWS config flags

**EARS requirements**: R7, R8, R9, R10, R11, R21, R22, R25.

**Carry-forward**: re-verify Phases 4–5 input/import paths.

### Why this phase exists

Operators reference keys by alias in production (the documented rotation pattern) and need profile/region control. This phase makes target selection and AWS context first-class.

### Locked decisions (non-negotiable)

- Exactly one of `--key-id` / `--key-arn` / `--alias` (R7); >1 errors (R8); 0 errors (R9).
- `--alias` without `alias/` gets the prefix prepended (R10); with it, used as-is (R11). Normalisation in the CLI layer only.
- `--profile` uses the named profile (R21); `--region` uses the specified region (R22).
- When target is `--alias`, the alias appears in confirmation output (R25).

### Flex zone

- How mutual-exclusivity is enforced (urfave/cli validation vs manual check).

### End-to-end behaviour to implement

Operator selects the target by id, arn, or alias (mutually exclusive, validated), optionally with `--profile`/`--region`. Alias is normalised and, when used, echoed in the confirmation.

### Acceptance criteria

- [ ] `[observable]` Import via `--alias my-app-key` (bare) succeeds; confirmation includes the alias.
- [ ] `[observable]` Providing two target flags exits non-zero; providing none exits non-zero.
- [ ] `[observable]` `--profile`/`--region` route the call to the intended account/region.
- [ ] `[structural]` Alias prefixing happens in the CLI layer; library receives the identifier verbatim.

### Verification

Run with bare alias, prefixed alias, two flags (expect error), no flags (expect error), and a non-default `--region`/`--profile`. Confirm behaviour and output.

### Replan triggers

- urfave/cli v3 mutual-exclusion ergonomics force an awkward UX → adjust approach.

-----

## Phase 7: Expiry + reimport

**EARS requirements**: R17, R18, R19.

**Carry-forward**: re-verify target-selection + import paths.

### Why this phase exists

Support compliance-driven expiry and the recovery path (reimport after expiry/deletion) without a separate mode.

### Locked decisions (non-negotiable)

- `--expires <RFC3339>` → `ExpirationModel = KEY_MATERIAL_EXPIRES`, `ValidTo` = supplied value (R17).
- Invalid or already-passed `--expires` → error **before any AWS call** (R18).
- Initial import and reimport use the same invocation; no flag/mode switch (R19).

### Flex zone

- Where expiry validation sits (CLI parse vs library option validation), provided it precedes any AWS call.

### End-to-end behaviour to implement

`--expires 2027-01-01T00:00:00Z` sets expiry on import. A past or malformed date fails fast with no API call made. Re-running the same command against a key with deleted/expired material reimports successfully.

### Acceptance criteria

- [ ] `[observable]` `--expires` with a valid future date sets `KEY_MATERIAL_EXPIRES` + `ValidTo` (verify via `describe-key`).
- [ ] `[observable]` Past date and malformed date both exit non-zero with **no** AWS call made.
- [ ] `[observable]` Reimport of the same material into a key with deleted material succeeds with no extra flag.
- [ ] `[structural]` Expiry validation runs before SDK calls.

### Verification

Run with valid future expiry (check `describe-key`), past date, garbage date. Then expire/delete material on a test key and reimport with the identical command.

### Regression watchpoints

- Default no-expiry path (R16) must remain unchanged when `--expires` is absent.

### Replan triggers

- AWS rejects reimport for reasons beyond the documented same-material constraint.

-----

## Phase 8: JSON output + error handling

**EARS requirements**: R26, R27.

**Carry-forward**: re-verify human-readable output (R24/R25) and all import paths.

### Why this phase exists

Machine-readable output for scripting, and consistent error semantics for automation.

### Locked decisions (non-negotiable)

- `--json` emits a JSON object with key ID, alias (if used), and key state, and **suppresses all other output** (R26).
- Any AWS API failure → non-zero exit + error to **stderr** (R27).

### Flex zone

- JSON field names (document whatever is chosen).

### End-to-end behaviour to implement

`--json` produces only the JSON object on stdout. Failures print to stderr and exit non-zero, in both human and JSON modes.

### Acceptance criteria

- [ ] `[observable]` `--json` import emits a single valid JSON object and nothing else on stdout.
- [ ] `[observable]` JSON object includes alias only when `--alias` was used.
- [ ] `[observable]` A forced AWS failure exits non-zero and writes the error to stderr.
- [ ] `[structural]` Unit test asserts the JSON schema matches the documented shape.

### Verification

Run `--json` and pipe stdout through `jq`. Force a failure (bad permissions/key) and confirm stderr + non-zero exit, JSON mode included.

### Regression watchpoints

- Human-readable output must stay clean (no stray logging) now that a quiet mode exists.

### Replan triggers

- Requirement emerges for partial/streaming output that conflicts with “suppress all other output”.

-----

## Phase 9: Release hardening (full tagged release + signing)

**EARS requirements**: R35, R36, R37, R38, R39.

**Carry-forward**: full `just verify` + a clean snapshot build before touching release config.

### Why this phase exists

Ship verifiable, multi-platform binaries operators can trust. Extends Phase 1’s snapshot pipeline into a real signed release.

### Locked decisions (non-negotiable)

- GoReleaser publishes release artifacts to GitHub Releases on tag (R35).
- Platforms: at minimum linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 (R36).
- Checksum file covering all artifacts (R37).
- Each binary signed with Cosign **keyless** (Sigstore OIDC) → signature + certificate per artifact; checksum file signed too (R38).
- README documents Cosign verification of a downloaded binary (R39).

### Flex zone

- GoReleaser archive/naming config; changelog config.
- Exact workflow trigger/permissions wiring (copy dollop’s keyless OIDC setup).

### End-to-end behaviour to implement

Pushing a version tag triggers GoReleaser: builds all five targets, generates a checksum file, signs each artifact and the checksum file with keyless Cosign, and publishes a GitHub Release with binaries, signatures, and certificates.

### Acceptance criteria

- [ ] `[observable]` A test tag produces a GitHub Release with all five platform binaries.
- [ ] `[observable]` Checksum file is present and covers every artifact.
- [ ] `[observable]` Each binary has a Cosign signature + certificate; `cosign verify-blob` succeeds following the README steps.
- [ ] `[structural]` GHA workflow uses `sigstore/cosign-installer` and keyless `cosign sign-blob`; OIDC permissions set.
- [ ] `[structural]` README verification section matches the actual artifact names/commands.

### Verification

Push a throwaway pre-release tag; inspect the resulting GitHub Release. Download a binary + its signature/cert and run the README’s `cosign verify-blob` command; confirm it passes.

### Regression watchpoints

- Phase 1 CI snapshot build must keep working alongside the new release workflow.

### Replan triggers

- GoReleaser v2 or cosign keyless flow changes break the documented signing path.
- OIDC trust/permissions can’t be granted in the target org.

-----

## Phase 10: Documentation

**EARS requirements**: R40, R41, R42, R43, R44.

**Carry-forward**: confirm CLI flag surface (Phases 4–8) and verification steps (Phase 9) are final before documenting them.

### Why this phase exists

The tool is only adoptable if operators understand the flags, the security rationale, the minimum IAM, and the rotation workflow. Docs last, so they describe shipped behaviour rather than intentions.

### Locked decisions (non-negotiable)

- CLI reference covering every flag: type, default, mutual-exclusivity constraints (R40).
- Security rationale for KMS-backed signing vs plaintext: non-extractability, IAM + resource-policy access control, CloudTrail audit, alias-based rotation (R41).
- Minimum IAM as an actual policy document (R42), **separating** key-resource-policy permissions from caller IAM permissions (R43) — matching the chinmina-bridge KMS docs pattern (`kms:GetParametersForImport` + `kms:ImportKeyMaterial`, scoped by `kms:RequestAlias` where an alias is the target).
- Recommended alias-based rotation workflow (R44).

### Flex zone

- Doc structure, headings, examples beyond the required content.

### End-to-end behaviour to implement

README (and any `docs/`) carries an accurate CLI reference, the security rationale, the split IAM policy fragments, and the rotation runbook.

### Acceptance criteria

- [ ] `[structural]` Every flag from the implemented CLI appears in the reference with type, default, and constraints.
- [ ] `[structural]` Security rationale covers all four listed points.
- [ ] `[structural]` IAM section gives a copy-pasteable policy doc, with caller-IAM and key-resource-policy fragments clearly separated.
- [ ] `[observable]` Following the rotation workflow as written produces a working rotated key (dry-run/manual walk-through against a test key).

### Verification

Cross-check the flag reference against `--help` output. Apply the documented IAM fragments to a test principal/key and confirm an import works with exactly those permissions (no more). Walk the rotation runbook against a test alias.

### Replan triggers

- Documented minimum IAM proves insufficient when tested against a least-privilege principal → correct the policy and re-verify.

-----

## Requirements coverage matrix

|Requirement ID|Phase(s)                         |Notes                                   |
|--------------|---------------------------------|----------------------------------------|
|R1            |4                                |`--key-file` PEM path                   |
|R2            |4                                |PKCS#1 → PKCS#8 DER                     |
|R3            |5                                |PKCS#8 → DER                            |
|R4            |5                                |Unsupported header error                |
|R5            |5                                |Unreadable file error                   |
|R6            |4                                |Undecodable PEM error                   |
|R7            |6                                |Exactly one target flag                 |
|R8            |6                                |>1 target → error                       |
|R9            |6                                |0 targets → error                       |
|R10           |6                                |Alias prefix prepend                    |
|R11           |6                                |Prefixed alias as-is                    |
|R12           |3                                |GPI RSA_AES_KEY_WRAP_SHA_256 / RSA_4096 |
|R13           |3                                |Encrypt per wrapping algo               |
|R14           |3                                |ImportKeyMaterial w/ token              |
|R15           |3                                |Same-response token + pubkey            |
|R16           |3                                |Default no-expiry                       |
|R17           |7                                |`--expires` sets EXPIRES + ValidTo      |
|R18           |7                                |Invalid/past expiry → pre-call error    |
|R19           |7                                |Reimport, no mode switch                |
|R20           |4                                |SDK credential chain                    |
|R21           |6                                |`--profile`                             |
|R22           |6                                |`--region`                              |
|R23           |4                                |SDK defaults when flags absent          |
|R24           |4                                |Human-readable confirmation             |
|R25           |6                                |Alias echoed in output                  |
|R26           |8                                |`--json`, suppress other output         |
|R27           |8                                |AWS failure → non-zero + stderr         |
|R28           |3                                |`Import` callable without CLI           |
|R29           |3                                |Functional options                      |
|R30           |3                                |`KMSClient` two-method interface        |
|R31           |3                                |Library builds no SDK client            |
|R32           |3                                |Structured result + error, no `os.Exit` |
|R33           |4                                |`Command()` returns `*cli.Command`      |
|R34           |4                                |Thin `cmd/` wrapper                     |
|R35           |1 (foundation) → 9 (complete)    |Snapshot in P1; tagged release in P9    |
|R36           |1 (linux/amd64) → 9 (full matrix)|Linux snapshot P1; 5 platforms P9       |
|R37           |9                                |Checksum file                           |
|R38           |9                                |Cosign keyless signing                  |
|R39           |9                                |README verification instructions        |
|R40           |10                               |CLI reference                           |
|R41           |10                               |Security rationale                      |
|R42           |10                               |Minimum IAM policy doc                  |
|R43           |10                               |IAM: resource policy vs caller IAM split|
|R44           |10                               |Alias-based rotation workflow           |

Phases 1 and 2 carry no PRD requirement (toolchain/CI and dev-environment infrastructure, front-loaded by decision). All of R1–R44 are covered; R35/R36 are intentionally split across Phases 1 and 9.
