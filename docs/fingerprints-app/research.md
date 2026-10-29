# Research

Researched 2026-10-29 against MyGo commit `c18c0df` (v0.7.3). Paths like
`docs/ui/table.md` are in <https://github.com/egoist/mygo>; the site mirrors
them at `https://mygo.egoist.dev/docs/ui/<page>`. MyGo postdates most model
training data: **fetch the current page before using any API named here**, and
re-check this doc after a MyGo upgrade.

## 1. MyGo

### What it is

- Pure Go, no cgo. Calls AppKit and Metal through `purego` (`go.mod`,
  `backend_darwin.go`). Builds without Xcode.
- Two window kinds: system webview, or **native UI** "drawn by MyGo itself on
  the GPU: no HTML, no JavaScript and no webview" (`README.md`). We use only
  native UI.
- Native UI widgets are **painted imitations of AppKit**, not `NSTableView` /
  `NSSplitView` / `NSToolbar`. Text is shaped by Core Text in the system font
  (`docs/ui/text.md`). Real system APIs: menu bar and context menus
  (`docs/menus.md`), dialogs, notifications, clipboard, shell, Keychain
  (`docs/native.md`).
- Immediate mode: a `func(c *ui.Context)` view runs on the main thread each
  frame. State lives in your structs; widgets take pointers to fields
  (`docs/ui/views.md`). No bindings, no retained widget tree.
- Maturity: created 2026-09-26, v0.7.3 on 2026-10-29, a release every 1–3 days,
  ~3.6k stars, 91 open issues, MIT. `docs/ui/migration.md` documents a source
  migration command — **expect breaking changes; pin the version**.
- Requires **Go 1.27.3+** (`go.mod`). macOS minimum default 12.0.

### Toolchain

| Step | Command | Source |
| --- | --- | --- |
| Scaffold | `go run github.com/egoist/mygo/cmd/mygo@v0.7.3 init -template native <dir>` | `docs/getting-started.md` |
| Dev loop (live rebuild) | `go tool mygo dev` | `docs/cli.md` |
| Bundle | `go tool mygo build` → `build/darwin-arm64/<Name>.app` + `.dmg`; `-platform darwin/universal` | `docs/distribution.md` |
| Check machine | `go tool mygo doctor` | `docs/cli.md` |
| Lint lifetimes | `go tool mygo vet` (elements stored or used off-frame) | `docs/ui/views.md` |

Native apps are configured by `mygo.json` (name, identifier, version, `macos.*`
signing/notarization/entitlements/Info.plist) — `docs/configuration.md`.
Default signing is ad hoc, which runs only on the building Mac. Dev builds are
named `<Name> Dev` with identifier `<identifier>.dev`, so their data directory
and Keychain entries are separate from the installed app's.

### API surface we depend on (signatures checked in `ui/api_gen.go`)

| Need | API | Notes |
| --- | --- | --- |
| Window | `mygo.NewWindow(mygo.WindowOptions{Title, Width, Height, MinWidth, MinHeight, StateKey, TitleBarStyle, Vibrancy, Content: ui.View(fn)})` | `StateKey` persists bounds (`docs/windows.md`) |
| Unified title bar | `TitleBarStyle: mygo.TitleBarHidden` + `c.TitleBar()` insets + `.DragWindow()` | `docs/ui/windows.md § Title bars` |
| Sidebar material | `WindowOptions.Vibrancy`, `c.Vibrancy()` | `docs/ui/windows.md § Vibrancy` |
| Source list | `ui.Sidebar(c, *string, fn)`, `SidebarSection`, `SidebarItem(...).Children(Badge)` | `docs/ui/sidebar.md` |
| Panes | `ui.Split(c, *float32, a, b)`, `ui.SplitVertical` | size pointer persists the divider |
| Toolbar | `ui.Toolbar(c, fn)`, `ui.SearchField(c, *string)` | overflow menu built in |
| Grid | `ui.Table(c, *ListState, []TableColumn, n, cell)` | `TableColumn{ID, Title, Width, Align, Sortable, Fixed, RowHeader}` |
| Sorting | `ListState.Sort *SortOrder` / `SortBy *[]SortOrder`; `SortOrder{Column, Descending}` | **the app sorts**; table renders rows in the order given |
| Column layout persistence | `ListState.Columns` (`TableLayout`, by column `ID`) | `docs/ui/table.md § Arranging columns` |
| Stable rows | `ListState.Key func(row) any` | duplicate keys panic in the tester |
| Selection | `ListState.Selected *int`, `Selection` (multi) | |
| Live log | `ui.List` / `ui.Table` with `ListState.FollowEnd`, `AtEnd()`, `ScrollToEnd()` | virtualized: "millions of rows" (`docs/ui/list.md`) |
| Settings | `ui.Form`, `ui.Field`, `ui.Fieldset`, `ui.Switch`, `ui.Select`, `ui.Combobox`, `ui.TextInput`, `ui.Tabs` | `docs/ui/form.md` |
| Profile picker | `ui.Select(c, *string, []string)` or `ui.Combobox` (type to filter) | painted, not `NSPopUpButton` |
| Background → UI | `win.Update(func(){ ... })` (runs on main, redraws) | the only safe way to mutate view state off-main |
| Menus | `mygo.NewMenu`, `mygo.App.SetMenu`, roles, `Accelerator: "CmdOrCtrl+,"` | no Preferences role: add a Settings… item |
| Keychain | `mygo.Secrets.Set/Get/Delete`, `mygo.ErrSecretNotFound` | ≤ 2560 bytes; `docs/native.md § Secrets` |
| Data dirs | `mygo.App.Path(mygo.PathUserData | PathCache | PathLogs)` | `~/Library/Application Support/<Name>` etc. |
| Save dialog | `mygo.Dialog.Save(...)` | blocking: call from a goroutine |
| Open links | shell `OpenExternal` | AWS console / GitHub App pages |
| Clipboard | `mygo.Clipboard.WriteText` | |
| Theme | `c.Theme()` follows light/dark, accent, contrast, reduce motion | `docs/ui/styling.md` |
| Fonts | `Font("monospace")`, `FontFeatures("tnum")` | fingerprints, log, numbers |
| Icons | `ui.Icon` with SVG (`ui.MustParseSVG`) | **no SF Symbols**; use Lucide (ISC licence) |
| Headless tests | `ui.NewTester(view, w, h)`: `Click`, `Type`, `Key`, `HasText`, `Texts`, `Advance`, `Image()` | `docs/ui/testing.md`, `examples/counter-native/main_test.go` |

Verbatim snippets for each of these are in `docs/ui/<widget>.md`. The
`examples/notes` and `examples/gallery` apps are the closest whole-app
references (sidebar + list + detail, and every widget respectively).

### Gaps and how the design absorbs them

| Gap | Impact | Mitigation |
| --- | --- | --- |
| Painted, not AppKit | No system toolbar customization, no native sidebar collapse animation, text-field Services/spellcheck undocumented | Follow [design.md § Mac design rules](design.md#mac-design-rules); keep chrome minimal so differences are few |
| No SF Symbols | Icons look less native | Lucide SVGs at 16 px, 1.5 px stroke, tinted with text colour — closest to SF Symbols' regular weight |
| No settings store | — | `settings.json` in `PathUserData`, written via rename (prototype's `save()` pattern) |
| Virtualized rows aren't selectable text across rows | Can't drag-select the log | "Copy" / "Copy All Visible" commands on the log, writing formatted lines to the clipboard |
| `FollowEnd` documented on `List`; on `Table` unverified | Log may not stick to the end | Phase 0 spike checks it; fall back to `ui.List` rendering column-aligned rows |
| API churn | Scaffold breaks on upgrade | Pin `v0.7.3` in `go.mod`; upgrade deliberately; run `mygo vet` in `just verify` |
| Notifications only in packaged apps | "Scan finished" notification absent under `go run` | Feature-flag on `mygo dev` builds; not needed in v1 |

## 2. AWS profiles

The SDK has **no public API to list profiles**. `config.LoadSharedConfigProfile`
loads one named profile; the INI parser is `internal/ini` and can't be
imported. So:

1. Read the file names from `config.DefaultSharedConfigFiles` and
   `config.DefaultSharedCredentialsFiles`. Those already honour
   `AWS_CONFIG_FILE` / `AWS_SHARED_CREDENTIALS_FILE`.
2. Scan for section headers: in the config file `[default]` and `[profile X]`
   (skip `[sso-session X]` and `[services X]`); in the credentials file `[X]`.
   Union, sorted, `default` first. ~40 lines with tests on fixture files.
3. For each profile, `config.LoadSharedConfigProfile` gives the
   `SharedConfig` fields used to label it in the picker: `Region`,
   `SSOSessionName` / `SSOStartURL` / `SSOAccountID` (SSO), `RoleARN`
   (assume-role), `CredentialProcess`.
4. Re-read on app activation (`OnDidBecomeActive`) so profiles added in a
   terminal appear without a restart.

**Expired SSO sessions** surface as `*ssocreds.InvalidTokenError`
(`credentials/ssocreds/sso_credentials_provider.go`). A GUI can't run the
device flow inline, so the app shows a banner offering to run
`aws sso login --profile <p>` (via `os/exec`, output streamed to the log) and
retry. Credentials are resolved before any scan starts with an
`sts:GetCallerIdentity` call, which also yields the account ID shown in the
toolbar and stamped on every finding.

**Netskope / TLS:** the app uses Go's transport, which reads the macOS system
trust store, so Netskope's injected CA works without configuration (unlike the
Docker case in the user's notes). MyGo's own HTTP client follows System
Settings proxies (`docs/native.md § Proxies`); the AWS and GitHub clients use
`net/http` defaults, which honour `HTTPS_PROXY` only.

## 3. The prototype (what to port)

Source: `pkg/cli/` on `prototype/secret-fingerprints`. ~1,550 lines plus
~900 lines of tests.

| File | Responsibility | Port as |
| --- | --- | --- |
| `fingerprints.go` | `findCredential` (bare PEM/token, or flat JSON — strongest candidate wins), token-prefix classification, `identifyApp` (cached → stored ID → every App), fingerprint computation, `describeValue` | `internal/scan/credential.go`, `identify.go` |
| `sources.go` | SSM `DescribeParameters` + `GetParameter`; KMS `ListKeys` + `ListAliases` + `DescribeKey` + `GetPublicKey`, `kmsSigner` (JWTs signed in KMS) | `internal/scan/source/{ssm,kms}.go` |
| `fingerprints.go` (`scanSecrets`) | Secrets Manager `ListSecrets` + `GetSecretValue` | `internal/scan/source/secretsmanager.go` |
| `github.go` | `githubAPI`: token inspection (`/user`, scopes header, repo count via `Link rel=last`), installation tokens, App identity via JWT, installations + granted permissions, org App seeding, host-pinned `get` | `internal/github/` |
| `appregistry.go` | JSON cache: fingerprint → App, misses (never expire), hashed invalid tokens (domain-separated SHA-256), atomic save | `internal/registry/` |

Behaviours to keep, each already covered by a prototype test:

- Error strings are short and fixed; AWS errors are reduced to their code
  (`awsFailure`). Parse errors never echo key material.
- `githubAPI.get` refuses `Link` URLs off the configured host, so a token is
  never sent elsewhere.
- Refresh tokens (`ghr_`) are never probed. Installation tokens use
  `/installation/repositories`. Classic tokens report `(all)` repos.
- App inspection never mints installation tokens (read-only by design).
- KMS keys are matched by signing in KMS, so imported keys are identified even
  though the PEM no longer exists.

What the prototype discards and the app needs:

- **Metadata already in hand:** `SecretListEntry` (Description, KmsKeyId,
  Created/LastChanged/LastAccessed/LastRotated/NextRotation dates,
  RotationEnabled, OwningService, PrimaryRegion, Tags, Type),
  `ParameterMetadata` (Type, Tier, Version, DataType, KeyId, LastModifiedDate,
  LastModifiedUser, Description, Policies), `KeyMetadata` (Origin, KeySpec,
  KeyState, KeyManager, CreationDate, ValidTo, ExpirationModel,
  SigningAlgorithms, MultiRegion, Description, CurrentKeyMaterialId). No extra
  API calls needed.
- **Structured results:** `secretRow` flattens installations and permissions
  into `;`-joined strings. The app needs them as structs.
- **Progress events:** the prototype is silent until the CSV is written.
- **Concurrency safety:** `appRegistry` has no lock; scanning sources in
  parallel needs one.
- **How an App was matched** (cache, stored ID, or search), and the JSON field
  the credential was found in: worth showing in the inspector.
