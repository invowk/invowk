#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Normalized goplint tool resolution: the analyzer and its consumer commands
# are pinned through the root go.mod tool directives, mirroring
# scripts/golangci-lint.sh. Every invocation verifies the embedded module
# version so an ambient or stale binary can never masquerade as the pin.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GO_CMD="${GO_CMD:-go}"
GOPLINT_MODULE="github.com/invowk/goplint"
GOPLINT_VERSION="v0.1.0"

usage() {
	cat <<'EOF'
Usage: scripts/goplint.sh <command> [args...]

Commands:
  build     Build the pinned analyzer to bin/goplint and verify its version.
  verify    Verify bin/goplint embeds the pinned goplint module version.
  version   Print the pinned goplint module version.
  help      Show this help.

Environment:
  GO_CMD    Go command used to build and inspect the pinned tool (default: go).
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

verify_binary_version() {
	local binary="$1"
	local resolved_version

	[[ -x "$binary" ]] || die "goplint binary is not executable: $binary"
	resolved_version="$("$GO_CMD" version -m "$binary" | awk -v module="$GOPLINT_MODULE" '$1 == "mod" && $2 == module {print $3; found=1} END {if (!found) exit 1}')" ||
		die "failed to read $GOPLINT_MODULE version from $binary"
	if [[ "$resolved_version" != "$GOPLINT_VERSION" ]]; then
		die "expected $GOPLINT_MODULE $GOPLINT_VERSION, got $resolved_version"
	fi
}

build_analyzer() {
	(cd "$REPO_ROOT" && "$GO_CMD" build -o bin/goplint "$GOPLINT_MODULE")
	verify_binary_version "$REPO_ROOT/bin/goplint"
	printf 'goplint %s built at bin/goplint\n' "$GOPLINT_VERSION"
}

main() {
	local command="${1:-help}"

	if [[ $# -gt 0 ]]; then
		shift
	fi

	case "$command" in
		build)
			build_analyzer
			;;
		verify)
			verify_binary_version "$REPO_ROOT/bin/goplint"
			printf 'bin/goplint matches %s %s\n' "$GOPLINT_MODULE" "$GOPLINT_VERSION"
			;;
		version)
			printf '%s %s\n' "$GOPLINT_MODULE" "$GOPLINT_VERSION"
			;;
		help|--help|-h)
			usage
			;;
		*)
			usage >&2
			die "unknown goplint command: $command"
			;;
	esac
}

if [[ "${INVOWK_GOPLINT_TESTING:-0}" != "1" ]]; then
	main "$@"
fi
