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

write_fake_govulncheck() {
	local path="$1"
	local log="$2"
	local fail_module="${3:-}"

	cat >"$path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s|%s\n' "\$PWD" "\$*" >>"$log"
if [[ -n "$fail_module" && "\$PWD" == *"$fail_module" ]]; then
	exit 7
fi
printf 'fake govulncheck ok\n'
EOF
	chmod +x "$path"
}

test_scans_tracked_modules() {
	local tmp
	local fake
	local log
	local output
	local status

	tmp="$(mktemp -d)"
	fake="$tmp/govulncheck"
	log="$tmp/log"
	output="$tmp/output"
	write_fake_govulncheck "$fake" "$log"

	set +e
	(
		cd "$REPO_ROOT"
		GOVULNCHECK_CMD="$fake" ./scripts/govulncheck-all.sh
	) >"$output" 2>&1
	status=$?
	set -e

	assert_status "scans tracked modules successfully" 0 "$status"
	assert_file_contains "logs root module scan" "==> govulncheck: ." "$output"
	assert_file_contains "logs tools/goplint scan" "==> govulncheck: tools/goplint" "$output"
	assert_file_contains "runs fake in root module" "$REPO_ROOT|./..." "$log"
	assert_file_contains "runs fake in tools/goplint module" "$REPO_ROOT/tools/goplint|./..." "$log"
	rm -rf "$tmp"
}

test_failure_identifies_module() {
	local tmp
	local fake
	local log
	local output
	local status

	tmp="$(mktemp -d)"
	fake="$tmp/govulncheck"
	log="$tmp/log"
	output="$tmp/output"
	write_fake_govulncheck "$fake" "$log" "tools/goplint"

	set +e
	(
		cd "$REPO_ROOT"
		GOVULNCHECK_CMD="$fake" ./scripts/govulncheck-all.sh
	) >"$output" 2>&1
	status=$?
	set -e

	assert_status "propagates govulncheck failure" 7 "$status"
	assert_file_contains "logs failing module before error" "==> govulncheck: tools/goplint" "$output"
	assert_file_contains "reports failing module" "govulncheck failed in module: tools/goplint" "$output"
	rm -rf "$tmp"
}

test_scans_tracked_modules
test_failure_identifies_module

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
