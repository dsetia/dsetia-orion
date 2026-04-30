# Orion Code Archive Release Design

## Overview

Orion code archives are released through a reviewed version-bump PR and a
GitHub Actions release workflow. A developer prepares the release request
locally, reviewers approve the version change, and CI builds and publishes the
archive from the merged commit.

The release source of truth is:

1. `VERSION` records the requested release version.
2. The merged commit on `master` records the reviewed release request.
3. The annotated Git tag `vX.Y.Z` identifies the exact released commit.
4. The GitHub release asset is built from that tagged commit.

This keeps release publication out of local workstation state. Local tooling can
prepare a release PR or build an archive for inspection, but the official
published archive is created by GitHub Actions.

## Files

| File | Purpose |
|------|---------|
| `release-orion-code.sh` | Local helper for release PR preparation and archive builds. |
| `.github/workflows/release.yml` | CI workflow that validates the merged release request, creates the tag, builds the archive, and publishes the GitHub release. |
| `VERSION` | Source-controlled version in bare `X.Y.Z` format. |
| `dist/orion-vX.Y.Z.tar.gz` | Archive produced by local builds and by CI. |

## Release Flow

### 1. Prepare the release PR

Run from a clean working tree:

```bash
./release-orion-code.sh prepare 1.3.0
```

The prepare command:

1. Validates that the version is in `X.Y.Z` format.
2. Requires `gh` and verifies `gh auth status`.
3. Requires a clean working tree.
4. Updates local `master` from `origin/master` with a fast-forward pull.
5. Verifies that `release/vX.Y.Z` does not already exist locally or remotely.
6. Verifies that tag `vX.Y.Z` does not already exist on `origin`.
7. Creates branch `release/vX.Y.Z`.
8. Writes the new version to `VERSION`.
9. Commits the version bump.
10. Pushes the release branch.
11. Opens a PR against `master`.

No official archive is published by this command.

### 2. Review and merge

Review the PR normally. The release branch should contain the `VERSION` bump.
If the team wants release notes or changelog updates in source control, include
them in the same PR and review them together.

Merging the PR to `master` is the release approval step.

### 3. CI tags, builds, and publishes

When the `VERSION` change lands on `master`, `.github/workflows/release.yml`
runs automatically.

The workflow:

1. Checks out the repository with full Git history.
2. Reads `VERSION`.
3. Validates `X.Y.Z` format.
4. Confirms that a manually supplied workflow version, if present, matches
   `VERSION`.
5. Confirms that tag `vX.Y.Z` does not already exist on `origin`.
6. Sets up Go.
7. Runs `./release-orion-code.sh build X.Y.Z`.
8. Verifies that `dist/orion-vX.Y.Z.tar.gz` exists and can be read.
9. Creates annotated tag `vX.Y.Z` on the merged commit.
10. Pushes the tag.
11. Creates the GitHub release.
12. Uploads the archive.
13. Generates release notes from GitHub history.

## Versioning

`VERSION` contains a bare semantic version:

```text
1.3.0
```

Derived names:

| Item | Value |
|------|-------|
| Release branch | `release/v1.3.0` |
| Git tag | `v1.3.0` |
| GitHub release title | `Orion v1.3.0` |
| Archive | `orion-v1.3.0.tar.gz` |

The `v` prefix is used for branch names, tags, release names, and archive names.
It is not stored in `VERSION`.

## Local Commands

```bash
./release-orion-code.sh prepare <version>
./release-orion-code.sh build [version]
```

| Command | Description |
|---------|-------------|
| `prepare <version>` | Creates the release branch, commits `VERSION`, pushes the branch, and opens the PR. |
| `build [version]` | Builds `dist/orion-vX.Y.Z.tar.gz` from the current checkout. If omitted, the version is read from `VERSION`. |

## Prerequisites

| Tool | Purpose | Required for |
|------|---------|--------------|
| `git` | Repository state, branch, and tag checks | Always |
| `make` | Builds Go binaries through the root `Makefile` | `build`, CI |
| GNU `tar` | Creates deterministic archives | `build`, CI |
| `gh` CLI | Opens PRs locally and creates releases in CI | `prepare`, CI |

Local macOS archive builds require GNU tar as `gtar`:

```bash
brew install gnu-tar
```

Local PR preparation requires GitHub CLI authentication:

```bash
gh auth login
```

Local archive builds do not require GitHub authentication.

## Archive Contents

The archive is written to `dist/orion-vX.Y.Z.tar.gz`.

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

The build copies named Go binaries from `${GOBIN:-$HOME/go/bin}`. Shell scripts
and SQL files are discovered from the repository and copied in sorted order.

## Deterministic Archive Settings

Archive creation uses GNU tar with stable metadata:

```text
--sort=name
--owner=0
--group=0
--numeric-owner
--mtime=@<source_date_epoch>
```

By default, `<source_date_epoch>` is the timestamp of the current Git commit.
Set `SOURCE_DATE_EPOCH` to override it.

These settings reduce archive drift between builds from the same commit. Full
binary reproducibility still depends on the Go build inputs and toolchain.

## Build Only

Build the version in `VERSION`:

```bash
./release-orion-code.sh build
```

Build an explicit version from the current checkout:

```bash
./release-orion-code.sh build 1.3.0
```

Useful environment overrides:

| Variable | Description |
|----------|-------------|
| `DIST_DIR` | Archive output directory. Defaults to `<repo>/dist`. |
| `DEPLOY_ROOT` | Temporary package root. Defaults to a `mktemp` directory. |
| `GOBIN` | Directory containing built Go binaries. Defaults to `$HOME/go/bin`. |
| `SOURCE_DATE_EPOCH` | Archive file timestamp. Defaults to the current Git commit time. |

## Manual Workflow Dispatch

The release workflow supports manual `workflow_dispatch`.

Manual dispatch is useful when a release needs to be retried after an
infrastructure failure. If a version input is supplied, it must match the
selected commit's `VERSION` file. This prevents publishing one version label
from a different source revision.

If a workflow run already created the remote tag but failed while creating the
GitHub release, treat the tag as release identity. Prefer creating or uploading
the release asset for that existing tag instead of deleting and recreating it.

## Error Handling

`release-orion-code.sh` runs with `set -euo pipefail` and stops on the first
failure.

Common failures:

| Symptom | Likely cause |
|---------|--------------|
| `version must be in X.Y.Z format` | The supplied version is missing or malformed. |
| `working tree is not clean` | Commit or stash local changes before `prepare`. |
| `gh is not authenticated` | Run `gh auth login` before `prepare`. |
| `local branch 'release/vX.Y.Z' already exists` | A local release branch already exists. |
| `remote branch 'release/vX.Y.Z' already exists` | A release PR branch already exists on origin. |
| `tag 'vX.Y.Z' already exists on origin` | That version has already been released or partially released. |
| `GNU tar is required` | Install GNU tar locally, usually as `gtar` on macOS. |
| `required release file is missing` | `make all` did not produce an expected binary or a required package file is absent. |

