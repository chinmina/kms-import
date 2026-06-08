# Plan: drop the `--alias` flag

> **Standalone change plan.** Self-contained; not a phase of `plan-kms-import.md`.
> The original PRD (`prd-kms-import.md`) is complete and is **reference only** —
> this plan does not edit it. Requirement IDs below (R7–R11, R25, …) refer to
> that PRD for traceability, not because the PRD is being changed.

## Why this change exists

The `--alias` target flag cannot work for the import flow and must be removed.
Two independent reasons, verified against current AWS documentation:

1. **The KMS import APIs do not accept an alias as the key identifier.** Both
   `GetParametersForImport` and `ImportKeyMaterial` document `KeyId` as *"the key
   ID or key ARN of the KMS key"* — and only those. Cryptographic operations
   (`Sign`, `Decrypt`, `DescribeKey`, …) explicitly list "Alias name" / "Alias
   ARN" as accepted `KeyId` forms; the import operations deliberately do not. The
   AWS SDK does not resolve aliases client-side, so the current code path
   (`my-app-key` → normalise to `alias/my-app-key` → pass verbatim as `KeyId`)
   would be **rejected at runtime by KMS**. It passes CI only because the unit
   tests exercise a mock `KMSClient` that never validates the identifier; the
   real-KMS path is the deferred operational smoke, so this has never been hit.
   This is the same root cause as the Phase 10 finding that `kms:RequestAlias`
   has no effect on the import actions — import operations are not in the
   alias-aware set.

2. **Even if it were accepted, it targets the wrong key in the primary use
   case.** The documented alias-based rotation runbook imports into the *new*
   key while the alias still points at the *old* one, then flips the alias. The
   import step must therefore use `--key-id`/`--key-arn`; using the alias would
   resolve to the old key. The alias only coincides with the import target for
   the very first import or an in-place reimport, where the key ID/ARN is just as
   easy to supply.

Supporting `--alias` *correctly* would require resolving the alias to a key ID
before importing (via `DescribeKey` or `ListAliases`, which do accept aliases),
which needs a third method on the library's `KMSClient` interface. That
interface is a locked two-method decision (`GetParametersForImport` +
`ImportKeyMaterial`). Rather than reopen that decision to support a flag the
rotation workflow argues against, drop the flag.

## Scope

Remove `--alias` and everything that exists solely to serve it: the flag, alias
prefixing, and the alias echo in human and JSON output. After this change the
target is specified by exactly one of `--key-id` / `--key-arn`.

### Requirements delta (relative to the reference PRD)

| PRD req | Today | After this change |
|---------|-------|-------------------|
| R7 | Exactly one of `--key-id` / `--key-arn` / `--alias` | Exactly one of `--key-id` / `--key-arn` |
| R8 | >1 of the three → error | >1 of the two → error (unchanged mechanism) |
| R9 | 0 of the three → error | 0 of the two → error (unchanged mechanism) |
| R10 | bare `--alias` gets `alias/` prepended | **removed** |
| R11 | prefixed `--alias` used as-is | **removed** |
| R25 | alias echoed in confirmation output | **removed** |
| R26 | `--json` object carries `alias` when used | `--json` object carries `keyId` + `keyState` only |

All other requirements (PEM handling, wrapping crypto, expiry, credentials,
human/JSON output of key ID + state, error handling, library/CLI contracts) are
unchanged.

## Change inventory

Concrete edits, grouped by area. Line numbers are approximate (current `main`).

### CLI — `pkg/cli/command.go`

- **`MutuallyExclusiveFlags`** (~L52–61): drop the `alias` `StringFlag` entry,
  leaving a two-flag required group (`key-id`, `key-arn`). `Required: true`
  stays, so "exactly one" (R7/R8/R9) is still framework-enforced.
- **`normaliseAlias`** (~L110–116): delete entirely (no longer referenced).
- **`resolveTarget`** (~L118–130): drop the alias branch and the `displayAlias`
  return value; it now returns a single `keyID` (from `--key-arn` or `--key-id`).
  Consider collapsing to a small expression at the call site if it reads cleaner.
- **Action** (~L63, L92): drop `displayAlias`; update the `runImport(...)` call to
  the new signature (no alias argument).

### CLI — `pkg/cli/import.go`

- **`runImport`** (~L20): remove the `alias string` parameter.
- **Confirmation line** (~L44–48): delete the `aliasPrefix` block; the message
  becomes `Imported key material — key ID: %s, state: %s`.
- **`writeJSON`** (~L65): remove the `alias` parameter.
- **`importResult`** (~L56–60): remove the `Alias` field. JSON shape is now
  `{"keyId":"…","keyState":"…"}`.
- **Doc comments** (~L15–19, L55, L63): drop alias references.

### Library — `pkg/kmsimport/import.go`

- **Doc comment only** (~L65): the comment notes the identifier may be a "key ID,
  key ARN, or alias". Drop "or alias" — the library is identifier-agnostic and
  needs **no functional change** (it accepts any string verbatim). No interface
  or behaviour change here.

### Tests — `pkg/cli/command_test.go`

- **`TestNormaliseAlias`** (~L91–108): delete (function is gone).
- **`TestCommand_NoTargetFlag_Errors`** (~L58): the asserted flag list drops
  `"alias"`, leaving `key-id`, `key-arn`.
- **`TestCommand_HelpListsFlags`** (~L169): drop `--alias` from the expected list.

### Tests — `pkg/cli/import_test.go`

- **`TestRunImport_AliasInConfirmation`** (~L66–84): delete.
- **`TestRunImport_JSONAliasOnlyWhenUsed`** (~L187–219): delete.
- All remaining `runImport(...)` calls (confirmation, JSON, expiry, failure
  tests): update to the new signature (drop the alias argument). The JSON-output
  test's schema assertion drops the `alias` key.

### Tests — `pkg/kmsimport/import_test.go`

- Calls use `WithKeyID("alias/app")` purely as an opaque identifier string;
  they still compile and pass unchanged because the library is identifier-
  agnostic. **Optional cleanup:** rename to a key-ID-shaped literal (e.g.
  `"1234abcd-…"`) so the fixtures don't imply alias support. Not required for
  correctness.

### Docs — `README.md` (in scope for this change)

- **CLI reference table:** remove the `--alias` row; update the target-selection
  prose to "exactly one of `--key-id` / `--key-arn`"; drop the alias-echo
  sentence.
- **JSON output:** update the example object to `{"keyId":…,"keyState":…}` and
  remove the "alias field present only when `--alias` was used" note.
- **Rotation runbook:** the import step already uses `--key-arn`; remove the
  now-moot "not `--alias`" parenthetical and keep the guidance that import
  targets a specific key generation by ID/ARN. The *runtime* alias switch
  (`update-alias`) and the alias-based-rotation rationale stay — those concern
  the KMS alias itself, not a `kms-import` flag.
- **IAM note:** the existing note that `kms:RequestAlias` doesn't apply to import
  (and remains correct for runtime `kms:Sign`) stays — it is reinforced, not
  contradicted, by this change.

### Docs NOT touched

- `docs/prd-kms-import.md` — reference only; the user has declared it complete.
- `docs/plan-kms-import.md`, `docs/progress-kms-import.md` — the original build
  record; this standalone plan supersedes R10/R11/R25 without rewriting history.

## Out of scope

- Any change to the library's `KMSClient` interface (stays two-method).
- Alias-to-key-ID resolution (the alternative that would *keep* `--alias`) — this
  plan explicitly chooses removal over that path.
- Wrapping crypto, expiry, PEM handling, credential resolution — untouched.

## Acceptance criteria

- [ ] `kms-import --help` lists `--key-id` and `--key-arn` and **no** `--alias`.
- [ ] Supplying `--alias` is rejected as an unknown flag.
- [ ] Exactly one of `--key-id` / `--key-arn` is still required; zero or both
      error (R8/R9 preserved over the two-flag group).
- [ ] Human-readable confirmation prints key ID + state with no alias segment.
- [ ] `--json` emits exactly `{"keyId":"…","keyState":"…"}` — no `alias` key.
- [ ] No references to alias prefixing or alias output remain in `pkg/cli` or in
      the library doc comment.
- [ ] `just verify` is green (fmt + vet + lint + test).
- [ ] README CLI reference, JSON example, and rotation runbook reflect the
      removal; no `--alias` usage example remains.

## Verification

Run `just verify`. Run the binary with `--help` (confirm no `--alias`), with
`--alias x` (confirm unknown-flag error), with neither target flag and with both
(confirm errors), and with `--json --key-id …` against the mock-backed tests
(confirm the two-field object). Grep the tree for `(?i)alias` and confirm the
only remaining hits are the legitimate KMS-alias-rotation discussion in the docs.

## Risks / notes

- **Behaviour-narrowing, not breaking a working path:** because `--alias` never
  functioned against real KMS, no working operator workflow is lost. Anyone who
  scripted `--alias` was already getting a runtime failure; they switch to
  `--key-id`/`--key-arn`, which is what the rotation runbook already prescribes.
- **TDD note for implementation:** drive each removal red-green — e.g. first
  assert `--alias` is unknown / absent from help (red against current code),
  then remove the flag (green); then tighten the JSON schema assertion (red),
  then drop the `Alias` field (green). One behaviour at a time.
