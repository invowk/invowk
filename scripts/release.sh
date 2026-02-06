#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Release helper script for invowk.
#
# Usage:
#   ./scripts/release.sh tag <version> [yes] [dry_run]
#   ./scripts/release.sh bump <type> [prerelease] [promote] [yes] [dry_run]
#
# Modes:
#   tag   - Create and push a signed tag for the given version.
#   bump  - Compute the next version from existing tags, then tag and push.
#
# Parameters:
#   version    - Semver version string (e.g., v1.0.0, v0.1.0-alpha.1)
#   type       - Bump type: major, minor, or patch
#   prerelease - Pre-release label: alpha, beta, or rc (optional)
#   promote    - Set to "1" to allow promoting a prerelease stream to stable
#   yes        - Set to "1" to skip confirmation prompt
#   dry_run    - Set to "1" to show computed version without git operations

set -euo pipefail

# Change to repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# Color output (only when stdout is a terminal)
# ---------------------------------------------------------------------------
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    RED='' GREEN='' YELLOW='' CYAN='' BOLD='' RESET=''
fi

die()  { echo -e "${RED}ERROR: $*${RESET}" >&2; exit 1; }
info() { echo -e "${CYAN}$*${RESET}"; }
warn() { echo -e "${YELLOW}WARNING: $*${RESET}"; }
ok()   { echo -e "${GREEN}$*${RESET}"; }

# ---------------------------------------------------------------------------
# Semver helpers
# ---------------------------------------------------------------------------

# Validate a version string matches vMAJOR.MINOR.PATCH[-PRERELEASE]
validate_semver() {
    local version="$1"
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$ ]]; then
        die "Invalid semver: '$version'. Expected format: v1.2.3 or v1.2.3-alpha.1"
    fi
}

# Parse version components from vMAJOR.MINOR.PATCH[-pre.N]
get_major() { echo "$1" | sed 's/^v//' | cut -d. -f1; }
get_minor() { echo "$1" | sed 's/^v//' | cut -d. -f2; }
get_patch() {
    local patch_part
    patch_part="$(echo "$1" | sed 's/^v//' | cut -d. -f3)"
    # Strip any prerelease suffix (e.g., "3-alpha" -> "3")
    echo "${patch_part%%-*}"
}

# Get the base version without prerelease suffix (v1.2.3-alpha.1 -> v1.2.3)
get_base_version() {
    local version="$1"
    echo "${version%%-*}"
}

# ---------------------------------------------------------------------------
# Tag discovery
# ---------------------------------------------------------------------------

# Get the latest stable tag (no prerelease suffix)
get_latest_stable_tag() {
    git tag --list 'v*' 2>/dev/null \
        | { grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true; } \
        | sort -V \
        | tail -1
}

# Get the latest prerelease tag matching a base version and label
# e.g., get_latest_prerelease_tag "v1.3.0" "alpha" -> v1.3.0-alpha.3
get_latest_prerelease_tag() {
    local base="$1"
    local label="$2"
    git tag --list "${base}-${label}.*" 2>/dev/null \
        | sort -V \
        | tail -1
}

# Get all prerelease tags for a base version (any label)
get_all_prerelease_tags() {
    local base="$1"
    git tag --list "${base}-*" 2>/dev/null \
        | { grep -E "^${base}-[a-zA-Z]+\.[0-9]+$" || true; } \
        | sort -V
}

# ---------------------------------------------------------------------------
# Bump logic
# ---------------------------------------------------------------------------

# Apply a version bump to a base version
# bump_version "v1.2.3" "minor" -> "v1.3.0"
bump_version() {
    local version="$1"
    local bump_type="$2"
    local major minor patch

    major="$(get_major "$version")"
    minor="$(get_minor "$version")"
    patch="$(get_patch "$version")"

    case "$bump_type" in
        major) echo "v$((major + 1)).0.0" ;;
        minor) echo "v${major}.$((minor + 1)).0" ;;
        patch) echo "v${major}.${minor}.$((patch + 1))" ;;
        *) die "Invalid bump type: '$bump_type'. Must be major, minor, or patch." ;;
    esac
}

# Compute the next version based on bump type and optional prerelease label.
# Sets global variable NEXT_VERSION.
compute_next_version() {
    local bump_type="$1"
    local prerelease="${2:-}"
    local promote="${3:-}"

    # Find latest stable tag, default to v0.0.0 if none
    local latest_stable
    latest_stable="$(get_latest_stable_tag)"
    if [ -z "$latest_stable" ]; then
        latest_stable="v0.0.0"
        info "No stable tags found, starting from v0.0.0"
    else
        info "Latest stable tag: ${BOLD}${latest_stable}${RESET}"
    fi

    # Bump to target base version
    local target_base
    target_base="$(bump_version "$latest_stable" "$bump_type")"

    if [ -z "$prerelease" ]; then
        # Stable release: check for existing prerelease tags (promotion guard)
        local existing_prereleases
        existing_prereleases="$(get_all_prerelease_tags "$target_base")"

        if [ -n "$existing_prereleases" ]; then
            if [ "$promote" != "1" ]; then
                echo ""
                warn "Prerelease tags exist for ${BOLD}${target_base}${RESET}:"
                echo "$existing_prereleases" | while IFS= read -r tag; do
                    echo "  - $tag"
                done
                echo ""
                die "This would promote a prerelease stream to stable. Pass PROMOTE=1 to confirm."
            fi
            echo ""
            info "Promoting prerelease stream to stable (PROMOTE=1):"
            echo "$existing_prereleases" | while IFS= read -r tag; do
                echo "  - $tag"
            done
        fi

        NEXT_VERSION="$target_base"
    else
        # Prerelease: find latest with this label and increment
        local latest_pre
        latest_pre="$(get_latest_prerelease_tag "$target_base" "$prerelease")"

        if [ -n "$latest_pre" ]; then
            # Extract numeric suffix and increment
            local current_num
            current_num="$(echo "$latest_pre" | grep -oE '[0-9]+$')"
            NEXT_VERSION="${target_base}-${prerelease}.$((current_num + 1))"
            info "Continuing prerelease stream: ${BOLD}${latest_pre}${RESET} -> ${BOLD}${NEXT_VERSION}${RESET}"
        else
            NEXT_VERSION="${target_base}-${prerelease}.1"
            info "Starting new prerelease stream: ${BOLD}${NEXT_VERSION}${RESET}"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Prerequisites check
# ---------------------------------------------------------------------------
check_prerequisites() {
    local version="$1"

    # Must be on main branch
    local current_branch
    current_branch="$(git rev-parse --abbrev-ref HEAD)"
    if [ "$current_branch" != "main" ]; then
        die "Must be on 'main' branch (currently on '${current_branch}')."
    fi

    # Working tree must be clean
    if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
        die "Working tree is dirty. Commit or stash changes before releasing."
    fi

    # Tag must not already exist
    if git rev-parse "refs/tags/${version}" >/dev/null 2>&1; then
        die "Tag '${version}' already exists."
    fi

    # Remote must be reachable
    if ! git ls-remote --exit-code origin >/dev/null 2>&1; then
        die "Cannot reach remote 'origin'. Check your network connection."
    fi
}

# ---------------------------------------------------------------------------
# Create and push tag
# ---------------------------------------------------------------------------
create_and_push_tag() {
    local version="$1"
    local skip_confirm="${2:-}"
    local dry_run="${3:-}"

    local head_sha
    head_sha="$(git rev-parse --short HEAD)"

    echo ""
    echo -e "${BOLD}=== Release Summary ===${RESET}"
    echo -e "  Version: ${BOLD}${version}${RESET}"
    echo -e "  Commit:  ${head_sha} ($(git log -1 --format='%s' HEAD))"
    echo -e "  Branch:  main"
    echo -e "  Remote:  origin"
    echo ""

    if [ "$dry_run" = "1" ]; then
        ok "[DRY RUN] Would create signed tag '${version}' and push to origin."
        return 0
    fi

    # Confirmation prompt
    if [ "$skip_confirm" != "1" ]; then
        echo -en "Create signed tag ${BOLD}${version}${RESET} and push to origin? [y/N] "
        read -r answer
        if [[ ! "$answer" =~ ^[Yy]$ ]]; then
            info "Aborted."
            exit 0
        fi
    fi

    # Create signed tag
    info "Creating signed tag ${version}..."
    git tag -s "$version" -m "Release ${version}"

    # Push tag to origin
    info "Pushing tag to origin..."
    git push origin "$version"

    echo ""
    ok "Release ${version} tagged and pushed successfully!"
    echo ""
    info "GitHub Actions will now build and publish the release."
    info "Monitor at: https://github.com/invowk/invowk/actions"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    local mode="${1:-}"

    case "$mode" in
        tag)
            local version="${2:-}"
            local yes="${3:-}"
            local dry_run="${4:-}"

            [ -z "$version" ] && die "VERSION is required. Usage: ./scripts/release.sh tag <version>"
            validate_semver "$version"
            check_prerequisites "$version"
            create_and_push_tag "$version" "$yes" "$dry_run"
            ;;

        bump)
            local bump_type="${2:-}"
            local prerelease="${3:-}"
            local promote="${4:-}"
            local yes="${5:-}"
            local dry_run="${6:-}"

            [ -z "$bump_type" ] && die "TYPE is required. Usage: ./scripts/release.sh bump <type> [prerelease]"

            # Validate bump type
            case "$bump_type" in
                major|minor|patch) ;;
                *) die "Invalid TYPE: '${bump_type}'. Must be major, minor, or patch." ;;
            esac

            # Validate prerelease label if provided
            if [ -n "$prerelease" ]; then
                case "$prerelease" in
                    alpha|beta|rc) ;;
                    *) die "Invalid PRERELEASE: '${prerelease}'. Must be alpha, beta, or rc." ;;
                esac
            fi

            # Compute next version
            compute_next_version "$bump_type" "$prerelease" "$promote"

            # Check prerequisites with computed version
            check_prerequisites "$NEXT_VERSION"

            # Create and push
            create_and_push_tag "$NEXT_VERSION" "$yes" "$dry_run"
            ;;

        *)
            die "Unknown mode: '${mode}'. Usage: ./scripts/release.sh {tag|bump} ..."
            ;;
    esac
}

main "$@"
