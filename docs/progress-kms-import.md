# Progress: kms-import

Tracks implementation progress against [`plan-kms-import.md`](./plan-kms-import.md). Check off each phase as its acceptance criteria and quality gate (`mise run verify`) pass.

## Phases

- [x] **Phase 1** — Hello-world binary + CI + GoReleaser snapshot (tracer bullet / P0)
- [x] **Phase 2** — CLAUDE.md + Claude Code web start hook
- [x] **Phase 3** — Crypto + library import core (R12–R16, R28–R32)
- [x] **Phase 4** — Minimal CLI end-to-end (R1, R2, R6, R20, R23, R24, R33, R34)
- [ ] **Phase 5** — PEM format coverage + input errors (R3, R4, R5)
- [ ] **Phase 6** — KMS key identification + AWS config flags (R7–R11, R21, R22, R25)
- [ ] **Phase 7** — Expiry + reimport (R17, R18, R19)
- [ ] **Phase 8** — JSON output + error handling (R26, R27)
- [ ] **Phase 9** — Release hardening: full tagged release + signing (R35–R39)
- [ ] **Phase 10** — Documentation (R40–R44)

## Lessons learned

Carry-forward knowledge that changes how later phases are built: scope changes, workarounds, and decisions that constrain future work. This is not a status log — phase completion is tracked by the checkboxes above. Append entries only when they will affect a future phase.

- **CI trigger scope:** CI runs only on pushes to `main` and on `pull_request`. Pushes to other branches (e.g. the `claude/*` feature branches) do not trigger CI — open a PR to exercise CI for branch work.
- **Wrapping is RFC 5649, not RFC 3394 (Phase 3):** KMS `RSA_AES_KEY_WRAP_SHA_256` wraps the key material with *AES Key Wrap with Padding* (RFC 5649, OpenSSL `id-aes256-wrap-pad`, AIV `0xA65959A6`), **not** plain RFC 3394. The wire format submitted to `ImportKeyMaterial` is `RSA-OAEP-SHA256(AES key) || RFC5649-wrap(key material)` — RSA part first. The crypto is verified by RFC 5649 known-answer tests in `pkg/kmsimport/wrap_test.go`; do not change the concat order or padding without re-checking the KATs.
- **Library API (Phase 3):** `pkg/kmsimport.Import(ctx, opts...) (Result, error)` with options `WithClient`, `WithKeyID`, `WithKeyMaterial` (PKCS#8 DER). Sentinel errors `ErrNoClient`/`ErrNoKeyID`/`ErrNoKeyMaterial` are returned for missing inputs before any AWS call. `Result.KeyState` is inferred as `"Enabled"` on success because the two-method `KMSClient` interface (locked) cannot call `DescribeKey`. Phase 4 CLI must build the SDK client and inject it via `WithClient`.
- **Outstanding manual smoke (Phase 3):** the real-KMS import acceptance criterion (import a throwaway RSA-2048 key into an EXTERNAL-origin KMS key, confirm `Enabled`) could **not** be run in the sandbox (no AWS credentials/network policy). The wire format is covered by KATs + a round-trip unit test, but the live import remains an operational check to perform before relying on the tool in production. **Phase 4 inherits this:** the binary's end-to-end run against a real KMS key is likewise unverified in the sandbox — same operational smoke covers both.
- **CLI seam (Phase 4):** the testable boundary in `pkg/cli` is `runImport(ctx, out io.Writer, client kmsimport.KMSClient, keyID string, pemBytes []byte)` — it decodes the PEM, calls `kmsimport.Import`, and writes the confirmation. `Command()`'s Action does only flag parsing, `os.ReadFile`, and SDK client construction (`config.LoadDefaultConfig` → `kms.NewFromConfig`) above it. Later phases extend, not replace: Phase 5 grows `decodeKeyMaterial` (PKCS#8/unsupported-header/file-read errors), Phase 6 adds the `--key-arn`/`--alias` target flags + `--profile`/`--region` (route into `LoadDefaultConfig`), Phase 8's `--json` must route output through `runImport` (the single Fprintf is the only stdout write today). PKCS#8 and unsupported-header PEMs are **not** handled yet — only PKCS#1 (`BEGIN RSA PRIVATE KEY`) is converted.
