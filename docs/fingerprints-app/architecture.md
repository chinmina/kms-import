# Architecture

## Repository layout

```text
fingerprints/
├── main.go                 mygo.App setup, menus, windows; nothing else
├── mygo.json               name, identifier, version, macos.*
├── go.mod                  go 1.27.3; tool github.com/egoist/mygo/cmd/mygo; mygo pinned
├── justfile                verify = fmt + vet + mygo vet + lint + test; dev; build
├── mise.toml               go, golangci-lint, just
├── resources/icon.png      1024×1024
├── internal/
│   ├── scan/               UI-free scanning engine: Scanner, Finding, events
│   │   └── source/         one file per Source: secretsmanager, ssm, kms
│   ├── credential/         findCredential, token kinds, fingerprints (pure functions)
│   ├── github/             GitHub API client (port of github.go)
│   ├── registry/           App cache (port of appregistry.go) + mutex
│   ├── awsprofile/         profile discovery, identity check, SSO-expiry detection
│   ├── settings/           Settings struct, load/save JSON, Keychain token
│   ├── logview/            slog.Handler → ring buffer → UI; JSONL file sink
│   └── app/                the UI: model, views, commands (only package importing mygo/ui)
└── testdata/               AWS config fixtures, recorded GitHub responses
```

Dependency direction is strictly downwards:
`main → app → {scan, settings, awsprofile, logview} → {credential, github,
registry}`. Only `main` and `app` import MyGo. Everything else builds and
tests with plain `go test`, as the prototype does today.

## Domain model (`internal/scan`)

```go
type Status string // Valid, Invalid, UnknownApp, Error, Skipped

type Finding struct {
	ID          string // source kind + account + region + resource ARN/ID; stable across scans
	Source      SourceKind
	Name        string
	ARN         string
	Target      Target // Profile, Region, AccountID
	Credential  CredentialKind
	JSONField   string // field the credential was found in, if the value was JSON
	Fingerprint string
	App         *AppMatch   // ID, Slug, Owner, MatchedVia, Installations []Installation
	Token       *TokenInfo  // User, Scopes []string, Expires time.Time, Repos RepoAccess
	Status      Status
	Reason      string      // short fixed text: "read failed: AccessDeniedException"
	Detail      Detail      // source-specific metadata, never secret material
	ScannedAt   time.Time
}

// Detail renders itself into labelled groups so the inspector stays generic.
type Detail interface {
	Groups() []DetailGroup // Title + []DetailRow{Label, Value, Kind(Text|Mono|Time|Link|Chips)}
}
```

`Installation{Account, Selection, Permissions map[string]string}` replaces the
prototype's `;`-joined strings. A `Finding.CSV() []string` keeps the
prototype's 10 columns (`source,name,type,fingerprint,app,user,scopes,repos,
expires,status`) for export.

## Sources

```go
type Source interface {
	Kind() SourceKind
	Title() string                     // "Secrets Manager"
	Options() []Option                 // per-source settings, rendered by the Settings window
	Scan(ctx context.Context, env Env, emit func(Candidate)) error
}

// Candidate is what a source found before credential analysis. Read returns
// the raw value; it is called once, by the scanner, and the bytes are dropped
// after analysis.
type Candidate struct {
	Name, ARN string
	Detail    Detail
	Read      func(ctx context.Context) ([]byte, error) // Secrets Manager, SSM
	Signer    credential.Signer                         // KMS: sign in place, no value
	PublicKey []byte                                    // KMS: SPKI DER for the fingerprint
}

type Env struct {
	AWS    aws.Config
	Target Target
	Match  string      // name filter, already lower-cased
	Log    *slog.Logger // pre-tagged with source=<kind>
}
```

- Sources do **listing and reading only**. The shared analysis pipeline
  (`findCredential` → token inspection or App identification) runs in
  `scan`, so every source gets identical GitHub handling and the security
  invariants live in one place.
- Registration is a slice in `scan/source/registry.go`. Adding a source is one
  file plus one line, with no UI changes. Candidate future sources: AWS
  AppConfig, Lambda environment variables, ECS task-definition secrets,
  Buildkite (cluster) secrets, 1Password, local `.env`/PEM files.
- Each source's AWS clients sit behind narrow interfaces, as in the prototype
  (`secretsClient`, `paramsClient`, `kmsKeysClient`), so fakes drive the tests.

## Scanner and events

```go
type Event interface{ isEvent() }
// Started{Target, Sources}, SourceStarted{Kind}, Found{Finding},
// SourceDone{Kind, Count, Err}, Finished{Duration, Err}

func (s *Scanner) Run(ctx context.Context, t Target, enabled []Source) <-chan Event
```

- One goroutine per enabled source. The candidate analysis runs through a
  bounded worker pool (Settings → Concurrency, default 4). This bounds
  concurrent GitHub requests, which matter more than AWS ones: authenticated
  GitHub REST has a 5,000 req/h budget, and App identification can cost
  N requests per key when searching N Apps.
- `registry.Registry` gains a `sync.Mutex`. It is saved once at `Finished`
  and on cancel, using the prototype's write-via-rename.
- Cancellation: ⌘. cancels the context. In-flight items finish as `Skipped`
  with reason "cancelled" and the registry is still saved.
- Per-item failures never abort a source. A failed listing call ends that
  source with `SourceDone{Err}` and the others continue (the prototype aborted
  the whole run).

## UI model (`internal/app`)

```go
type Model struct {
	Settings  settings.Settings
	Profiles  []awsprofile.Profile
	Current   Target
	Results   map[Target]*ResultSet   // findings by ID + last scan time + per-source state
	Scan      *RunningScan            // nil when idle; cancel func, progress counts
	Banner    *Banner                 // SSO expired, credential errors
	UI        UIState                 // split sizes, column layout, sidebar selection, sort, filter
	Log       *logview.Buffer
}
```

- The view functions read `Model`; commands (`Scan()`, `Stop()`,
  `ExportCSV()`) mutate it.
- A single pump goroutine drains `Scanner.Run`'s channel and applies events
  in batches via `win.Update` at most every 50 ms. Each `Update` draws a frame,
  and per-event updates would draw hundreds of frames per second on a large
  account.
- Derived views (filtered + sorted findings, sidebar counts) are recomputed
  only when an input changes (results version, filter, sort, sidebar
  selection), not every frame. A version counter on `ResultSet` makes this one
  comparison.
- `UIState` persists to `PathUserData/ui-state.json` on change (debounced 1 s):
  split sizes, `ListState.Columns` layouts for grid and console, sort, console
  visibility and level.
- `ui.Element` values are never stored or touched off-frame. `mygo vet`
  enforces this in `just verify`.

## Structured log pipeline (`internal/logview`)

```text
scan/source/github code ──slog.Logger──► logview.Handler ─┬─► Buffer (ring, 20k) ──► console view
                                                          └─► JSONL file sink (PathLogs)
```

- `Handler` implements `slog.Handler` (`Enabled`, `Handle`, `WithAttrs`,
  `WithGroup`). `Handle` copies the record into a compact `Entry{Time, Level,
  Source, Msg, Attrs []Attr, FindingID}` under a mutex, then calls the
  `notify` func the UI provides. That func coalesces into the same 50 ms
  `win.Update` batching as scan events.
- **Redaction is structural, not filtering.** Loggers only ever receive names,
  ARNs, codes, counts and fingerprints. Credential and raw value types have a
  `LogValue()` that returns `"[redacted]"`. A test feeds known secret strings
  through a full fake scan and asserts they appear in neither the buffer, the
  JSONL file, the CSV export, nor any `Finding` field (reflect walk).
- Standard attribute keys: `source`, `finding`, `profile`, `account`,
  `region`, `op` (AWS/GitHub operation), `code` (error code), `status`,
  `duration`.

## Settings and persistence

| What | Where | Format |
| --- | --- | --- |
| Settings | `PathUserData/settings.json` | JSON, versioned (`"version": 1`), write-via-rename |
| UI state | `PathUserData/ui-state.json` | JSON |
| GitHub token | Keychain, `mygo.Secrets` key `github-token` | bytes |
| App cache | `PathCache/github.json` | prototype format. On first run, copy from `$XDG_CACHE_HOME/kms-import/github.json` if present, so learned fingerprints carry over |
| Scan logs | `PathLogs/scan-*.jsonl` | last 10 kept |
| Last results | memory only in v1 | — |

```go
type Settings struct {
	Version       int
	NameMatch     string            // "github"
	DefaultProfile string           // "" = last used
	ScanOnLaunch  bool
	GitHub        GitHubSettings    // Org "cultureamp", APIURL "https://api.github.com"
	Sources       map[SourceKind]SourceSettings // Enabled + options map
	Concurrency   int               // 4
}
```

Token resolution order: Keychain → `GITHUB_TOKEN` / `GH_TOKEN` env (only when
launched from a shell). The "Use `gh auth token`" button runs `gh auth token`
once and stores the result in the Keychain. The token is only needed to seed
Apps from the org; tokens found in AWS authenticate only themselves.

## Security invariants (tested)

1. No secret value, token or PEM byte is stored in `Finding`, `Entry`,
   settings, UI state, CSV or the clipboard. The token-hash cache uses the
   prototype's domain-separated `tokenHash`.
2. Error text shown anywhere is fixed or an AWS error code
   (`awsFailure`). It is never `err.Error()` from x509/asn1/json on secret
   input.
3. GitHub requests stay on the configured API host (`githubAPI.get`).
4. Read-only: the app never writes to AWS or GitHub and never mints
   installation tokens. IAM needs: `secretsmanager:ListSecrets`,
   `secretsmanager:GetSecretValue`, `ssm:DescribeParameters`,
   `ssm:GetParameter`, `kms:Decrypt` (for SecureString / secrets),
   `kms:ListKeys`, `kms:ListAliases`, `kms:DescribeKey`, `kms:GetPublicKey`,
   `kms:Sign`, `sts:GetCallerIdentity`. Settings → Advanced shows this list
   with a Copy button.
5. Raw values are held only for the duration of one candidate's analysis and
   aren't retained by any closure after it.

## Testing strategy

| Layer | How | Bar |
| --- | --- | --- |
| `credential`, `github`, `registry` | Port the prototype's tests verbatim, then adapt | All prototype cases pass |
| `source/*` | Fake AWS clients (the narrow interfaces) | Pagination, filter, metadata mapping, per-item error rows |
| `scan` | Fake sources + `httptest` GitHub | Event ordering, cancel, concurrency bound, registry saved once, redaction walk |
| `awsprofile` | Fixture config/credentials files via `AWS_CONFIG_FILE` | Headers parsed, sso-session skipped, SSO error detected |
| `logview` | Direct `slog` calls | Ring wrap, attrs, finding linkage, file rotation |
| `app` views | `ui.NewTester` with a model seeded from fixtures | Empty states, sidebar filtering, sort, inspector groups per source, keyboard shortcuts; `tt.Image()` snapshots for light and dark |
| Manual | `go tool mygo dev` against a real sandbox account | Once per phase; no real-AWS tests in CI (matches kms-import policy) |
