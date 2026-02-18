#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Enhance GoReleaser-generated WinGet installer manifest with fields that
# GoReleaser doesn't natively support: Commands, MinimumOSVersion, Platform.
#
# Usage: GH_TOKEN=<token> bash scripts/enhance-winget-manifest.sh <version>
#
# This script is designed to run as a CI step immediately after GoReleaser
# creates the WinGet PR. It fetches the installer manifest from the fork,
# injects the missing fields, and pushes the update back.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERSION="${1:?Usage: enhance-winget-manifest.sh <version>}"

# Skip pre-releases — GoReleaser uses skip_upload: auto, so no WinGet PR is
# created for pre-release tags.
if [[ "$VERSION" == *-* ]]; then
  echo "Skipping WinGet manifest enhancement for pre-release: $VERSION"
  exit 0
fi

REPO="invowk/winget-pkgs"
BRANCH="invowk-${VERSION}"
FILE_PATH="manifests/i/Invowk/Invowk/${VERSION}/Invowk.Invowk.installer.yaml"
TMPFILE="$(mktemp)"
trap 'rm -f "$TMPFILE"' EXIT

echo "Enhancing WinGet installer manifest for version ${VERSION}..."
echo "  Repo:   ${REPO}"
echo "  Branch: ${BRANCH}"
echo "  File:   ${FILE_PATH}"

# Fetch the current file from the fork. If the file doesn't exist (e.g., WinGet
# PR was not created due to a missing token), exit gracefully.
RESPONSE=$(gh api "repos/${REPO}/contents/${FILE_PATH}?ref=${BRANCH}" 2>/dev/null) || {
  echo "Warning: Could not fetch WinGet manifest from ${REPO}."
  echo "The WinGet PR may not have been created. Skipping enhancement."
  exit 0
}

FILE_SHA=$(echo "$RESPONSE" | jq -r '.sha')
echo "$RESPONSE" | jq -r '.content' | base64 -d > "$TMPFILE"

echo "  SHA:    ${FILE_SHA}"
echo "  Original manifest:"
cat "$TMPFILE"
echo ""

# Inject missing fields using the companion Python script.
# This preserves GoReleaser's exact formatting while inserting fields at the
# correct positions in the manifest. The operation is idempotent.
python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$TMPFILE"

echo "  Enhanced manifest:"
cat "$TMPFILE"
echo ""

# Push the enhanced manifest back to the fork branch.
CONTENT=$(base64 -w0 "$TMPFILE")
gh api -X PUT "repos/${REPO}/contents/${FILE_PATH}" \
  -f message="Add Commands, MinimumOSVersion, Platform to installer manifest" \
  -f branch="${BRANCH}" \
  -f sha="${FILE_SHA}" \
  -f content="${CONTENT}" \
  --silent

echo "WinGet installer manifest enhanced successfully."
