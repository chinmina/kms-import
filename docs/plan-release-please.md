# Plan: release-please–driven release flow

Status: proposed (scoping). Branch: `claude/release-please-attestation-flow`.

## Goal

Replace the manual "human pushes a `v*` tag" trigger with an automated
[release-please](https://github.com/googleapis/release-please) flow modelled on
`jamestelfer/dollop`, **without losing the build-provenance attestations** that
Phase 9 added (PRD R38/R39). release-please owns version bumps, the changelog,
the tag, and the GitHub Release; GoReleaser builds the artifacts; `actions/attest`
binds provenance to every artifact.

The single non-obvious part — and the reason this is its own plan — is
**attestation ordering**: in the dollop template GoReleaser just publishes, but
kms-import must guarantee no attested binary is downloadable before its
attestation exists. release-please changes *who* creates the release and *when*
the tag appears, which breaks the naive ordering. See
[The attestation ordering problem](#the-attestation-ordering-problem).

## Current state vs. target

| Aspect | Current (Phase 9) | Target |
| --- | --- | --- |
| Trigger | human pushes `v*` tag | merge a release-please "Release PR" to `main` |
| Version source | the tag the human chose | release-please, from Conventional Commits |
| Changelog | `changelog.use: github-native` in GoReleaser | release-please `CHANGELOG.md` + release notes |
| Release creation | GoReleaser (`release --clean`) | release-please creates the GitHub Release + tag |
| Artifacts | GoReleaser builds + uploads | GoReleaser builds; assets uploaded after attestation |
| Attestation | `actions/attest` *after* GoReleaser publishes | `actions/attest` *before* artifacts are uploaded |

No tags or GitHub Releases exist yet (`v*` is unused), so this is a clean
bootstrap — no historical-version migration needed.

## Reference: what dollop gives us, and the gap

dollop (`jamestelfer/dollop`) ships four workflows. We copy three structures and
extend one:

- `ci.yml`, `pr-title.yml` — already mirrored here; no change.
- `release-please.yml` — **new for us.** Push-to-`main` trigger; mints a GitHub
  App token via `actions/create-github-app-token`, then runs
  `googleapis/release-please-action`. The App token is not cosmetic — see
  [Why a GitHub App token](#why-a-github-app-token-is-required).
- `release.yml` — dollop's is tag-triggered and runs `goreleaser release --clean`
  and **stops there. It has no attestation step and no `attestations` /
  `artifact-metadata` permissions.** This is the gap: kms-import's `release.yml`
  already does the attestation, and we must keep it while slotting into the
  release-please flow.

dollop also adds `release-please-config.json` (`release-type: simple`, root
package) and `.release-please-manifest.json` (current version). We need both,
tuned for a Go binary whose version is stamped via ldflags (no in-source version
file to bump).

## The attestation ordering problem

The supply-chain invariant we must preserve: **build → attest → publish.** A
binary must not be downloadable from the Release until its provenance
attestation has been recorded. Two release-please behaviours make this harder
than the current tag flow:

1. **release-please creates the GitHub Release, and it is published immediately
   on PR merge** — before any binary is built. If GoReleaser then uploads assets
   and we attest afterward (today's order: `release --clean` → `attest`), there
   is a window where attested-by-policy binaries are publicly downloadable with
   no attestation on file. That violates R38's intent.

2. **A *draft* release does not create a real git tag.** The obvious fix —
   "make release-please create a draft, build, attest, then publish" — fails in a
   two-workflow split: GitHub stores a draft release's tag only inside the
   release object, so no `refs/tags/v*` exists, so a `on: push: tags` workflow
   never fires and GoReleaser has no tag to derive the version from. Draft-first
   only works if the build job is chained in the *same* run (see Option B).

### Resolution

Let release-please create a **published** release + tag (so the tag exists), but
change GoReleaser so it **does not upload**, attest, then upload as the final
step:

```
goreleaser release --clean --skip=publish   # build archives + dist/checksums.txt, NO upload
actions/attest  subject-checksums=dist/checksums.txt   # provenance recorded first
gh release upload "$TAG" dist/*.tar.gz dist/*.zip dist/checksums.txt --clobber  # publish assets last
```

`--skip=publish` makes GoReleaser produce `dist/` (archives + `checksums.txt`)
without touching the GitHub Release, so it no longer fights release-please over
the release body. Attestation runs over the checksum file (attesting every
artifact by digest, order-independent of the release). Only then are the binaries
attached. The Release page may briefly show source-zips only — acceptable; what
matters is that no *attested* binary is downloadable before its attestation
exists.

## Design decisions

1. **Mirror dollop's two-workflow shape** (`release-please.yml` push-to-main +
   `release.yml` tag-triggered). Faithful to the example and keeps the release
   build isolated. (Option B below is the single-workflow alternative.)
2. **release-please creates a published release + tag.** Draft-first is rejected
   because of the draft/tag gotcha above.
3. **GoReleaser builds but does not publish** (`--skip=publish`); assets uploaded
   via `gh release upload` after attestation.
4. **release-please owns the changelog;** drop `changelog.use: github-native`
   from `.goreleaser.yaml` to avoid two changelog sources (with `--skip=publish`
   GoReleaser won't post notes anyway, but remove it to be unambiguous).
5. **`release-type: simple`,** root package, no `extra-files`. Version is stamped
   from the tag via existing ldflags; there is no in-source version constant to
   bump, so no extra files to manage (unlike dollop's `flake.nix`).
6. **App token for release-please** (`actions/create-github-app-token`), required
   so the release tag triggers `release.yml`.

## File-by-file changes

### New: `.github/workflows/release-please.yml`
Push-to-`main`. Job mints an App token (client-id/private-key from secrets) and
runs `googleapis/release-please-action` (pinned by SHA) with that token. Pin the
action by commit SHA per repo convention. Optionally expose `release_created` /
`tag_name` outputs for observability. `environment: automation` (as dollop) if a
protected environment guards the secrets.

### Change: `.github/workflows/release.yml`
Keep the `on: push: tags: ['v*']` trigger and the existing checkout/setup/mise/
goreleaser scaffold. Three edits:
- GoReleaser args → `release --clean --skip=publish`.
- Keep the existing `actions/attest` step (`subject-checksums: dist/checksums.txt`)
  **after** GoReleaser.
- Add a final step: `gh release upload "${GITHUB_REF_NAME}" dist/*.tar.gz
  dist/*.zip dist/checksums.txt --clobber` (`GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`).
- Permissions stay `contents: write`, `id-token: write`, `attestations: write`,
  `artifact-metadata: write` (already present — this is exactly what dollop lacks).

### New: `release-please-config.json`
```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "release-type": "simple",
  "packages": { ".": {} }
}
```
Consider `"initial-version": "0.1.0"` (or `"bootstrap-sha"`) so the first release
starts pre-1.0 rather than release-please's default `1.0.0`.

### New: `.release-please-manifest.json`
```json
{ ".": "0.0.0" }
```
Seed at `0.0.0` (or chosen initial version) since no prior release exists.

### Change: `.goreleaser.yaml`
- Remove the `changelog:` block (release-please owns the changelog).
- Leave `release.prerelease: auto`, `checksum.name_template: checksums.txt`
  (the attest step + upload step both depend on `dist/checksums.txt`), builds,
  and archives unchanged.

### Change: `README` / `docs`
- Update the release/verification docs: releases are cut by merging the Release
  PR, not by pushing a tag. `gh attestation verify` steps are unchanged (the
  attestation still covers each artifact by digest).
- Update `docs/progress-kms-import.md` once implemented.

## Prerequisites (repo settings — outside the code change)

- **GitHub App** with `contents: write` + `pull-requests: write`, installed on
  the repo; its Client ID and a private key stored as secrets
  (`RELEASE_PLEASE_CLIENT_ID`, `RELEASE_PLEASE_APP_PRIVATE_KEY`), matching the
  names dollop uses. (Owner action; document it.)
- If using `environment: automation`/`release`, create those environments and
  scope the secrets to them.
- Branch protection on `main` must allow the App to merge/commit as needed.

## Why a GitHub App token is required

A tag pushed using the default `GITHUB_TOKEN` does **not** trigger other
workflows (GitHub's recursion guard). In the two-workflow design the release tag
created by release-please must trigger `release.yml`, so release-please has to
push it with a non-`GITHUB_TOKEN` identity — hence the App token. This is the
mechanism behind dollop's `create-github-app-token` step, and skipping it is the
most common reason "the release workflow never ran."

## End-to-end sequence

1. Conventional-commit PRs merge to `main`; `ci.yml` runs as today.
2. `release-please.yml` opens/updates a **Release PR** (version bump + changelog).
3. Maintainer merges the Release PR.
4. release-please (App-token identity) creates the **published GitHub Release**
   and pushes the **`vX.Y.Z` tag**.
5. The tag triggers `release.yml`: GoReleaser builds all five targets +
   `checksums.txt` with `--skip=publish` (no upload).
6. `actions/attest` records build-provenance for every artifact by digest.
7. `gh release upload` attaches the binaries + checksums to the Release —
   binaries become downloadable only now, after attestation. ✅

## Risks & gotchas

- **Draft/tag gotcha** (covered above) — do not switch release-please to draft in
  the two-workflow design.
- **App-token trigger** — without it, step 5 never fires.
- **Glob expansion in `gh release upload`** — ensure the runner shell expands
  `dist/*.tar.gz`/`*.zip`; otherwise enumerate from `dist/artifacts.json` or use
  `goreleaser`'s `--skip=publish` plus a known file list.
- **`--skip` syntax** — GoReleaser v2 uses `--skip=publish` (comma-separated for
  multiple); verify against the pinned GoReleaser version in `mise.toml`.
- **Two changelog sources** — must remove `changelog:` from `.goreleaser.yaml`.
- **First release version** — set `initial-version`/manifest deliberately to
  avoid an unintended `1.0.0`.

## Alternative — Option B: single chained workflow

Fold both jobs into one push-to-`main` workflow: a `release-please` job whose
`release_created`/`tag_name` outputs gate a `goreleaser` job
(`needs:` + `if: needs.release-please.outputs.release_created == 'true'`,
checkout `ref: <tag_name>`). Advantages: no cross-workflow trigger, so the App
token becomes optional; and it *could* support a true draft→attest→publish flow
since the build runs in the same run. Disadvantage: diverges from dollop's
structure and couples release-please with the heavy build job. Recommend Option A
(two workflows) for fidelity to the example unless the team prefers B.

## Out of scope

- Homebrew tap / cask (dollop has one; kms-import does not — omit
  `HOMEBREW_GITHUB_TOKEN` and the brew block).
- Container images, package registries, or signing beyond the existing keyless
  build-provenance attestation.
- Integration tests against real AWS (explicitly excluded repo-wide).

## Verification

- Dry run on a throwaway branch/fork: merge a `feat:` commit, confirm a Release
  PR appears; merge it; confirm tag + published Release; confirm `release.yml`
  runs, attestation appears in the repo Attestations tab, and assets attach only
  after the attest step.
- `gh attestation verify <artifact> --repo chinmina/kms-import` passes following
  the README steps (R39).
- `just verify` stays green; `ci.yml` snapshot build unaffected.
