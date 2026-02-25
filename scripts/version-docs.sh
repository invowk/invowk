#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Automated documentation versioning for the Docusaurus website.
#
# This script snapshots the current docs for a given release version,
# creates immutable version-scoped asset snapshots (snippets + diagrams),
# copies i18n translations, updates docusaurus.config.ts, and validates
# all version asset references.
#
# All versions (stable and pre-release) are kept indefinitely. The default
# landing version is the latest stable release; if none exists, the latest
# pre-release is used instead.
#
# Usage:
#   ./scripts/version-docs.sh <version>
#
# Environment:
#   ALLOW_MISSING_LOCALES=1   Allow missing locale source docs/labels during
#                             copy_i18n_translations (emergency bypass).
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

# Sort versions.json by semver descending (newest first).
# Docusaurus prepends new versions to the array, so retroactively versioning
# an older release puts it above newer ones. This function restores semver order
# after each docs:version run.
sort_versions_json() {
    if [ ! -f "$VERSIONS_FILE" ]; then
        return 0
    fi

    node -e "
        const fs = require('fs');

        // Parse 'X.Y.Z' or 'X.Y.Z-label.N' into comparable parts
        function parseSemver(v) {
            const [core, ...preParts] = v.split('-');
            const [major, minor, patch] = core.split('.').map(Number);
            const pre = preParts.length > 0 ? preParts.join('-') : null;
            return { major, minor, patch, pre };
        }

        // Compare pre-release identifiers per semver spec:
        // numeric identifiers compared as integers, string identifiers lexically.
        function comparePrerelease(a, b) {
            const partsA = a.split('.');
            const partsB = b.split('.');
            const len = Math.max(partsA.length, partsB.length);
            for (let i = 0; i < len; i++) {
                if (i >= partsA.length) return -1; // fewer fields = lower precedence
                if (i >= partsB.length) return 1;
                const isNumA = /^\d+$/.test(partsA[i]);
                const isNumB = /^\d+$/.test(partsB[i]);
                if (isNumA && isNumB) {
                    const diff = Number(partsA[i]) - Number(partsB[i]);
                    if (diff !== 0) return diff;
                } else if (isNumA !== isNumB) {
                    return isNumA ? -1 : 1; // numeric < string
                } else {
                    if (partsA[i] < partsB[i]) return -1;
                    if (partsA[i] > partsB[i]) return 1;
                }
            }
            return 0;
        }

        // Sort descending: higher versions first.
        function compareSemver(a, b) {
            const pa = parseSemver(a);
            const pb = parseSemver(b);
            if (pa.major !== pb.major) return pb.major - pa.major;
            if (pa.minor !== pb.minor) return pb.minor - pa.minor;
            if (pa.patch !== pb.patch) return pb.patch - pa.patch;
            // Both stable? Equal.
            if (!pa.pre && !pb.pre) return 0;
            // Stable > pre-release (stable sorts first in descending).
            if (!pa.pre) return -1;
            if (!pb.pre) return 1;
            // Both pre-release: compare identifiers (descending).
            return -comparePrerelease(pa.pre, pb.pre);
        }

        const versions = JSON.parse(fs.readFileSync('$VERSIONS_FILE', 'utf8'));
        versions.sort(compareSemver);
        fs.writeFileSync('$VERSIONS_FILE', JSON.stringify(versions, null, 2) + '\n');
    "
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
    local allow_missing_locales="${ALLOW_MISSING_LOCALES:-0}"

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
            if [ "$allow_missing_locales" = "1" ]; then
                echo "::warning::Missing i18n docs for locale '${locale}' at ${src_docs} — skipping because ALLOW_MISSING_LOCALES=1."
            else
                die "Missing i18n docs for locale '${locale}' at ${src_docs}. Set ALLOW_MISSING_LOCALES=1 to bypass in emergency cases."
            fi
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
            if [ "$allow_missing_locales" = "1" ]; then
                echo "::warning::Missing i18n sidebar labels for locale '${locale}' at ${src_json} — skipping because ALLOW_MISSING_LOCALES=1."
            else
                die "Missing i18n sidebar labels for locale '${locale}' at ${src_json}. Set ALLOW_MISSING_LOCALES=1 to bypass in emergency cases."
            fi
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

    # Read versions.json to determine lastVersion.
    # Use semver comparison to find the highest stable version rather than
    # relying on array order (which can be wrong after retroactive versioning).
    read_versions

    local last_version=""
    last_version=$(node -e "
        const versions = $(cat "$VERSIONS_FILE");

        function parseSemver(v) {
            const [core, ...preParts] = v.split('-');
            const [major, minor, patch] = core.split('.').map(Number);
            return { major, minor, patch, pre: preParts.length > 0 };
        }

        // Find highest stable version by semver comparison
        let best = null;
        let bestParsed = null;
        for (const v of versions) {
            const p = parseSemver(v);
            if (p.pre) continue; // skip pre-releases
            if (!bestParsed
                || p.major > bestParsed.major
                || (p.major === bestParsed.major && p.minor > bestParsed.minor)
                || (p.major === bestParsed.major && p.minor === bestParsed.minor && p.patch > bestParsed.patch)) {
                best = v;
                bestParsed = p;
            }
        }

        // Fall back to highest pre-release if no stable version exists
        if (!best && versions.length > 0) {
            best = versions[0]; // versions.json is sorted descending after sort step
        }

        if (best) process.stdout.write(best);
    ")

    if [ -z "$last_version" ] && [ "${#EXISTING_VERSIONS[@]}" -gt 0 ]; then
        last_version="${EXISTING_VERSIONS[0]}"
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

        // Add banner entries for pre-release versions.
        // Skip the lastVersion — Docusaurus's 'unreleased' banner links TO lastVersion,
        // so setting it ON lastVersion creates a contradictory self-referential message.
        for (const pre of prereleaseVersions) {
            if (pre === lastVersion) continue;
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
    info "Step 1/7: Creating docs version snapshot..."
    create_version_snapshot "$version"
    echo ""

    # Step 2: Sort versions.json by semver descending
    info "Step 2/7: Sorting versions.json by semver..."
    sort_versions_json
    ok "versions.json sorted."
    echo ""

    # Step 3: Snapshot version assets (snippets + diagrams)
    info "Step 3/7: Snapshotting version assets..."
    node "$REPO_ROOT/scripts/snapshot-version-assets.mjs" "$version"
    echo ""

    # Step 4: Copy i18n translations
    info "Step 4/7: Copying i18n translations..."
    copy_i18n_translations "$version"
    echo ""

    # Step 5: Validate docs parity across locales
    info "Step 5/7: Validating docs parity..."
    node "$REPO_ROOT/scripts/validate-docs-parity.mjs" --mode strict --all-locales
    echo ""

    # Step 6: Update docusaurus.config.ts
    info "Step 6/7: Updating docusaurus.config.ts..."
    update_docusaurus_config "$version"
    echo ""

    # Step 7: Validate version assets
    info "Step 7/7: Validating version assets..."
    node "$REPO_ROOT/scripts/validate-version-assets.mjs"
    echo ""

    ok "Documentation versioning complete for ${BOLD}v${version}${RESET}!"
    echo ""
    info "Files modified:"
    info "  - website/versions.json"
    info "  - website/versioned_docs/version-${version}/"
    info "  - website/versioned_sidebars/version-${version}-sidebars.json"
    info "  - website/src/components/Snippet/versions/v${version}.ts"
    info "  - website/src/components/Snippet/versions/index.ts"
    info "  - website/src/components/Diagram/versions/v${version}.ts"
    info "  - website/src/components/Diagram/versions/index.ts"
    info "  - website/static/diagrams/v${version}/"
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
