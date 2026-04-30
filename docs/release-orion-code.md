# release-orion-code.sh - Design and Usage

## Overview

`release-orion-code.sh` is the long-term release helper for Orion. It replaces
the older local build-and-publish flow with a safer design:

1. A local developer prepares a version-bump PR.
2. The PR is reviewed and merged into `master`.
3. GitHub Actions builds and publishes the release from the merged commit.

The important design choice is that release artifacts are produced in CI from a
clean checkout, not from a developer workstation. The merged `VERSION` file is
the release request, the Git tag is the immutable release anchor, and the GitHub
release asset is built from that exact commit.

`build-orion-code.sh` is intentionally left in place for the older workflow.
New releases should use `release-orion-code.sh` and
`.github/workflows/release.yml`.

## Files

| File | Purpose |
|------|---------|
| `release-orion-code.sh` | Local helper for preparing release PRs and building release archives. |
| `.github/workflows/release.yml` | CI workflow that validates `VERSION`, creates the release tag, builds the archive, and publishes the GitHub release. |
| `VERSION` | Source-controlled release version in bare `X.Y.Z` format. |
| `dist/orion-vX.Y.Z.tar.gz` | Release archive produced by the build command or workflow. |

## Design goals

The new release design is meant to address weaknesses in the older script:

- Keep build and publish separate from version preparation.
- Require human review before a release is published.
- Build in a clean, repeatable Linux environment.
- Avoid publishing archives that contain local uncommitted files.
- Anchor every release to an annotated Git tag.
- Make the release archive more deterministic.
- Keep GitHub authentication out of plain local archive builds.

## Version tracking

The repository root contains a `VERSION` file holding the current release
version as a bare `X.Y.Z` string:

```text
1.0.6
```

The tag and archive names derive from this value:

| VERSION | Tag | Archive |
|---------|-----|---------|
| `1.3.0` | `v1.3.0` | `orion-v1.3.0.tar.gz` |

The `v` prefix is used only for Git tags and release names. It is not stored in
`VERSION`.

## Commands

```bash
./release-orion-code.sh prepare <version>
./release-orion-code.sh build [version]
```

| Command | Description |
|---------|-------------|
| `prepare <version>` | Creates `release/vX.Y.Z`, writes `VERSION`, commits it, pushes the branch, and opens a PR against `master`. |
| `build [version]` | Builds `dist/orion-vX.Y.Z.tar.gz` from the current checkout. If omitted, `version` is read from `VERSION`. |

## Prerequisites

| Tool | Purpose | Required for |
|------|---------|--------------|
| `git` | Branch, tag, and repository state checks | Always |
| `make` | Builds all Go binaries via the root `Makefile` | `build`, CI release |
| GNU `tar` | Creates deterministic archives | `build`, CI release |
| `gh` CLI | Opens release PRs locally and creates GitHub releases in CI | `prepare`, CI release |

Local macOS builds require GNU tar as `gtar`, usually installed with:

```bash
brew install gnu-tar
```

`gh` must be authenticated before running `prepare`:

```bash
gh auth login
```

Plain archive builds do not require GitHub authentication.

## Recommended release workflow

### Step 1 - Prepare the release PR

Run from a clean working tree:

```bash
./release-orion-code.sh prepare 1.3.0
```

This will:

1. Validate that `1.3.0` matches `X.Y.Z`.
2. Require `gh` and verify `gh auth status`.
3. Require a clean working tree.
4. Fetch and fast-forward local `master` from `origin/master`.
5. Check that `release/v1.3.0` does not already exist locally or remotely.
6. Check that tag `v1.3.0` does not already exist on `origin`.
7. Create branch `release/v1.3.0`.
8. Write `1.3.0` to `VERSION`.
9. Commit `VERSION` as `chore: release v1.3.0`.
10. Push the branch.
11. Open a GitHub PR titled `Release v1.3.0` against `master`.

No archive is built and no release is published in this step.

### Step 2 - Review and merge

Review the generated PR in the normal way. The release decision happens when
the PR is merged into `master`.

The release branch should contain only the `VERSION` bump unless the team
explicitly decides to include release-note or changelog updates in the same PR.

### Step 3 - GitHub Actions publishes the release

When the `VERSION` change lands on `master`, `.github/workflows/release.yml`
runs automatically.

The workflow will:

1. Check out the repository with full Git history.
2. Read the merged `VERSION`.
3. Validate `X.Y.Z` format.
4. Confirm that a manually supplied workflow version, if any, matches
   `VERSION`.
5. Check that `vX.Y.Z` does not already exist on `origin`.
6. Set up Go.
7. Run `./release-orion-code.sh build X.Y.Z`.
8. Verify that `dist/orion-vX.Y.Z.tar.gz` exists and is readable.
9. Create annotated tag `vX.Y.Z` on the merged commit.
10. Push the tag.
11. Create the GitHub release.
12. Upload `dist/orion-vX.Y.Z.tar.gz`.
13. Generate release notes from GitHub history.

## Manual release workflow dispatch

The release workflow also supports `workflow_dispatch`.

Manual dispatch is useful if:

- The automatic workflow was disabled.
- The release workflow file changed and needs a deliberate run.
- A release needs to be retried after fixing CI infrastructure.

If a version is supplied in the workflow input, it must match the repository
`VERSION` file at the selected commit. This prevents accidentally publishing
`v1.3.0` from a commit whose source says `1.2.9`.

## Archive layout

The release archive is named `orion-vX.Y.Z.tar.gz` and written to `dist/`.

It unpacks to:

```text
opt/
  bin/
    apis
    dbtool
    updater
    objupdater
    provisioner
    *.sh from utils/
    *.sh from db/
  db/
    *.sql from db/
  docker/
    docker-compose.yml
```

The archive intentionally copies named Go binaries instead of every file in the
Go binary directory. Shell scripts and SQL files are copied in sorted order.

## Deterministic archive behavior

The build command uses GNU tar with:

```text
--sort=name
--owner=0
--group=0
--numeric-owner
--mtime=@<source_date_epoch>
```

By default, `<source_date_epoch>` is the timestamp of the current Git commit.
It can be overridden with `SOURCE_DATE_EPOCH`.

This reduces differences between archives built from the same commit. It does
not make the build fully reproducible if the compiled Go binaries themselves
include non-deterministic data.

## Build only

To build the current version locally:

```bash
./release-orion-code.sh build
```

To build a specific version from the current checkout:

```bash
./release-orion-code.sh build 1.3.0
```

The build command:

1. Reads or validates the version.
2. Creates a temporary package root using `mktemp -d`.
3. Runs `make all`.
4. Copies the expected binaries from `${GOBIN:-$HOME/go/bin}`.
5. Copies release shell scripts and SQL files.
6. Writes the archive to `${DIST_DIR:-<repo>/dist}`.
7. Cleans up the temporary package root.

Useful environment overrides:

| Variable | Description |
|----------|-------------|
| `DIST_DIR` | Output directory for the archive. Defaults to `<repo>/dist`. |
| `DEPLOY_ROOT` | Temporary package root. Defaults to a `mktemp` directory. |
| `GOBIN` | Directory containing built Go binaries. Defaults to `$HOME/go/bin`. |
| `SOURCE_DATE_EPOCH` | Timestamp used for archive file mtimes. Defaults to current Git commit time. |

## Why build and publish are separate

Release preparation and release publication have different risk profiles.

The local `prepare` command changes source control state by creating a PR. It
needs GitHub authentication, but it should not build or publish anything.

The CI release workflow publishes an artifact. It should run only after review,
from a clean checkout, and from the exact merged commit. This prevents a release
asset from including local files, stale binaries, or unreviewed changes.

## Why `gh auth` is required only for prepare

`prepare` uses `gh pr create`, so local GitHub authentication is required.

`build` is intentionally offline with respect to GitHub. A developer should be
able to build and inspect an archive without any GitHub credentials.

In GitHub Actions, the workflow uses the built-in `GITHUB_TOKEN` through
`GH_TOKEN` to create the release.

## Error handling

The script runs with `set -euo pipefail` and aborts on the first failure.

Common failures:

| Symptom | Likely cause |
|---------|--------------|
| `version must be in X.Y.Z format` | The supplied version is missing or malformed. |
| `working tree is not clean` | Commit or stash local changes before `prepare`. |
| `gh is not authenticated` | Run `gh auth login` before `prepare`. |
| `local branch 'release/vX.Y.Z' already exists` | A previous prepare run created the branch. |
| `remote branch 'release/vX.Y.Z' already exists` | A release PR already exists or was not cleaned up. |
| `tag 'vX.Y.Z' already exists on origin` | The version was already released or partially released. |
| `GNU tar is required` | Install GNU tar locally, usually as `gtar` on macOS. |
| `required release file is missing` | `make all` did not produce an expected binary or a required package file is absent. |

## Recovery notes

If `prepare` fails before pushing the branch, inspect the local branch and either
continue manually or delete it:

```bash
git branch -D release/v1.3.0
```

If the GitHub workflow creates the tag but fails while creating the GitHub
release, do not rerun blindly. Either:

1. Create/upload the release asset manually for the existing tag, or
2. Delete the failed remote tag and rerun only if the team agrees the tag was
   never consumed.

Tags are release identity. Treat tag deletion as an exceptional operation.

## Comparison with build-orion-code.sh

| Area | Older `build-orion-code.sh` | New `release-orion-code.sh` plus workflow |
|------|-----------------------------|-------------------------------------------|
| Version bump | Local script can update `VERSION` and optionally open PR | Local script only prepares a reviewed PR |
| Build location | Developer machine | GitHub Actions runner |
| Publish location | Developer machine | GitHub Actions runner |
| GitHub auth | Required for PR and publish flags | Required locally only for PR creation |
| Release source | Current local checkout | Merged commit on `master` |
| Tag creation | Local publish path | CI after validation |
| Archive output | Repository root | `dist/` |
| Temporary directory | Fixed `/tmp/deploy` | Per-run `mktemp` directory |
| Archive contents | Broad copy from `$HOME/go/bin`, `utils`, and `db` | Named binaries plus sorted script/schema copies |

