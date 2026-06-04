#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0

record_pass() {
	PASS=$((PASS + 1))
}

record_fail() {
	local desc="$1"
	local detail="$2"

	FAIL=$((FAIL + 1))
	printf 'FAIL: %s\n  %s\n' "$desc" "$detail" >&2
}

assert_eq() {
	local desc="$1"
	local expected="$2"
	local actual="$3"

	if [[ "$expected" == "$actual" ]]; then
		record_pass
	else
		record_fail "$desc" "expected: $expected; actual: $actual"
	fi
}

assert_file_contains() {
	local desc="$1"
	local needle="$2"
	local file="$3"

	if grep -Fq -- "$needle" "$file"; then
		record_pass
	else
		record_fail "$desc" "missing text in $file: $needle"
	fi
}

assert_status() {
	local desc="$1"
	local expected="$2"
	local actual="$3"

	if [[ "$expected" -eq "$actual" ]]; then
		record_pass
	else
		record_fail "$desc" "expected status: $expected; actual status: $actual"
	fi
}

write_fake_tools() {
	local tmp="$1"
	local version="$2"
	local binary_path="$tmp/golangci-lint"
	local go_path="$tmp/go"

	cat >"$binary_path" <<'EOF'
#!/usr/bin/env bash
{
	printf 'pwd=%s\n' "$PWD"
	printf 'args='
	printf '%q ' "$@"
	printf '\n'
} >>"$FAKE_GOLANGCI_LOG"
if [[ "${1:-}" == "version" ]]; then
	printf 'golangci-lint has version 2.12.2\n'
fi
EOF
	chmod +x "$binary_path"

	cat >"$go_path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
case "\${1:-}" in
	tool)
		if [[ "\${2:-}" == "-n" && "\${3:-}" == "golangci-lint" ]]; then
			printf '%s\n' "$binary_path"
		else
			exit 2
		fi
		;;
	version)
		if [[ "\${2:-}" == "-m" ]]; then
			printf '%s: go1.26.4\n' "$binary_path"
			printf 'path github.com/golangci/golangci-lint/v2/cmd/golangci-lint\n'
			printf 'mod github.com/golangci/golangci-lint/v2 %s h1:fake\n' "$version"
		else
			exit 2
		fi
		;;
	*)
		exit 2
		;;
esac
EOF
	chmod +x "$go_path"
}

run_wrapper() {
	local tmp="$1"
	shift

	FAKE_GOLANGCI_LOG="$tmp/golangci.log" GO_CMD="$tmp/go" "$REPO_ROOT/scripts/golangci-lint.sh" "$@"
}

test_root_and_tools_dispatch() {
	local tmp

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	write_fake_tools "$tmp" "v2.12.2"
	run_wrapper "$tmp" root-run --show-stats=false
	run_wrapper "$tmp" tools-run --show-stats=false

	assert_file_contains "root run executes from repo root" "pwd=$REPO_ROOT" "$tmp/golangci.log"
	assert_file_contains "root run passes config and package pattern" "args=run --config=.golangci.toml --show-stats=false ./... " "$tmp/golangci.log"
	assert_file_contains "tools run executes from nested module" "pwd=$REPO_ROOT/tools/goplint" "$tmp/golangci.log"
}

test_formatter_and_config_dispatch() {
	local tmp

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	write_fake_tools "$tmp" "v2.12.2"
	run_wrapper "$tmp" root-fmt
	run_wrapper "$tmp" tools-config-verify

	assert_file_contains "formatter check uses diff mode" "args=fmt --config=.golangci.toml --diff " "$tmp/golangci.log"
	assert_file_contains "config verify uses config file" "args=config verify --config=.golangci.toml " "$tmp/golangci.log"
	assert_file_contains "tools config verify executes from nested module" "pwd=$REPO_ROOT/tools/goplint" "$tmp/golangci.log"
}

test_linter_inspection_and_version() {
	local tmp

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	write_fake_tools "$tmp" "v2.12.2"
	run_wrapper "$tmp" root-linters
	run_wrapper "$tmp" version

	assert_file_contains "linter inspection emits JSON" "args=linters --config=.golangci.toml --json " "$tmp/golangci.log"
	assert_file_contains "version command uses normalized binary" "args=version " "$tmp/golangci.log"
}

test_version_mismatch_fails() {
	local tmp
	local status

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	write_fake_tools "$tmp" "v0.0.0"
	set +e
	run_wrapper "$tmp" root-run >"$tmp/out" 2>"$tmp/err"
	status=$?
	set -e

	assert_status "version mismatch exits before linting" 1 "$status"
	assert_file_contains "version mismatch names expected version" \
		"expected github.com/golangci/golangci-lint/v2 v2.12.2, got v0.0.0" \
		"$tmp/err"
}

test_missing_tool_fails() {
	local tmp
	local status

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	cat >"$tmp/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "tool" && "${2:-}" == "-n" ]]; then
	printf '%s\n' "/does/not/exist/golangci-lint"
	exit 0
fi
exit 2
EOF
	chmod +x "$tmp/go"
	set +e
	run_wrapper "$tmp" root-run >"$tmp/out" 2>"$tmp/err"
	status=$?
	set -e

	assert_status "missing resolved tool exits before linting" 1 "$status"
	assert_file_contains "missing tool is actionable" \
		"resolved golangci-lint is not executable: /does/not/exist/golangci-lint" \
		"$tmp/err"
}

test_unknown_command_fails() {
	local tmp
	local status

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	write_fake_tools "$tmp" "v2.12.2"
	set +e
	run_wrapper "$tmp" nope >"$tmp/out" 2>"$tmp/err"
	status=$?
	set -e

	assert_status "unknown command exits" 1 "$status"
	assert_file_contains "unknown command reports name" "unknown golangci-lint command: nope" "$tmp/err"
}

test_root_and_tools_dispatch
test_formatter_and_config_dispatch
test_linter_inspection_and_version
test_version_mismatch_fails
test_missing_tool_fails
test_unknown_command_fails

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
