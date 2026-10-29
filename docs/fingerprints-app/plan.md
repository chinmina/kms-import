# Scaffolding plan

For the agent building the app. Each phase ends in a working, committed,
`just verify`-green state. Do the phases in order; Phase 0's outcome can change
later phases (see [Replan triggers](#replan-triggers)).

## Before you start

- Read [research.md](research.md), [design.md](design.md) and
  [architecture.md](architecture.md).
- MyGo is newer than your training data. **Fetch `docs/ui/<widget>.md` from
  <https://github.com/egoist/mygo> (at the pinned tag) before using any widget**,
  and copy its patterns; don't write MyGo code from memory.
- The prototype lives in `pkg/cli/` of `chinmina/kms-import` on branch
  `prototype/secret-fingerprints`. Port from it; keep its tests.
- Toolchain: Go 1.27.3+, `just`, `golangci-lint`, pinned in `mise.toml`.
- Conventional Commits. One commit per completed step is fine; one per phase
  is the minimum.

## Phase 0 — Spike: prove the risky widgets (½ day)

Goal: confirm MyGo can do the four things the design leans on before
investing in it.

1. `go run github.com/egoist/mygo/cmd/mygo@v0.7.3 init -template native fingerprints`;
   pin `github.com/egoist/mygo v0.7.3` in `go.mod`; add `mise.toml` and a
   `justfile` with `verify`, `dev` (`go tool mygo dev`), `build`
   (`go tool mygo build`), `test`, `lint`, `vet` (`go vet` + `go tool mygo vet`).
2. One throwaway window that has:
   - hidden title bar + `Vibrancy` + `ui.Sidebar` + a toolbar row in the
     title-bar insets;
   - a `ui.Table` of 5,000 fake rows, three sortable columns, `SortBy`,
     reorderable columns;
   - a console `ui.Table` with `FollowEnd` fed 200 lines/s from a goroutine via
     batched `win.Update`;
   - a `ui.Combobox` of 60 profile names.
3. A `ui.NewTester` test that clicks a column header and asserts the order.

**Accept when:** all four render and behave; console stays pinned to the end
and unpins on scroll-up; the app idles below 5% CPU with the console paused;
screenshots of light and dark are saved in `docs/spike/`. Record findings in
`docs/spike/README.md`. Then delete the spike code, keeping the
justfile/mise/go.mod.

## Phase 1 — Engine, no UI (1–2 days)

Goal: the prototype's behaviour as a UI-free library with events.

1. `internal/credential`: port `findCredential`, `parseCredential`,
   `githubTokenKind`, `isTokenBody`, `GitHubFingerprint`,
   `publicKeyFingerprint`, `spkiFingerprint`, `appIDField`, plus their tests.
   Import `pkg/kmsimport.KeyMaterialFromPEM` from kms-import.
2. `internal/github`: port `githubAPI` and `appJWT`. Change `inspect` /
   `inspectApp` to return structs (`TokenInfo`, `[]Installation`), not
   `secretRow` strings. Port the tests and adapt their assertions.
3. `internal/registry`: port `appRegistry`, add a mutex, take the path from
   the caller, and add `MigrateFromXDG(dst string)`.
4. `internal/scan`: `Finding`, `Detail`, `Candidate`, `Source`, `Event`,
   `Scanner.Run` with the worker pool, cancellation and per-source errors
   ([architecture.md § Scanner](architecture.md#scanner-and-events)).
   Implement `identifyApp` / `appCandidates` / `tryApp` here, recording
   `MatchedVia`.
5. `internal/scan/source`: `secretsmanager`, `ssm`, `kms`, each populating its
   `Detail` from the list/describe metadata
   ([research.md § 3](research.md#3-the-prototype-what-to-port)).
6. `internal/logview`: `Handler`, `Buffer`, JSONL sink.
7. `cmd/fingerprints-scan` (dev-only, not bundled): runs a scan for
   `--profile` and writes the prototype's CSV, so output can be diffed
   against `kms-import fingerprints` on a real account.

**Accept when:** every ported prototype test passes; the redaction test (a
reflect walk over findings, plus log buffer, JSONL and CSV, all free of seeded
secret strings) passes; a fake-source scan test proves a cancel yields
`Skipped` rows and one registry save; CSV from `fingerprints-scan` matches the
prototype's on the same account (manual, note the result in the PR).

## Phase 2 — Window shell with fixture data (1 day)

Goal: the main window looks right before it does anything.

1. `internal/app.Model`, seeded by `app.FixtureModel()` (about 40 findings
   across the three sources, every status and credential type).
2. Layout from [design.md § Main window](design.md#main-window): the three
   splits, persisted sizes, hidden title bar toolbar (profile and region
   pickers non-functional), sidebar with counts, grid with all columns,
   sorting, filtering, column layout persistence.
3. Menus with all shortcuts; View toggles work.
4. Empty states.

**Accept when:** Tester tests cover sidebar filtering (source AND status),
multi-column sort, search filter, inspector/console toggles via shortcut,
and every empty state; light and dark snapshots are committed; a manual check
against the [Mac design rules](design.md#mac-design-rules) is listed in the
PR.

## Phase 3 — Profiles, live scan, console (1–2 days)

1. `internal/awsprofile`: discovery (fixture tests), `Describe` (auth kind,
   region), `Identity` via `sts:GetCallerIdentity`, `IsSSOExpired(err)`.
2. Wire the profile and region pickers. Results are cached per `Target`;
   switching shows the cache. Re-read profiles on app activation.
3. Scan / Stop commands; the pump goroutine with 50 ms batched `win.Update`;
   streaming rows; progress (per-source spinner in the sidebar, counts).
4. Console view: follow-end, Jump to Latest, level filter, search, Copy,
   Clear, log-line → grid selection.
5. SSO banner with Sign In (`aws sso login --profile`, output to console,
   retry on success).

**Accept when:** a fake-scanner Tester test streams 500 findings and 2,000
log lines and the UI shows them all with the console at the end; cancel
works mid-scan; a fixture SSO-expired error shows the banner; manual scan of
a real sandbox profile completes.

## Phase 4 — Inspector (1 day)

1. Generic inspector rendering `Detail.Groups()` plus the credential groups
   from [design.md § Inspector](design.md#inspector-details-view).
2. Copy buttons, Open in AWS Console (region-correct console URLs for
   secret, parameter and key), Open GitHub App.
3. Grid context menu.

**Accept when:** a Tester test per source × credential type asserts the
expected labels appear; no group renders empty; copied values match.

## Phase 5 — Settings and persistence (1 day)

1. `internal/settings`: struct, defaults (`cultureamp`, `github`,
   `https://api.github.com`, concurrency 4, all sources on), versioned JSON,
   write-via-rename.
2. Settings window: four tabs, live apply, single instance, ⌘,.
3. Keychain token via `mygo.Secrets`; the `gh auth token` import button;
   Refresh Apps (org seeding) with progress in the console.
4. App cache XDG migration on first run; Clear App Cache with confirmation.
5. Scan on launch, default profile.

**Accept when:** settings survive restart (test with
`App.SetPath(PathUserData, t.TempDir())`, as MyGo's own tests do); a
disabled source isn't scanned and disappears from the sidebar; the token
never appears in `settings.json` (test).

## Phase 6 — Finish (1 day)

1. Export CSV (⇧⌘E, save sheet, prototype columns).
2. Accessibility pass: labels on all icon buttons, VoiceOver walk through
   each pane.
3. App icon; `mygo.json` metadata; `just build` produces a universal `.app` and
   `.dmg`.
4. README: install, IAM policy, the screenshots.
5. CI: `just verify` on `macos-latest` (MyGo's `docs/github-actions.md`).
   Add signing and notarization only if the owner decides to distribute.

**Accept when:** a clean checkout runs `mise install && just verify && just
build` green; the `.app` launches from Finder.

## Replan triggers

Stop and report rather than work around when:

- **Phase 0 fails on Table + FollowEnd:** switch the console to `ui.List` with
  aligned cells. If `List` also fails, use a read-only `ui.TextArea`
  appended in batches and drop log-line → grid linking.
- **Phase 0 shows the painted widgets miss the Mac rules badly** (sidebar
  vibrancy, toolbar in title bar, table header look): the owner decides
  between accepting the look, building custom widgets on `ui.*Base`, or
  leaving MyGo for an AppKit binding.
- **A MyGo upgrade breaks the build:** stay pinned. Upgrades are a separate,
  deliberate PR that runs MyGo's migration command (`docs/ui/migration.md`).
- **GitHub rate limiting** (403 with `X-RateLimit-Remaining: 0`) appears
  during real scans: add a token-bucket limiter in `internal/github`, show
  the reset time in the banner, and treat this as Phase 3 scope.

## Later

Not in v1; noted so the architecture doesn't preclude them:

- Scan several profiles or all regions in one run (`Results` is already
  keyed by `Target`).
- Persist the last results to disk (SQLite plugin or JSON) for history and
  diffs ("new since last scan").
- More sources (AppConfig, Lambda env, ECS task definitions, Buildkite
  secrets, local files).
- Expiry notifications ("3 credentials expire this week").
- Rotate an App key straight into KMS via `kmsimport.Import` from the
  inspector of a KMS finding.
- Menu-bar extra showing credentials expiring soon.
