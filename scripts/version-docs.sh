#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Automated documentation versioning for the Docusaurus website.
#
# This script snapshots the current docs for a given release version,
# copies i18n translations, and updates docusaurus.config.ts.
#
# All versions (stable and pre-release) are kept indefinitely. The default
# landing version is the latest stable release; if none exists, the latest
# pre-release is used instead.
#
# Usage:
#   ./scripts/version-docs.sh <version>
#
# Parameters:
#   version - Semver version WITHOUT 'v' prefix (e.g., 0.1.0-alpha.1, 1.0.0)
#
# Examples:
#   ./scripts/version-docs.sh 0.1.0-alpha.1   # Pre-release
#   ./scripts/version-docs.sh 0.1.0-alpha.2   # Another pre-release (alpha.1 kept)
#   ./scripts/version-docs.sh 1.0.0           # Stable release (becomes default)

set -euo pipefail

# Change to repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

WEBSITE_DIR="$REPO_ROOT/website"
VERSIONS_FILE="$WEBSITE_DIR/versions.json"
CONFIG_FILE="$WEBSITE_DIR/docusaurus.config.ts"

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
# Helpers
# ---------------------------------------------------------------------------

# Check if a version string is a pre-release (contains '-')
is_prerelease() {
    [[ "$1" == *-* ]]
}

# Read versions.json into a bash array. Sets EXISTING_VERSIONS.
read_versions() {
    EXISTING_VERSIONS=()
    if [ -f "$VERSIONS_FILE" ]; then
        while IFS= read -r line; do
            EXISTING_VERSIONS+=("$line")
        done < <(node -e "
            const v = require('$VERSIONS_FILE');
            v.forEach(x => console.log(x));
        ")
    fi
}

# ---------------------------------------------------------------------------
# Step 1: Run docusaurus docs:version
# ---------------------------------------------------------------------------
create_version_snapshot() {
    local version="$1"

    # Idempotency guard: check if version already exists
    if [ -f "$VERSIONS_FILE" ]; then
        local exists
        exists=$(node -e "
            const v = require('$VERSIONS_FILE');
            console.log(v.includes('$version') ? 'yes' : 'no');
        ")
        if [ "$exists" = "yes" ]; then
            warn "Version ${BOLD}${version}${RESET} already exists in versions.json — skipping docs:version."
            return 0
        fi
    fi

    info "Creating docs snapshot for version ${BOLD}${version}${RESET}..."
    cd "$WEBSITE_DIR"
    npx docusaurus docs:version "$version"
    cd "$REPO_ROOT"
    ok "Docs snapshot created for ${version}."
}

# ---------------------------------------------------------------------------
# Step 2: Copy i18n translations
# ---------------------------------------------------------------------------
copy_i18n_translations() {
    local version="$1"

    for locale_dir in "$WEBSITE_DIR"/i18n/*/; do
        [ -d "$locale_dir" ] || continue
        local locale
        locale="$(basename "$locale_dir")"
        local docs_dir="${locale_dir}docusaurus-plugin-content-docs"

        # Copy docs content: current/ -> version-<VERSION>/
        local src_docs="${docs_dir}/current"
        local dst_docs="${docs_dir}/version-${version}"

        if [ -d "$src_docs" ]; then
            info "Copying i18n docs for locale ${BOLD}${locale}${RESET}..."
            rm -rf "$dst_docs"
            cp -r "$src_docs" "$dst_docs"
        else
            echo "::warning::Missing i18n docs for locale '${locale}' at ${src_docs} — skipping."
        fi

        # Copy sidebar labels: current.json -> version-<VERSION>.json
        local src_json="${docs_dir}/current.json"
        local dst_json="${docs_dir}/version-${version}.json"

        if [ -f "$src_json" ]; then
            info "Copying i18n sidebar labels for locale ${BOLD}${locale}${RESET}..."
            # Update the version.label.message to reflect the version number
            node -e "
                const fs = require('fs');
                const data = JSON.parse(fs.readFileSync('$src_json', 'utf8'));
                if (data['version.label']) {
                    data['version.label'].message = '$version';
                    data['version.label'].description = 'The label for version $version';
                }
                fs.writeFileSync('$dst_json', JSON.stringify(data, null, 2) + '\n');
            "
        else
            echo "::warning::Missing i18n sidebar labels for locale '${locale}' at ${src_json} — skipping."
        fi
    done

    ok "i18n translations copied."
}

# ---------------------------------------------------------------------------
# Step 3: Update docusaurus.config.ts
# ---------------------------------------------------------------------------
update_docusaurus_config() {
    local version="$1"

    info "Updating docusaurus.config.ts..."

    # Read versions.json to determine lastVersion
    read_versions

    # Find the latest stable version (no '-' in name).
    # versions.json is ordered newest-first, so the first stable hit is the latest.
    local last_version=""
    for v in "${EXISTING_VERSIONS[@]}"; do
        if ! is_prerelease "$v"; then
            last_version="$v"
            break
        fi
    done

    # If no stable version exists, fall back to the latest pre-release
    if [ -z "$last_version" ]; then
        if [ "${#EXISTING_VERSIONS[@]}" -gt 0 ]; then
            last_version="${EXISTING_VERSIONS[0]}"
        fi
    fi

    if [ -z "$last_version" ]; then
        die "No versions found in versions.json after versioning — something went wrong."
    fi

    info "Setting lastVersion to: ${BOLD}${last_version}${RESET}"

    # Collect pre-release versions that need banner config
    local prerelease_versions=()
    for v in "${EXISTING_VERSIONS[@]}"; do
        if is_prerelease "$v"; then
            prerelease_versions+=("$v")
        fi
    done

    # Use Node.js to update the TypeScript config file
    node -e "
        const fs = require('fs');
        const configPath = '$CONFIG_FILE';
        let content = fs.readFileSync(configPath, 'utf8');

        const lastVersion = '$last_version';
        const prereleaseVersions = $(printf '%s\n' "${prerelease_versions[@]:-}" | node -e "
            const lines = [];
            const rl = require('readline').createInterface({ input: process.stdin });
            rl.on('line', l => { if (l.trim()) lines.push(l.trim()); });
            rl.on('close', () => console.log(JSON.stringify(lines)));
        ");

        // Build the versions config block
        // 'current' always maps to the 'next' (unreleased) path
        let versionsEntries = [];
        versionsEntries.push(\"            current: {\");
        versionsEntries.push(\"              label: 'Next',\");
        versionsEntries.push(\"              path: 'next',\");
        versionsEntries.push(\"            },\");

        // Add banner entries for pre-release versions
        for (const pre of prereleaseVersions) {
            versionsEntries.push(\"            '\" + pre + \"': {\");
            versionsEntries.push(\"              banner: 'unreleased',\");
            versionsEntries.push(\"            },\");
        }

        const versionsBlock = versionsEntries.join('\n');

        // Replace the docs config section
        // Match the existing lastVersion + versions block
        const docsRegex = /lastVersion:\s*'[^']*',\s*\n\s*versions:\s*\{[\s\S]*?\},\s*\n\s*\},/;

        const newDocsBlock = \"lastVersion: '\" + lastVersion + \"',\n          versions: {\n\" + versionsBlock + \"\n          },\";

        if (docsRegex.test(content)) {
            content = content.replace(docsRegex, newDocsBlock);
        } else {
            // Fallback: the config may not have been updated before
            // Look for the simpler pattern from the initial config
            const simpleRegex = /lastVersion:\s*'current',\s*\n\s*versions:\s*\{[\s\S]*?\},\s*\n\s*\},/;
            if (simpleRegex.test(content)) {
                content = content.replace(simpleRegex, newDocsBlock);
            } else {
                console.error('ERROR: Could not find versioning config block in docusaurus.config.ts');
                process.exit(1);
            }
        }

        fs.writeFileSync(configPath, content);
        console.log('Updated docusaurus.config.ts successfully.');
    "

    ok "docusaurus.config.ts updated."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    local version="${1:-}"

    if [ -z "$version" ]; then
        die "VERSION is required (without v prefix). Usage: ./scripts/version-docs.sh <version>"
    fi

    # Strip 'v' prefix if accidentally provided
    version="${version#v}"

    # Validate version format (without v prefix)
    if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$ ]]; then
        die "Invalid version format: '${version}'. Expected: X.Y.Z or X.Y.Z-label.N (e.g., 1.0.0, 0.1.0-alpha.1)"
    fi

    echo ""
    echo -e "${BOLD}=== Documentation Versioning ===${RESET}"
    echo -e "  Version: ${BOLD}${version}${RESET}"
    echo -e "  Type:    $(is_prerelease "$version" && echo "Pre-release" || echo "Stable")"
    echo ""

    # Ensure we're in the website directory and dependencies are installed
    if [ ! -d "$WEBSITE_DIR/node_modules" ]; then
        info "Installing website dependencies..."
        cd "$WEBSITE_DIR"
        npm ci
        cd "$REPO_ROOT"
    fi

    # Step 1: Create version snapshot
    info "Step 1/3: Creating docs version snapshot..."
    create_version_snapshot "$version"
    echo ""

    # Step 2: Copy i18n translations
    info "Step 2/3: Copying i18n translations..."
    copy_i18n_translations "$version"
    echo ""

    # Step 3: Update docusaurus.config.ts
    info "Step 3/3: Updating docusaurus.config.ts..."
    update_docusaurus_config "$version"
    echo ""

    ok "Documentation versioning complete for ${BOLD}v${version}${RESET}!"
    echo ""
    info "Files modified:"
    info "  - website/versions.json"
    info "  - website/versioned_docs/version-${version}/"
    info "  - website/versioned_sidebars/version-${version}-sidebars.json"
    for locale_dir in "$WEBSITE_DIR"/i18n/*/; do
        [ -d "$locale_dir" ] || continue
        local locale
        locale="$(basename "$locale_dir")"
        info "  - website/i18n/${locale}/docusaurus-plugin-content-docs/version-${version}/"
        info "  - website/i18n/${locale}/docusaurus-plugin-content-docs/version-${version}.json"
    done
    info "  - website/docusaurus.config.ts"
}

main "$@"
