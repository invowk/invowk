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

assert_file_exists() {
	local desc="$1"
	local file="$2"

	if [[ -f "$file" ]]; then
		record_pass
	else
		record_fail "$desc" "missing file: $file"
	fi
}

assert_file_missing() {
	local desc="$1"
	local file="$2"

	if [[ -e "$file" ]]; then
		record_fail "$desc" "unexpected file: $file"
	else
		record_pass
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

test_interrupt_status_detection() {
	if interrupted_status 143; then
		record_pass
	else
		record_fail "signal exit status is interrupted" "expected status 143 to be treated as interrupted"
	fi

	if interrupted_status 4; then
		record_fail "quality gate exit status is not interrupted" "status 4 should allow remaining modules to run"
	else
		record_pass
	fi
}

test_untracked_cleanup_preserves_existing_files() {
	local existing_path
	local new_path

	existing_path="internal/agentcmd/.mutation-existing-$$"
	new_path="internal/agentcmd/.mutation-generated-$$"
	rm -f -- "$REPO_ROOT/$existing_path" "$REPO_ROOT/$new_path"
	trap 'rm -f -- "$REPO_ROOT/$existing_path" "$REPO_ROOT/$new_path"; [[ -n "${MUTATION_CLEANUP_DIR:-}" ]] && rm -rf "$MUTATION_CLEANUP_DIR"' RETURN

	printf 'existing\n' >"$REPO_ROOT/$existing_path"
	snapshot_untracked_paths root
	printf 'generated\n' >"$REPO_ROOT/$new_path"
	remove_new_untracked_paths root

	if [[ -f "$REPO_ROOT/$existing_path" ]]; then
		record_pass
	else
		record_fail "untracked cleanup preserves pre-existing files" "$existing_path was removed"
	fi

	if [[ -e "$REPO_ROOT/$new_path" ]]; then
		record_fail "untracked cleanup removes generated files" "$new_path was left behind"
	else
		record_pass
	fi
}

test_tool_report_collection() {
	local tmp
	local workdir
	local report_dir
	local report

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	workdir="$tmp/work"
	report_dir="$tmp/reports"
	mkdir -p "$workdir" "$report_dir"

	for report in \
		report.json \
		go-mutesting-summary.json \
		go-mutesting-agentic.json \
		go-mutesting-gitlab.json \
		go-mutesting-report.html; do
		printf '%s\n' "$report" >"$workdir/$report"
	done

	collect_tool_reports "$workdir" "$report_dir"

	for report in \
		report.json \
		go-mutesting-summary.json \
		go-mutesting-agentic.json \
		go-mutesting-gitlab.json \
		go-mutesting-report.html; do
		assert_file_exists "collects $report" "$report_dir/$report"
		assert_file_missing "moves $report out of workdir" "$workdir/$report"
	done
}

test_tool_report_collection_before_untracked_cleanup() {
	local tmp
	local workdir
	local report_dir
	local report

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"; rm -f -- "$REPO_ROOT/tools/goplint/go-mutesting-summary.json"; [[ -n "${MUTATION_CLEANUP_DIR:-}" ]] && rm -rf "$MUTATION_CLEANUP_DIR"' RETURN
	workdir="$REPO_ROOT/tools/goplint"
	report_dir="$tmp/reports"
	report="$workdir/go-mutesting-summary.json"
	mkdir -p "$report_dir"

	snapshot_untracked_paths goplint
	printf '{"totalMutantsCount":1}\n' >"$report"
	restore_tracked_mutation_paths goplint
	collect_tool_reports "$workdir" "$report_dir"
	remove_new_untracked_paths goplint

	assert_file_exists "collects nested-module report before cleanup" "$report_dir/go-mutesting-summary.json"
	assert_file_missing "removes nested-module workdir report after collection" "$report"
	assert_file_contains "preserves nested-module report content" '"totalMutantsCount":1' "$report_dir/go-mutesting-summary.json"
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

	assert_contains "root targets include dependency package" "github.com/invowk/invowk/internal/app/deps" "$targets"
	assert_contains "root targets include config package" "github.com/invowk/invowk/internal/config" "$targets"
	assert_contains "root targets include public schema package" "github.com/invowk/invowk/pkg/invowkfile" "$targets"
	assert_not_contains "root curated seed omits CLI adapter package" "github.com/invowk/invowk/cmd/invowk" "$targets"
	assert_not_contains "root curated seed omits virtual runtime package" "github.com/invowk/invowk/internal/runtime" "$targets"
}

test_goplint_target_resolution() {
	local tmp
	local targets

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	targets="$(resolve_targets goplint "$tmp/goplint")"

	assert_contains "goplint targets include analyzer entrypoint" "goplint/analyzer.go" "$targets"
	assert_contains "goplint targets include constructor validation analyzer" "goplint/analyzer_constructor_validates.go" "$targets"
	assert_contains "goplint targets include Windows pitfalls analyzer" "goplint/analyzer_windows_pitfalls.go" "$targets"
	assert_not_contains "goplint targets avoid leading dot-slash ids" "./goplint/analyzer.go" "$targets"
	assert_not_contains "goplint curated profile omits command package" "github.com/invowk/invowk/tools/goplint" "$targets"
	assert_not_contains "goplint curated profile omits full analyzer package" "github.com/invowk/invowk/tools/goplint/goplint" "$targets"
	assert_file_contains "goplint file target metadata records owning package" \
		"file target in github.com/invowk/invowk/tools/goplint/goplint" \
		"$tmp/goplint/package-candidates.txt"
}

test_paths
test_command_construction
test_interrupt_status_detection
test_untracked_cleanup_preserves_existing_files
test_tool_report_collection
test_tool_report_collection_before_untracked_cleanup
test_dirty_path_policy
test_root_target_resolution
test_goplint_target_resolution

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
