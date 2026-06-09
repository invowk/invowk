#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GOVULNCHECK_CMD="${GOVULNCHECK_CMD:-govulncheck}"

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

discover_go_modules() {
	git -C "$REPO_ROOT" ls-files 'go.mod' '*/go.mod' |
		while IFS= read -r mod_file; do
			dirname "$mod_file"
		done |
		sort -u
}

module_abs_path() {
	local module_dir="$1"

	if [[ "$module_dir" == "." ]]; then
		printf '%s\n' "$REPO_ROOT"
	else
		printf '%s/%s\n' "$REPO_ROOT" "$module_dir"
	fi
}

run_govulncheck() {
	local module_dir="$1"
	local module_path
	local status

	module_path="$(module_abs_path "$module_dir")"
	printf '==> govulncheck: %s\n' "$module_dir"
	set +e
	(
		cd "$module_path"
		"$GOVULNCHECK_CMD" ./...
	)
	status=$?
	set -e
	if [[ "$status" -ne 0 ]]; then
		printf 'govulncheck failed in module: %s\n' "$module_dir" >&2
		return "$status"
	fi
}

main() {
	local modules=()
	local module_dir

	command -v git >/dev/null 2>&1 || die "git is required"
	command -v "$GOVULNCHECK_CMD" >/dev/null 2>&1 || die "govulncheck is required; install the pinned version from .agents/rules/version-pinning.md"

	mapfile -t modules < <(discover_go_modules)
	[[ "${#modules[@]}" -gt 0 ]] || die "no tracked Go modules found"

	for module_dir in "${modules[@]}"; do
		run_govulncheck "$module_dir"
	done
}

main "$@"
