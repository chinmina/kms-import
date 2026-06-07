# Progress: kms-import

Tracks implementation progress against [`plan-kms-import.md`](./plan-kms-import.md). Check off each phase as its acceptance criteria and quality gate (`mise run verify`) pass.

## Phases

- [x] **Phase 1** — Hello-world binary + CI + GoReleaser snapshot (tracer bullet / P0)
- [x] **Phase 2** — CLAUDE.md + Claude Code web start hook
- [ ] **Phase 3** — Crypto + library import core (R12–R16, R28–R32)
- [ ] **Phase 4** — Minimal CLI end-to-end (R1, R2, R6, R20, R23, R24, R33, R34)
- [ ] **Phase 5** — PEM format coverage + input errors (R3, R4, R5)
- [ ] **Phase 6** — KMS key identification + AWS config flags (R7–R11, R21, R22, R25)
- [ ] **Phase 7** — Expiry + reimport (R17, R18, R19)
- [ ] **Phase 8** — JSON output + error handling (R26, R27)
- [ ] **Phase 9** — Release hardening: full tagged release + signing (R35–R39)
- [ ] **Phase 10** — Documentation (R40–R44)

## Lessons learned

Carry-forward knowledge that changes how later phases are built: scope changes, workarounds, and decisions that constrain future work. This is not a status log — phase completion is tracked by the checkboxes above. Append entries only when they will affect a future phase.

- **CI trigger scope:** CI runs only on pushes to `main` and on `pull_request`. Pushes to other branches (e.g. the `claude/*` feature branches) do not trigger CI — open a PR to exercise CI for branch work.
- **`just verify` does not fail on unformatted code:** the recipe is `fmt build lint test`, and `fmt` rewrites files in place (`gofmt -w .`) instead of failing on a diff. A stricter gate (fmt-check + `go vet`) would need to be added separately.
