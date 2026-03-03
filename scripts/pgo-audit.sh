#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

GO_CMD="${GOCMD:-go}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILE_PATH="${1:-$ROOT_DIR/default.pgo}"

if ! command -v "$GO_CMD" >/dev/null 2>&1; then
	echo "Error: Go command not found: $GO_CMD" >&2
	exit 1
fi

if [[ ! -f "$PROFILE_PATH" ]]; then
	echo "Error: PGO profile not found: $PROFILE_PATH" >&2
	echo "Hint: run 'make pgo-profile-parse-discovery' (or another pgo-profile target)." >&2
	exit 1
fi

tmp_out="$(mktemp)"
trap 'rm -f "$tmp_out"' EXIT

if ! "$GO_CMD" tool pprof -top "$PROFILE_PATH" >"$tmp_out" 2>&1; then
	cat "$tmp_out" >&2
	echo "" >&2
	echo "Error: unable to inspect profile at $PROFILE_PATH" >&2
	exit 1
fi

if grep -Fq "invowk-cli/" "$tmp_out"; then
	echo "Error: stale profile symbols detected (legacy 'invowk-cli/' namespace)." >&2
	echo "Regenerate default.pgo with current package paths." >&2
	exit 1
fi

required_symbols=(
	"github.com/invowk/invowk/pkg/invowkfile.ParseBytes"
	"github.com/invowk/invowk/pkg/invowkmod.ParseInvowkmodBytes"
	"github.com/invowk/invowk/internal/discovery.(*Discovery).LoadAll"
)

missing_count=0
for symbol in "${required_symbols[@]}"; do
	if ! grep -Fq "$symbol" "$tmp_out"; then
		echo "Error: required hot-path symbol missing from profile: $symbol" >&2
		missing_count=$((missing_count + 1))
	fi
done

if [[ "$missing_count" -gt 0 ]]; then
	echo "Hint: regenerate with 'make pgo-profile-parse-discovery'." >&2
	exit 1
fi

echo "PGO audit passed:"
echo "  profile: $PROFILE_PATH"
echo "  checked symbols:"
for symbol in "${required_symbols[@]}"; do
	echo "    - $symbol"
done
