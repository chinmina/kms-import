# Design

## Main window

One window (`StateKey: "main"`, default 1280×800, minimum 900×560), hidden
title bar with the toolbar drawn into it, vibrant sidebar. Three columns over a
collapsible console:

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ ● ● ●   Fingerprints    [ ops-prod ▾ ] [ ap-southeast-2 ▾ ]  ⟳ Scan    🔍 Filter      ▣ ▤│
│                          123456789012 · SSO · scanned 2 min ago                      │
├───────────────┬──────────────────────────────────────────────┬───────────────────────┤
│ SOURCES       │ Name              Source  Type      App/User │ github-app-chinmina   │
│  ◎ All     42 │ github-app-chin…  SM      App key   chinmina │ Secrets Manager       │
│  ▢ Secrets 18 │ /ci/github-token  SSM     Fine PAT  ci-bot   │ ● Valid               │
│  ▢ SSM     16 │ alias/chinmina    KMS     KMS key   chinmina │                       │
│  ▢ KMS      8 │ legacy-token      SM      Classic   —        │ CREDENTIAL            │
│               │ …                                            │ Type   App private key│
│ STATUS        │                                              │ Finger SHA256:4bX…  ⧉ │
│  ● Valid   30 │                                              │ App    chinmina (1234)│
│  ● Invalid  5 │                                              │ Matched via app cache │
│  ● Unknown  4 │                                              │                       │
│  ● Errors   3 │                                              │ INSTALLATIONS         │
│               │                                              │ cultureamp  all  …    │
│               │                                              │                       │
│               │                                              │ SECRETS MANAGER       │
│               │                                              │ Last changed 3 Mar    │
│               │                                              │ Rotation    Off       │
├───────────────┴──────────────────────────────────────────────┴───────────────────────┤
│ CONSOLE   [All levels ▾]  🔍                                       Copy  Clear  ⌄    │
│ 10:42:01.120  INFO   ssm   listed parameters   page=3 matched=4                      │
│ 10:42:01.410  WARN   kms   key not RS256       key=alias/old-signer                  │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

Split structure (each size a `*float32` in persisted UI state):

```text
SplitVertical(console height)
├── Split(sidebar width 200–260)
│   ├── Sidebar
│   └── Split(inspector width 300–420)
│       ├── Findings grid
│       └── Inspector        (toggle ⌥⌘I; hidden → grid fills)
└── Console                  (toggle ⇧⌘Y; collapsed → 0, remembers last height)
```

## Toolbar

Left to right, inside `c.TitleBar()` insets, the whole row `.DragWindow()`:

| Control | Widget | Behaviour |
| --- | --- | --- |
| Profile | `ui.Combobox` (type to filter; profiles are often 30+) | Changing it shows that profile's cached results; does **not** auto-scan |
| Region | `ui.Select` | Defaults to the profile's region; "Profile default" first |
| Identity line | secondary text under the pickers | `account · auth kind (SSO / role / keys / process) · scanned <relative time>` |
| Scan / Stop | `ui.PrimaryButton` toggling to a Stop button with a spinner | ⌘R / ⌘. |
| Filter | `ui.SearchField` | ⌘F; matches name, app, user, fingerprint prefix |
| Inspector, Console toggles | icon toggles | mirror the View menu items |

When the profile's SSO session has expired, a banner sits under the toolbar:
"The SSO session for ops-prod has expired. [Sign In] [Dismiss]". Sign In runs
`aws sso login --profile ops-prod`, streams its output into the console, and
retries the identity check when it exits 0.

## Sidebar

`ui.Sidebar` over the window's `Vibrancy`. Two sections. Selecting an item
filters the grid, and the two sections combine (Source AND Status).

- **Sources**: All, then one item per enabled source with a count badge. The
  list comes from the source registry, so a new source appears without UI
  changes.
- **Status**: Valid, Invalid, Unknown App, Errors, with counts. Status colours
  appear only as an 8 px dot.

## Findings grid

`ui.Table` with multi-column sort (`SortBy`), column reorder and resize
persisted through `ListState.Columns`, and row `Key` = finding ID.

| ID | Title | Default | Content |
| --- | --- | --- | --- |
| `name` | Name | shown, 260, `RowHeader` | secret / parameter name, or KMS aliases |
| `source` | Source | shown, 110 | "Secrets Manager", "Parameter Store", "KMS" |
| `type` | Type | shown, 130 | App key, KMS key, Fine-grained PAT, Classic PAT, OAuth, User-to-server, Installation, Refresh, Legacy |
| `identity` | App / User | shown, 160 | App slug, else user login |
| `status` | Status | shown, 110 | dot + label |
| `expires` | Expires | shown, 110, right-aligned, `tnum` | relative ("in 12 days"); red when < 7 days |
| `access` | Access | hidden | repo count, `All` badge for unrestricted |
| `fingerprint` | Fingerprint | hidden, monospace | `SHA256:…` truncated in the middle |
| `region` | Region | hidden | |

- Rows stream in as the scan finds them. Sorting applies on every frame, so
  an incoming row lands in place, not at the bottom. Rows are never resorted
  under the pointer while a drag-select is in progress.
- Empty states (centred, secondary text, one action): no profile chosen →
  "Choose an AWS profile to scan"; not scanned → "[Scan ops-prod]"; scanned,
  nothing found → "No GitHub credentials found in 3 sources"; filtered to
  nothing → "No matches for 'xyz' [Clear Filter]".
- Context menu: Copy Name, Copy ARN, Copy Fingerprint, Open in AWS Console,
  Open GitHub App (when matched), Rescan This Item.
- ⌘C copies the selected rows as CSV with a header.

## Inspector (details view)

A scrolling column of titled groups using `ui.Form`-style label/value rows.
Labels are right-aligned secondary text; values are selectable. Copyable
values get a hover ⧉ button. Groups appear only when they have content.

**Header (all findings):** name (title 2, wraps), source + type line, status
pill, "Open in AWS Console" link. Status detail text sits under the pill when
it isn't plain Valid, e.g. "read failed: AccessDeniedException",
"invalid (cached)", "not RS256", "key PendingDeletion".

**Credential group, tailored by credential type:**

| Credential | Rows |
| --- | --- |
| App private key / KMS key | Fingerprint (mono, copy) · App slug + ID (link to `github.com/apps/<slug>`) · Owner · Matched via (app cache / stored App ID / searched N apps) · JSON field (when nested) |
| ↳ Installations (table) | Account · Repositories (All / Selected) · Permissions (wrapped `name:level` chips) |
| Personal / OAuth token | User · Scopes (chips; classic only) · Expires (absolute + relative) · Repositories (count, or "All N") |
| Installation token | Repositories (count, All/Selected) |
| Refresh token | "Not probed: refresh tokens can't be used without the App's client secret." |

**Source group, tailored by source** (from metadata already listed, no extra calls):

| Source | Rows |
| --- | --- |
| Secrets Manager | ARN · Description · KMS key (`aws/secretsmanager` when unset) · Created · Last changed · Last accessed (date only, as AWS reports) · Rotation (On/Off, last/next, Lambda) · Owning service · Primary region (replicas) · Tags |
| Parameter Store | ARN · Type (SecureString/String) · Tier · Version · Data type · KMS key · Last modified + by whom · Description · Policies (expiration etc.) |
| KMS | Key ID / ARN · Aliases · State · Origin (**EXTERNAL** shows "Imported key material", which links this to kms-import) · Key spec · Signing algorithms · Created · Key material expires (`ValidTo`, red when < 30 days) · Multi-Region · Description |

New sources implement a detail provider (see
[architecture.md](architecture.md#sources)). The inspector renders the
provider's groups generically, so adding a source needs no inspector code.

## Console (live structured log)

- Virtualized `ui.Table` (or `ui.List` with aligned cells; Phase 0 decides)
  with `FollowEnd`. It sticks to the newest line until the user scrolls up;
  a "Jump to Latest" pill appears while it isn't `AtEnd()`.
- Columns: time (`HH:MM:SS.mmm`, mono, tnum) · level (DEBUG grey, INFO
  default, WARN orange, ERROR red — text colour only) · source · message ·
  attributes (`key=value` in secondary colour, mono).
- Controls: level filter (`ui.Select`: Debug / Info / Warnings / Errors),
  search field, Copy (selected lines), Clear, collapse chevron.
- Selecting a log line that carries a `finding` attribute selects that row in
  the grid.
- Bounded at 20,000 records (ring buffer); the full log of each scan is also
  written as JSON lines to `PathLogs/scan-<timestamp>.jsonl`, with the last
  10 kept.

## Settings window

⌘, from the app menu "Settings…" opens a second window (`StateKey:
"settings"`, 560×420, not resizable, single instance). `ui.Tabs` across the
top: General · Sources · GitHub · Advanced. Changes apply immediately, with no
Save button, as in macOS System Settings.

| Tab | Fields |
| --- | --- |
| General | Name filter (`github`; "Secrets and parameters whose name contains this text") · Default profile (Combobox, or "Last used") · Scan on launch (Switch, off) |
| Sources | One Fieldset per registered source: enabled Switch + that source's own options (Secrets Manager, SSM: override name filter; KMS: "Only keys with imported material" Switch) |
| GitHub | Organization (`cultureamp`) · API URL (`https://api.github.com`; GHES) · Token: status line ("Stored in Keychain" / "Not set") + [Use `gh auth token`] [Paste…] [Remove] · Known Apps: count, "Seeded from cultureamp 3 days ago", [Refresh Apps] |
| Advanced | Cache: path, size, [Reveal in Finder] [Clear App Cache…] (confirm dialog) · Logs: [Reveal in Finder] · Concurrency (Stepper 1–8, default 4) |

## Menus

Native `NSMenu` via `mygo.App.SetMenu`:

- **App**: About, Settings… ⌘,, Quit (roles).
- **File**: Scan ⌘R, Stop ⌘., Export CSV… ⇧⌘E (save sheet), Close ⌘W.
- **Edit**: role menu (Copy works in grid, inspector and console).
- **View**: Show Inspector ⌥⌘I, Show Console ⇧⌘Y, Show Sidebar ⌃⌘S, Filter ⌘F.
- **Window**, **Help** (opens the README in a browser).

## Mac design rules

MyGo paints its widgets, so fidelity comes from restraint. These rules are
acceptance criteria, not suggestions:

1. **System everything.** Font is the system font (`c.Theme()` defaults),
   13 pt body, 11 pt secondary, the accent from System Settings. Light/dark
   follow the system; no in-app theme switch in v1. Never hard-code a colour:
   derive from the theme, except four semantic status colours mapped to the
   theme's red/orange/green/grey.
2. **Open, quiet layout.** No borders around panes. The split dividers are
   the only lines. Group spacing 20 pt, row spacing 6 pt, 16 pt content
   insets. Section titles are 11 pt semibold uppercase secondary (as Finder's
   sidebar). Use colour for state, never for decoration.
3. **Vibrancy where AppKit has it:** the sidebar and toolbar only. Grid,
   inspector and console use the window background.
4. **Monospace only for machine values:** fingerprints, ARNs, key IDs, log
   lines. Tabular figures (`tnum`) for counts, times and dates.
5. **Icons:** Lucide SVG at 16 px, stroke 1.5, tinted with the text colour.
   Toolbar buttons are icon-only with tooltips, except Scan.
6. **Keyboard first:** every command in a menu with its shortcut; Tab moves
   between sidebar → grid → inspector → console; arrow keys in the grid move
   the selection and update the inspector immediately.
7. **No modal interruptions** during a scan. Errors on items become row
   status; scan-level errors (credentials, network) become the banner.
8. **Motion:** only MyGo's built-in transitions. Honour reduce motion (the
   theme exposes it).
9. **Accessibility:** every icon button gets a `Label`. Status is never
   conveyed by colour alone (dot + text). Check VoiceOver once per phase
   (`docs/ui/accessibility.md`).

Reference apps for look and behaviour: Finder (sidebar, toolbar), Console.app
(log pane, streaming, filter), Keychain Access (grid + inspector of
credentials), Xcode (inspector toggles and console area shortcuts).
