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

1. **By default release-please publishes the GitHub Release immediately on PR
   merge** — before any binary is built. If GoReleaser then uploads assets and we
   attest afterward (today's order: `release --clean` → `attest`), there is a
   window where attested-by-policy binaries are publicly downloadable with no
   attestation on file. That violates R38's intent.

2. **A plain *draft* release does not create a real git tag.** GitHub stores a
   draft release's tag only inside the release object, so no `refs/tags/v*`
   exists, a `on: push: tags` workflow never fires, and GoReleaser has no tag to
   derive the version from. This is the trap that makes "just use a draft" look
   unworkable in a two-workflow split — but release-please has a purpose-built
   escape hatch (below).

### Resolution — defer publishing with a draft release + forced tag

release-please should **create the Release as a draft and not publish it**,
deferring publication to the end of the build. The schema gives us exactly the
two knobs needed (per-package in `release-please-config.json`):

- `"draft": true` — create the GitHub Release in draft mode (not public; draft
  assets are visible only to write-access users).
- `"force-tag-creation": true` — *"Force the creation of a Git tag for the
  release... particularly useful when `draft` is enabled, because GitHub does
  not create a Git tag for draft releases until they are published."* This
  defeats gotcha #2: the `v*` tag is pushed even though the Release stays draft,
  so `release.yml` still fires.

> Note: `"skip-github-release": true` is **not** the right tool here. Its schema
> warning — *"Release-Please still requires releases to be tagged, so this option
> should only be used if you have existing infrastructure to tag these releases"*
> — means it suppresses the tag too, breaking the trigger. Draft + force-tag is
> the supported way to "create the tag now, publish the release later."

GoReleaser then targets that existing draft and the final step publishes it:

```
goreleaser release --clean   # release.draft: true + use_existing_draft: true + mode: keep-existing
actions/attest subject-checksums=dist/checksums.txt   # provenance recorded first
gh release edit "$TAG" --draft=false                  # publish last, after attestation
```

Because the Release (page *and* assets) stays a draft until the final `gh
release edit`, **nothing is publicly downloadable until after the attestation is
recorded** — a strictly cleaner build → attest → publish than uploading assets to
an already-public release. `use_existing_draft` (GoReleaser v2.5+) makes
GoReleaser fill the draft release-please created rather than make its own, and
`mode: keep-existing` preserves release-please's changelog body.

## Design decisions

1. **Mirror dollop's two-workflow shape** (`release-please.yml` push-to-main +
   `release.yml` tag-triggered). Faithful to the example and keeps the release
   build isolated. (Option B below is the single-workflow alternative.)
2. **release-please creates a *draft* release + a forced tag** (`draft: true` +
   `force-tag-creation: true`); publication is deferred. The draft/tag gotcha is
   handled by `force-tag-creation`, not avoided.
3. **GoReleaser fills the existing draft** (`release.draft: true`,
   `use_existing_draft: true`, `mode: keep-existing`); a final
   `gh release edit --draft=false` publishes only after attestation.
4. **release-please owns the changelog;** drop `changelog.use: github-native`
   from `.goreleaser.yaml`. `mode: keep-existing` ensures GoReleaser does not
   overwrite release-please's release notes.
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
`tag_name` outputs for observability. Runs in the `automation` environment
(as dollop), which holds/guards the App secrets.

### Change: `.github/workflows/release.yml`
Keep the `on: push: tags: ['v*']` trigger and the existing checkout/setup/mise/
goreleaser scaffold. Edits:
- GoReleaser args stay `release --clean` (it now targets the existing draft).
- Keep the existing `actions/attest` step (`subject-checksums: dist/checksums.txt`)
  **after** GoReleaser.
- Add a final publish step: `gh release edit "${GITHUB_REF_NAME}" --draft=false`
  (`GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`).
- Permissions stay `contents: write`, `id-token: write`, `attestations: write`,
  `artifact-metadata: write` (already present — this is exactly what dollop lacks).

### New: `release-please-config.json`
```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "release-type": "simple",
  "packages": {
    ".": {
      "draft": true,
      "force-tag-creation": true
    }
  }
}
```
`draft` + `force-tag-creation` are the crux: the Release is created unpublished
but the `v*` tag is still pushed (so `release.yml` fires). Consider
`"initial-version": "0.1.0"` (or `"bootstrap-sha"`) so the first release starts
pre-1.0 rather than release-please's default `1.0.0`.

### New: `.release-please-manifest.json`
```json
{ ".": "0.0.0" }
```
Seed at `0.0.0` (or chosen initial version) since no prior release exists.

### Change: `.goreleaser.yaml`
- Remove the `changelog:` block (release-please owns the changelog).
- Set `release.draft: true`, `release.use_existing_draft: true`, and
  `release.mode: keep-existing` so GoReleaser fills the draft release-please
  created without overwriting its notes or publishing it.
- Leave `release.prerelease: auto`, `checksum.name_template: checksums.txt`
  (the attest step depends on `dist/checksums.txt`), builds, and archives
  unchanged.

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
- **Required environments** (the jobs declare `environment:`, so they will not
  start until these exist): `automation` for `release-please.yml` and `release`
  for `release.yml`. Scope the App secrets to `automation`; `release` is the gate
  for the signing/publish job (add required reviewers / branch filters here if
  desired). The App secrets must be reachable from the `automation` environment.
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
4. release-please (App-token identity) creates a **draft GitHub Release** and
   pushes the **`vX.Y.Z` tag** (`force-tag-creation`). Nothing is public yet.
5. The tag triggers `release.yml`: GoReleaser builds all five targets +
   `checksums.txt` and fills the **existing draft** (`use_existing_draft`), still
   unpublished.
6. `actions/attest` records build-provenance for every artifact by digest.
7. `gh release edit "$TAG" --draft=false` publishes the Release — binaries become
   publicly downloadable only now, after attestation. ✅

## Risks & gotchas

- **Draft/tag gotcha** — a plain draft creates no git tag; `force-tag-creation`
  is what makes the tag (and therefore the trigger) appear.
- **`use_existing_draft` matching** — GoReleaser matches the draft by tag name;
  requires GoReleaser ≥ v2.5 (verify the pinned version in `mise.toml`).
- **App-token trigger** — without it, the tag push won't fire `release.yml`
  (step 5 never runs).
- **Publish step is the gate** — if `gh release edit --draft=false` is skipped or
  fails, the release stays an invisible draft. It must run only after attest.
- **Two changelog sources** — must remove `changelog:` from `.goreleaser.yaml`;
  `mode: keep-existing` protects release-please's notes.
- **First release version** — set `initial-version`/manifest deliberately to
  avoid an unintended `1.0.0`.

## Alternative — Option B: single chained workflow

Fold both jobs into one push-to-`main` workflow: a `release-please` job whose
`release_created`/`tag_name` outputs gate a `goreleaser` job
(`needs:` + `if: needs.release-please.outputs.release_created == 'true'`,
checkout `ref: <tag_name>`). Advantage: no cross-workflow trigger, so the App
token becomes optional. Disadvantages: diverges from dollop's structure and
couples release-please with the heavy build job. With the draft + force-tag
design above, Option A already achieves a clean draft→attest→publish, so the main
reason to prefer B is dropping the App token. Recommend Option A (two workflows)
for fidelity to the example unless the team would rather avoid the App.

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
