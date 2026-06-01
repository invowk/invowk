#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INVOWK_MUTATION_TESTING=1
export INVOWK_MUTATION_TESTING

# shellcheck source=mutation.sh
source "$SCRIPT_DIR/mutation.sh"

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

assert_contains() {
	local desc="$1"
	local needle="$2"
	local haystack="$3"

	if grep -Fqx -- "$needle" <<<"$haystack"; then
		record_pass
	else
		record_fail "$desc" "missing line: $needle"
	fi
}

assert_not_contains() {
	local desc="$1"
	local needle="$2"
	local haystack="$3"

	if grep -Fqx -- "$needle" <<<"$haystack"; then
		record_fail "$desc" "unexpected line: $needle"
	else
		record_pass
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

assert_path_allowed() {
	local desc="$1"
	local path="$2"

	if dirty_path_is_allowed "$path"; then
		record_pass
	else
		record_fail "$desc" "path was unexpectedly rejected: $path"
	fi
}

assert_path_rejected() {
	local desc="$1"
	local path="$2"

	if dirty_path_is_allowed "$path"; then
		record_fail "$desc" "path was unexpectedly allowed: $path"
	else
		record_pass
	fi
}

test_paths() {
	local tmp

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN

	assert_eq "root baseline path" \
		"$REPO_ROOT/tools/mutation/baselines/root-baseline.json" \
		"$(baseline_path root)"
	assert_eq "goplint baseline path" \
		"$REPO_ROOT/tools/mutation/baselines/goplint-baseline.json" \
		"$(baseline_path goplint)"
	assert_eq "profile report path" \
		"$tmp/reports/pr/root" \
		"$(profile_report_dir pr root "$tmp/reports")"
}

test_command_construction() {
	local args

	args="$(build_go_mutesting_args pr root advisory origin/main "")"
	assert_contains "pr enables changed-line mutation" "--git-diff-lines" "$args"
	assert_contains "pr pins diff base" "--git-diff-base=origin/main" "$args"
	assert_contains "pr enables GitHub annotations" "--logger-github" "$args"
	assert_contains "pr writes summary JSON" "--logger-summary-json" "$args"
	assert_contains "pr writes agentic escaped-mutant JSON" "--logger-agentic-json" "$args"
	assert_contains "pr ignores no-mutation score failures" "--ignore-msi-with-no-mutations" "$args"
	assert_contains "pr uses short Go tests" "--test-flags=-short" "$args"
	assert_not_contains "pr does not pass race" "-race" "$args"
	assert_not_contains "advisory pr does not fail on escaped mutants" "--fail-on-escaped" "$args"

	args="$(build_go_mutesting_args pr root blocking origin/main "")"
	assert_contains "blocking pr fails on escaped mutants" "--fail-on-escaped" "$args"

	args="$(build_go_mutesting_args dry-run root advisory origin/main "")"
	assert_contains "dry-run enables dry run" "--dry-run" "$args"
	assert_not_contains "dry-run does not pass race" "-race" "$args"

	args="$(build_go_mutesting_args rerun goplint advisory origin/main escaped-123)"
	assert_contains "rerun targets stable mutant id" "--run-mutant-id=escaped-123" "$args"
	assert_contains "rerun prints escaped status" "--output-statuses=e" "$args"
}

test_dirty_path_policy() {
	assert_path_allowed "allows root mutation baseline" "tools/mutation/baselines/root-baseline.json"
	assert_path_allowed "allows goplint mutation baseline" "tools/mutation/baselines/goplint-baseline.json"
	assert_path_allowed "allows generated mutation reports" "artifacts/mutation/pr/root/go-mutesting-summary.json"
	assert_path_rejected "rejects source changes" "cmd/invowk/root.go"
	assert_path_rejected "rejects docs changes" ".agents/rules/commands.md"
}

test_root_target_resolution() {
	local tmp
	local targets

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	targets="$(resolve_targets root "$tmp/root")"

	assert_contains "root targets include CLI package" "github.com/invowk/invowk/cmd/invowk" "$targets"
	assert_contains "root targets include runtime package" "github.com/invowk/invowk/internal/runtime" "$targets"
	assert_contains "root targets include public schema package" "github.com/invowk/invowk/pkg/invowkfile" "$targets"
	assert_file_contains "root exclusions explain test helpers" "github.com/invowk/invowk/internal/testutil" "$tmp/root/excluded-packages.txt"
	assert_file_contains "root exclusions expose no-local-test package" "github.com/invowk/invowk/internal/llm" "$tmp/root/excluded-packages.txt"
}

test_goplint_target_resolution() {
	local tmp
	local targets

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	targets="$(resolve_targets goplint "$tmp/goplint")"

	assert_contains "goplint targets include command package" "github.com/invowk/invowk/tools/goplint" "$targets"
	assert_contains "goplint targets include analyzer package" "github.com/invowk/invowk/tools/goplint/goplint" "$targets"
}

test_paths
test_command_construction
test_dirty_path_policy
test_root_target_resolution
test_goplint_target_resolution

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
