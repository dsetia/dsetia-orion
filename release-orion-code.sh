#!/usr/bin/env bash
#
# Prepare Orion releases and build deterministic release archives.
#
# Long-term release flow:
#   1. ./release-orion-code.sh prepare 1.2.3
#   2. Review and merge the generated release PR.
#   3. The GitHub release workflow tags the merged commit, builds the archive,
#      and publishes the GitHub release asset.

set -euo pipefail

die() { echo "Error: $*" >&2; exit 1; }
info() { echo "==> $*"; }

usage() {
    cat <<'EOF'
Usage:
  ./release-orion-code.sh prepare <version>
  ./release-orion-code.sh build [version]

Commands:
  prepare <version>  Create release/v<version>, update VERSION, push it, and
                     open a GitHub PR against master. Requires gh auth.
  build [version]    Build dist/orion-v<version>.tar.gz from the current tree.
                     If version is omitted, VERSION is read from the repo root.

Environment:
  DIST_DIR           Archive output directory. Default: <repo>/dist
  DEPLOY_ROOT        Temporary package root. Default: mktemp directory
EOF
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "'$1' is required"
}

gnu_tar() {
    if tar --version 2>/dev/null | grep -q 'GNU tar'; then
        echo tar
        return
    fi

    if command -v gtar >/dev/null 2>&1 && gtar --version 2>/dev/null | grep -q 'GNU tar'; then
        echo gtar
        return
    fi

    die "GNU tar is required for deterministic archives; install it as 'gtar' on macOS"
}

validate_version() {
    [[ "${1:-}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
        die "version must be in X.Y.Z format (got '${1:-}')"
}

repo_root() {
    git rev-parse --show-toplevel 2>/dev/null
}

ensure_clean_tree() {
    if [[ -n "$(git status --porcelain)" ]]; then
        die "working tree is not clean; commit or stash local changes first"
    fi
}

read_version() {
    local version_file=$1
    [[ -f "$version_file" ]] || die "VERSION file not found: $version_file"
    tr -d '[:space:]' < "$version_file"
}

ensure_master_current() {
    local base_branch=${1:-master}
    git fetch origin "$base_branch"
    git checkout "$base_branch"
    git pull --ff-only origin "$base_branch"
}

prepare_release() {
    local version=$1
    validate_version "$version"
    require_cmd gh
    gh auth status >/dev/null 2>&1 || die "gh is not authenticated; run 'gh auth login' first"

    local root
    root=$(repo_root)
    cd "$root"

    ensure_clean_tree
    ensure_master_current master

    local tag="v${version}"
    local branch="release/${tag}"

    if git rev-parse --verify "$branch" >/dev/null 2>&1; then
        die "local branch '$branch' already exists"
    fi
    if git ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
        die "remote branch '$branch' already exists"
    fi
    if git ls-remote --exit-code --tags origin "refs/tags/${tag}" >/dev/null 2>&1; then
        die "tag '$tag' already exists on origin"
    fi

    git checkout -b "$branch"
    printf '%s\n' "$version" > VERSION
    git add VERSION
    git commit -m "chore: release ${tag}"
    git push -u origin "$branch"

    gh pr create \
        --title "Release ${tag}" \
        --body "Bump VERSION to \`${tag}\`. After this PR is merged, the release workflow will tag the merged commit, build the archive, and publish the GitHub release." \
        --base master \
        --head "$branch"

    info "Release PR created for ${tag}"
}

copy_if_exists() {
    local src=$1
    local dst=$2
    [[ -e "$src" ]] || die "required release file is missing: $src"
    cp "$src" "$dst"
}

build_archive() {
    local root
    root=$(repo_root)
    cd "$root"

    local version=${1:-}
    if [[ -z "$version" ]]; then
        version=$(read_version "$root/VERSION")
    fi
    validate_version "$version"

    local tag="v${version}"
    local archive="orion-${tag}.tar.gz"
    local dist_dir="${DIST_DIR:-$root/dist}"
    local deploy_root="${DEPLOY_ROOT:-}"
    local cleanup=0

    require_cmd make
    local tar_cmd
    tar_cmd=$(gnu_tar)

    local source_date_epoch="${SOURCE_DATE_EPOCH:-}"
    if [[ -z "$source_date_epoch" ]]; then
        source_date_epoch=$(git log -1 --format=%ct)
    fi

    if [[ -z "$deploy_root" ]]; then
        deploy_root=$(mktemp -d "${TMPDIR:-/tmp}/orion-release.XXXXXX")
        cleanup=1
    fi
    trap '[[ ${cleanup:-0} -eq 1 ]] && rm -rf "$deploy_root"' EXIT

    local package_root="$deploy_root/package"
    rm -rf "$package_root"
    mkdir -p "$package_root/opt/bin" "$package_root/opt/db" "$package_root/opt/docker"
    mkdir -p "$dist_dir"

    info "Building Orion binaries"
    make all

    local bin_dir="${GOBIN:-$HOME/go/bin}"
    local binaries=(apis dbtool updater objupdater provisioner)
    local binary
    for binary in "${binaries[@]}"; do
        copy_if_exists "$bin_dir/$binary" "$package_root/opt/bin/"
    done

    local util
    while IFS= read -r util; do
        copy_if_exists "$util" "$package_root/opt/bin/"
    done < <(find "$root/utils" -maxdepth 1 -type f -name '*.sh' | sort)

    local db_file
    while IFS= read -r db_file; do
        copy_if_exists "$db_file" "$package_root/opt/bin/"
    done < <(find "$root/db" -maxdepth 1 -type f -name '*.sh' | sort)

    while IFS= read -r db_file; do
        copy_if_exists "$db_file" "$package_root/opt/db/"
    done < <(find "$root/db" -maxdepth 1 -type f -name '*.sql' | sort)

    copy_if_exists "$root/docker-compose.yml" "$package_root/opt/docker/"

    info "Creating deterministic archive $dist_dir/$archive"
    "$tar_cmd" \
        --sort=name \
        --owner=0 \
        --group=0 \
        --numeric-owner \
        --mtime="@${source_date_epoch}" \
        -czf "$dist_dir/$archive" \
        -C "$package_root" .

    info "Created $dist_dir/$archive"
}

main() {
    [[ $# -ge 1 ]] || { usage; exit 1; }

    case "$1" in
        prepare)
            [[ $# -eq 2 ]] || { usage; exit 1; }
            prepare_release "$2"
            ;;
        build)
            [[ $# -le 2 ]] || { usage; exit 1; }
            build_archive "${2:-}"
            ;;
        -h|--help|help)
            usage
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

main "$@"
