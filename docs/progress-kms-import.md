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

Only things that affect later phases: scope changes, workarounds required, and decisions that constrain future work. Append as they arise.

- **Phase 1 (complete):** local quality gate green (`just fmt`/`build`/`lint`/`test`/`verify`) and `goreleaser build --snapshot --clean --single-target` builds linux/amd64. CI (Lint + Test + Build snapshot) ran green on `main` at commit `83c6234` with all actions SHA-pinned and the toolchain installed via mise.
- **CI trigger scope:** CI runs on pushes to `main` and on `pull_request`. Pushes to other branches (e.g. the `claude/*` feature branches) do not trigger CI on their own — open a PR to exercise CI for branch work.
- **`just verify` composition:** the recipe is `fmt build lint test`; `fmt` rewrites files in place (`gofmt -w .`) rather than failing on a diff. If a stricter pre-commit/CI gate is wanted later, add a separate fmt-check/`go vet` step.
- **Phase 2 (complete):** `CLAUDE.md` and the `.claude/hooks/session-start.sh` SessionStart hook (wired via `.claude/settings.json`) were already in place; validated rather than recreated. The hook runs synchronously, gates on `CLAUDE_CODE_REMOTE`, runs `mise trust/install/reshim`, persists the mise shims PATH to `$CLAUDE_ENV_FILE`, and ends with `just build || true`. Verified it exits 0 and leaves the toolchain on PATH; `just verify` green in the bootstrapped session.
- **Start hook is synchronous:** guarantees the toolchain is ready before the agent loop starts (no race), at the cost of slightly slower session startup. Switch to async mode only if startup latency becomes a concern.
