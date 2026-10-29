# Fingerprints app — research and plan

The `kms-import fingerprints` prototype (an uncommitted CLI subcommand on the
`prototype/secret-fingerprints` branch) is moving into its own macOS desktop
app, built with [MyGo](https://mygo.egoist.dev/docs/ui)'s native UI. These docs
are the brief for an agent scaffolding that app. Read them in order:

| Doc | What it settles |
| --- | --- |
| [research.md](research.md) | What MyGo is and isn't, verified API surface, AWS profile discovery, what the prototype already does |
| [design.md](design.md) | Window layout, every view, interaction and keyboard model, Mac design rules |
| [architecture.md](architecture.md) | Module layout, domain model, `Source` interface, log pipeline, concurrency, persistence, security invariants |
| [plan.md](plan.md) | Phased scaffolding plan with acceptance criteria and verification per phase |

## The app in one paragraph

A single-window Mac app that scans one AWS profile at a time for GitHub
credentials across pluggable secret sources (Secrets Manager, SSM Parameter
Store, KMS signing keys to start). It shows each finding in a sortable grid and
explains the selected one in an inspector tailored to its source, while a
console pane streams the structured log of the scan as it happens. A Settings
window holds the GitHub org (default `cultureamp`), API URL, name filter,
per-source toggles and the GitHub token (in the login Keychain).

## Decisions taken in these docs

1. **MyGo native UI, accepting that it is painted, not AppKit.** MyGo draws its
   widgets on the GPU (Metal) and imitates AppKit; only the menu bar, dialogs,
   notifications and Keychain are system APIs. That meets "native, not web" —
   no webview, no HTML — but Mac fidelity is a design discipline, not a given.
   [design.md § Mac design rules](design.md#mac-design-rules) carries that load.
2. **Separate repository and Go module.** MyGo needs Go ≥ 1.27.3, a `go tool`
   CLI, and releases every 1–3 days; kms-import is a small, stable, static CLI.
   Coupling their toolchains and release cadence costs more than it saves. The
   app imports `github.com/chinmina/kms-import/pkg/kmsimport` for PEM decoding.
3. **The scanner is a UI-free library inside the app module** (`internal/scan`),
   so 90%+ of the logic is tested with plain `go test` and fakes, as the
   prototype already is. The UI only renders `scan` events.
4. **One profile per scan, results kept per profile.** Switching profile shows
   that profile's last results instantly; scanning many accounts at once is a
   later feature (see [plan.md § Later](plan.md#later)).
5. **Secret values never leave the scanner.** Not into findings, the log, the
   clipboard or the UI. The prototype's fixed-error-message discipline is a
   tested invariant.
6. **CSV export keeps parity** with the prototype's 10-column output, so
   nothing is lost when the CLI subcommand is removed.

## Open questions for the owner

- App name, bundle identifier and repo location (docs assume
  `github.com/chinmina/fingerprints`, identifier `io.chinmina.fingerprints`).
- Distribution: ad-hoc signed local builds only, or Developer ID signing and
  notarization (needs an Apple Developer account and CI secrets)?
- Should the `fingerprints` subcommand be deleted from kms-import once the app
  ships, or kept as a thin CLI over the same `scan` package?
- **The prototype is uncommitted.** Commit `prototype/secret-fingerprints`
  (`pkg/cli/{fingerprints,github,appregistry,sources}.go` and tests) before
  starting — Phase 1 ports from it.
