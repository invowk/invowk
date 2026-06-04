#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GO_CMD="${GO_CMD:-go}"
GOLANGCI_LINT_TOOL="golangci-lint"
GOLANGCI_LINT_MODULE="github.com/golangci/golangci-lint/v2"
GOLANGCI_LINT_VERSION="v2.12.2"

GOLANGCI_LINT_BIN=""

usage() {
	cat <<'EOF'
Usage: scripts/golangci-lint.sh <command> [args...]

Commands:
  run                  Run golangci-lint for root and tools/goplint.
  root-run             Run golangci-lint for the root module.
  tools-run            Run golangci-lint for tools/goplint.
  fmt                  Check formatter diffs for root and tools/goplint.
  root-fmt             Check formatter diffs for the root module.
  tools-fmt            Check formatter diffs for tools/goplint.
  config-verify        Verify both golangci-lint config files.
  root-config-verify   Verify the root golangci-lint config.
  tools-config-verify  Verify the tools/goplint golangci-lint config.
  linters              Print effective linter JSON for both modules.
  root-linters         Print effective linter JSON for the root module.
  tools-linters        Print effective linter JSON for tools/goplint.
  version              Print the normalized golangci-lint version.
  help                 Show this help.

Environment:
  GO_CMD               Go command used to resolve the pinned tool (default: go).
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

module_dir() {
	local module="$1"

	case "$module" in
		root)
			printf '%s\n' "$REPO_ROOT"
			;;
		tools)
			printf '%s\n' "$REPO_ROOT/tools/goplint"
			;;
		*)
			die "unknown golangci-lint module: $module"
			;;
	esac
}

module_label() {
	local module="$1"

	case "$module" in
		root)
			printf 'root module\n'
			;;
		tools)
			printf 'tools/goplint module\n'
			;;
		*)
			die "unknown golangci-lint module: $module"
			;;
	esac
}

golangci_lint_binary() {
	(cd "$REPO_ROOT" && "$GO_CMD" tool -n "$GOLANGCI_LINT_TOOL")
}

verify_golangci_lint_version() {
	local binary
	local resolved_version

	if [[ -n "$GOLANGCI_LINT_BIN" ]]; then
		return 0
	fi

	binary="$(golangci_lint_binary)" ||
		die "failed to resolve $GOLANGCI_LINT_TOOL from the root go.mod tool directive"
	[[ -x "$binary" ]] || die "resolved $GOLANGCI_LINT_TOOL is not executable: $binary"

	resolved_version="$("$GO_CMD" version -m "$binary" | awk -v module="$GOLANGCI_LINT_MODULE" '$1 == "mod" && $2 == module {print $3; found=1} END {if (!found) exit 1}')" ||
		die "failed to read $GOLANGCI_LINT_MODULE version from $binary"

	if [[ "$resolved_version" != "$GOLANGCI_LINT_VERSION" ]]; then
		die "expected $GOLANGCI_LINT_MODULE $GOLANGCI_LINT_VERSION, got $resolved_version"
	fi

	GOLANGCI_LINT_BIN="$binary"
}

run_lint() {
	local module="$1"
	shift

	verify_golangci_lint_version
	printf 'Running golangci-lint (%s)...\n' "$(module_label "$module")"
	(cd "$(module_dir "$module")" && "$GOLANGCI_LINT_BIN" run --config=.golangci.toml "$@" ./...)
}

run_format_check() {
	local module="$1"
	shift

	verify_golangci_lint_version
	printf 'Checking golangci-lint formatters (%s)...\n' "$(module_label "$module")"
	(cd "$(module_dir "$module")" && "$GOLANGCI_LINT_BIN" fmt --config=.golangci.toml --diff "$@")
}

run_config_verify() {
	local module="$1"
	shift

	verify_golangci_lint_version
	printf 'Verifying golangci-lint config (%s)...\n' "$(module_label "$module")"
	(cd "$(module_dir "$module")" && "$GOLANGCI_LINT_BIN" config verify --config=.golangci.toml "$@")
}

print_linters() {
	local module="$1"
	shift

	verify_golangci_lint_version
	printf 'Effective golangci-lint linters (%s):\n' "$(module_label "$module")" >&2
	(cd "$(module_dir "$module")" && "$GOLANGCI_LINT_BIN" linters --config=.golangci.toml --json "$@")
}

print_version() {
	verify_golangci_lint_version
	"$GOLANGCI_LINT_BIN" version
}

main() {
	local command="${1:-help}"

	if [[ $# -gt 0 ]]; then
		shift
	fi

	case "$command" in
		run)
			run_lint root "$@"
			run_lint tools "$@"
			;;
		root-run)
			run_lint root "$@"
			;;
		tools-run)
			run_lint tools "$@"
			;;
		fmt)
			run_format_check root "$@"
			run_format_check tools "$@"
			;;
		root-fmt)
			run_format_check root "$@"
			;;
		tools-fmt)
			run_format_check tools "$@"
			;;
		config-verify)
			run_config_verify root "$@"
			run_config_verify tools "$@"
			;;
		root-config-verify)
			run_config_verify root "$@"
			;;
		tools-config-verify)
			run_config_verify tools "$@"
			;;
		linters)
			print_linters root "$@"
			print_linters tools "$@"
			;;
		root-linters)
			print_linters root "$@"
			;;
		tools-linters)
			print_linters tools "$@"
			;;
		version)
			print_version
			;;
		help|--help|-h)
			usage
			;;
		*)
			usage >&2
			die "unknown golangci-lint command: $command"
			;;
	esac
}

if [[ "${INVOWK_GOLANGCI_LINT_TESTING:-0}" != "1" ]]; then
	main "$@"
fi
