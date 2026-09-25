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

	args="$(build_go_mutesting_args rerun root advisory origin/main escaped-123)"
	assert_contains "rerun targets stable mutant id" "--run-mutant-id=escaped-123" "$args"
	assert_contains "rerun selects escaped status code" "--output-statuses=e" "$args"
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
	trap 'rm -rf "$tmp"; rm -f -- "$REPO_ROOT/go-mutesting-summary.json"; [[ -n "${MUTATION_CLEANUP_DIR:-}" ]] && rm -rf "$MUTATION_CLEANUP_DIR"' RETURN
	workdir="$REPO_ROOT"
	report_dir="$tmp/reports"
	report="$workdir/go-mutesting-summary.json"
	mkdir -p "$report_dir"

	snapshot_untracked_paths root
	printf '{"totalMutantsCount":1}\n' >"$report"
	restore_tracked_mutation_paths root
	collect_tool_reports "$workdir" "$report_dir"
	remove_new_untracked_paths root

	assert_file_exists "collects module report before cleanup" "$report_dir/go-mutesting-summary.json"
	assert_file_missing "removes module workdir report after collection" "$report"
	assert_file_contains "preserves module report content" '"totalMutantsCount":1' "$report_dir/go-mutesting-summary.json"
}

test_dirty_path_policy() {
	assert_path_allowed "allows root mutation baseline" "tools/mutation/baselines/root-baseline.json"
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

test_rapid_determinism_env() {
	local env_lines

	env_lines="$(unset RAPID_SEED RAPID_SHRINKTIME RAPID_NOFAILFILE; export_rapid_determinism_env; env | grep '^RAPID_' | sort)"
	assert_contains "rapid seed is fixed" "RAPID_SEED=$MUTATION_RAPID_SEED" "$env_lines"
	assert_contains "rapid failure files are disabled" "RAPID_NOFAILFILE=1" "$env_lines"
	assert_contains "rapid shrink time is bounded" "RAPID_SHRINKTIME=$MUTATION_RAPID_SHRINKTIME" "$env_lines"

	env_lines="$(RAPID_SEED=7 RAPID_NOFAILFILE=0; export RAPID_SEED RAPID_NOFAILFILE; export_rapid_determinism_env; env | grep '^RAPID_' | sort)"
	assert_contains "caller may override the seed" "RAPID_SEED=7" "$env_lines"
	assert_contains "failure files stay disabled" "RAPID_NOFAILFILE=1" "$env_lines"
}

# ---------------------------------------------------------------------------
# formal-bindings target set

FORMAL_TEST_TMP=""

# write_formal_fixture DIR writes a plan over DIR/a.go (Decide on lines 3-8,
# Stop on lines 10-12) and a stub go that logs its arguments and replays
# STUB_GO_MODE.
write_formal_fixture() {
	local dir="$1"

	mkdir -p "$dir/bin"
	cat >"$dir/a.go" <<'GO'
package a

func Decide(x int) bool {
	if x > 0 {
		return true
	}
	return false
}

func Stop() error {
	return nil
}
GO
	cat >"$dir/plan.json" <<JSON
{"version": 1, "root": "$dir", "files": ["a.go"],
 "groups": [{"match": "^(Decide|Stop)$", "files": ["a.go"]}],
 "functions": [
  {"file": "a.go", "symbol": "Decide", "name": "Decide", "start": 3, "end": 8,
   "rows": [{"model": "M", "element": "decide"}], "killers": ["TestCross", "TestDecide"],
   "packages": ["example.com/a", "example.com/b"]},
  {"file": "a.go", "symbol": "Stop", "name": "Stop", "start": 10, "end": 12,
   "rows": [{"model": "M", "element": "stop"}], "killers": ["TestStop"], "packages": ["example.com/a"]}
 ],
 "unbound": [], "preflight": {"a.go": {"clean_seconds": 1.0, "timeout_seconds": 11, "overridden": false}}}
JSON
	cat >"$dir/bin/go" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$STUB_GO_ARGS"
case "$STUB_GO_MODE" in
	pass) printf 'ok  \texample.com/a\t0.1s\n'; exit 0 ;;
	fail) printf -- '--- FAIL: TestDecide (0.00s)\nFAIL\nFAIL\texample.com/a\t0.1s\n'; exit 1 ;;
	panic) printf 'panic: nil map\nFAIL\texample.com/a\t0.1s\n'; exit 1 ;;
	timeout) printf 'panic: test timed out after 11s\nFAIL\texample.com/a\t11.0s\n'; exit 1 ;;
	build) printf '# example.com/a\n./a.go:4:2: undefined: y\nFAIL\texample.com/a [build failed]\n'; exit 1 ;;
	setup) printf 'FAIL\texample.com/a [setup failed]\n'; exit 1 ;;
	weird) printf 'go: something else\n'; exit 2 ;;
	json-pass) printf '{"Action":"pass","Test":"%s"}\n' TestCross TestDecide TestStop; exit 0 ;;
	json-skip) printf '{"Action":"pass","Test":"%s"}\n' TestCross TestDecide; printf '{"Action":"skip","Test":"TestStop"}\n'; exit 0 ;;
esac
exit 99
STUB
	chmod +x "$dir/bin/go"
	# A mutant of Decide (line 4) and one of the line between the two functions.
	sed '4s/x > 0/x >= 0/' "$dir/a.go" >"$dir/mutant_decide.go"
	sed '9s/^$/var _ = 1/' "$dir/a.go" >"$dir/mutant_outside.go"
}

# run_formal_exec MODE CHANGED prints the exec script's exit code.
run_formal_exec() {
	local mode="$1"
	local changed="$2"
	local status

	set +e
	STUB_GO_MODE="$mode" STUB_GO_ARGS="$FORMAL_TEST_TMP/go-args" GO_CMD="$FORMAL_TEST_TMP/bin/go" \
		MUTATION_FORMAL_PLAN="$FORMAL_TEST_TMP/plan.json" MUTATE_ORIGINAL="$FORMAL_TEST_TMP/a.go" \
		MUTATE_CHANGED="$changed" RAPID_SEED=1 RAPID_NOFAILFILE=1 RAPID_SHRINKTIME=1s \
		"$SCRIPT_DIR/mutation-formal-exec.sh" 2>"$FORMAL_TEST_TMP/exec-stderr"
	status=$?
	set -e
	printf '%s\n' "$status"
}

test_formal_exec_result_mapping() {
	local tracked_before
	local mode
	local expected
	local args

	FORMAL_TEST_TMP="$(mktemp -d)"
	trap 'rm -rf "$FORMAL_TEST_TMP"' RETURN
	write_formal_fixture "$FORMAL_TEST_TMP"
	tracked_before="$(git -C "$REPO_ROOT" status --porcelain --untracked-files=no)"

	for mode in pass:1 fail:0 panic:0 timeout:0 build:2 setup:2 weird:3; do
		expected="${mode#*:}"
		mode="${mode%%:*}"
		assert_eq "exec maps go test $mode to $expected" "$expected" "$(run_formal_exec "$mode" "$FORMAL_TEST_TMP/mutant_decide.go")"
	done
	run_formal_exec timeout "$FORMAL_TEST_TMP/mutant_decide.go" >/dev/null
	assert_file_contains "timeout kills are logged per file" "formal-mutation: timeout-kill file=a.go function=Decide" "$FORMAL_TEST_TMP/exec-stderr"

	run_formal_exec pass "$FORMAL_TEST_TMP/mutant_decide.go" >/dev/null
	args="$(cat "$FORMAL_TEST_TMP/go-args")"
	assert_contains "exec disables the test cache" "-count=1" "$args"
	assert_contains "exec runs only the mutated function's killers" "^(TestCross|TestDecide)$" "$args"
	assert_contains "exec uses the file's pre-flight timeout" "11s" "$args"
	assert_contains "exec runs killers in every owning package" "example.com/b" "$args"
	assert_not_contains "exec does not run the other function's killers" "^(TestCross|TestDecide|TestStop)$" "$args"
	assert_not_contains "exec never passes -short" "-short" "$args"
	assert_not_contains "exec never passes -race" "-race" "$args"
	if grep -q '^-overlay=' <<<"$args"; then
		record_pass
	else
		record_fail "exec applies the mutant through an overlay" "no -overlay argument in: $args"
	fi

	assert_eq "exec errors on a line outside every planned function" "3" "$(run_formal_exec pass "$FORMAL_TEST_TMP/mutant_outside.go")"
	cp "$FORMAL_TEST_TMP/a.go" "$FORMAL_TEST_TMP/unplanned.go"
	set +e
	STUB_GO_MODE=pass STUB_GO_ARGS="$FORMAL_TEST_TMP/go-args" GO_CMD="$FORMAL_TEST_TMP/bin/go" \
		MUTATION_FORMAL_PLAN="$FORMAL_TEST_TMP/plan.json" MUTATE_ORIGINAL="$FORMAL_TEST_TMP/unplanned.go" \
		MUTATE_CHANGED="$FORMAL_TEST_TMP/unplanned.go" RAPID_SEED=1 RAPID_NOFAILFILE=1 RAPID_SHRINKTIME=1s \
		"$SCRIPT_DIR/mutation-formal-exec.sh" 2>/dev/null
	assert_eq "exec errors on an unplanned file" "3" "$?"
	set -e

	assert_eq "exec leaves tracked files untouched" "$tracked_before" "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=no)"
	assert_file_missing "exec removes its temporary overlay" "$(sed -n 's/^-overlay=//p' "$FORMAL_TEST_TMP/go-args")"
}

test_formal_exec_requires_determinism() {
	local variable
	local status

	FORMAL_TEST_TMP="$(mktemp -d)"
	trap 'rm -rf "$FORMAL_TEST_TMP"' RETURN
	write_formal_fixture "$FORMAL_TEST_TMP"
	local -a settings=()

	for variable in MUTATION_FORMAL_PLAN RAPID_SEED RAPID_NOFAILFILE RAPID_SHRINKTIME; do
		rm -f "$FORMAL_TEST_TMP/go-args"
		settings=(
			"MUTATION_FORMAL_PLAN=$FORMAL_TEST_TMP/plan.json" RAPID_SEED=1 RAPID_NOFAILFILE=1 RAPID_SHRINKTIME=1s
		)
		mapfile -t settings < <(printf '%s\n' "${settings[@]}" | grep -v "^$variable=")
		set +e
		env -u MUTATION_FORMAL_PLAN -u RAPID_SEED -u RAPID_NOFAILFILE -u RAPID_SHRINKTIME \
			STUB_GO_MODE=pass STUB_GO_ARGS="$FORMAL_TEST_TMP/go-args" GO_CMD="$FORMAL_TEST_TMP/bin/go" \
			MUTATE_ORIGINAL="$FORMAL_TEST_TMP/a.go" MUTATE_CHANGED="$FORMAL_TEST_TMP/mutant_decide.go" \
			"${settings[@]}" "$SCRIPT_DIR/mutation-formal-exec.sh" 2>/dev/null
		status=$?
		set -e
		assert_eq "exec refuses to run without $variable" "3" "$status"
		assert_file_missing "exec runs no tests without $variable" "$FORMAL_TEST_TMP/go-args"
	done
	set +e
	env STUB_GO_MODE=pass STUB_GO_ARGS="$FORMAL_TEST_TMP/go-args" GO_CMD="$FORMAL_TEST_TMP/bin/go" \
		MUTATION_FORMAL_PLAN="$FORMAL_TEST_TMP/plan.json" MUTATE_ORIGINAL="$FORMAL_TEST_TMP/a.go" \
		MUTATE_CHANGED="$FORMAL_TEST_TMP/mutant_decide.go" RAPID_SEED=1 RAPID_NOFAILFILE=0 RAPID_SHRINKTIME=1s \
		"$SCRIPT_DIR/mutation-formal-exec.sh" 2>/dev/null
	status=$?
	set -e
	assert_eq "exec refuses RAPID_NOFAILFILE other than 1" "3" "$status"
}

# formal_preflight_with_stub MODE runs the pre-flight over the fixture with the
# stub go replaying MODE.
formal_preflight_with_stub() {
	local mode="$1"

	STUB_GO_MODE="$mode" STUB_GO_ARGS="$FORMAL_TEST_TMP/go-args" GO_CMD="$FORMAL_TEST_TMP/bin/go" \
		REPO_ROOT="$FORMAL_TEST_TMP" RAPID_SEED=1 RAPID_NOFAILFILE=1 RAPID_SHRINKTIME=1s \
		run_formal_preflight "$FORMAL_TEST_TMP/plan.json" "$FORMAL_TEST_TMP/report"
}

test_formal_preflight() {
	local status
	local mode

	FORMAL_TEST_TMP="$(mktemp -d)"
	trap 'rm -rf "$FORMAL_TEST_TMP"' RETURN
	write_formal_fixture "$FORMAL_TEST_TMP"

	set +e
	(formal_preflight_with_stub json-pass) >/dev/null 2>&1
	status=$?
	set -e
	assert_eq "pre-flight passes when every killer passes" "0" "$status"
	assert_contains "pre-flight runs the file's whole killer set" "^(TestCross|TestDecide|TestStop)$" "$(cat "$FORMAL_TEST_TMP/go-args")"
	assert_contains "pre-flight asks go test for JSON" "-json" "$(cat "$FORMAL_TEST_TMP/go-args")"
	assert_eq "pre-flight records the derived timeout" "10" \
		"$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["preflight"]["a.go"]["timeout_seconds"])' "$FORMAL_TEST_TMP/plan.json")"

	for mode in json-skip fail timeout; do
		set +e
		(formal_preflight_with_stub "$mode") >/dev/null 2>&1
		status=$?
		set -e
		assert_eq "pre-flight fails the run on $mode" "1" "$status"
	done
}

test_formal_command_construction() {
	local args
	local baseline="$REPO_ROOT/tools/mutation/baselines/formal-bindings-baseline.json"

	assert_eq "formal baseline path" "$baseline" "$(baseline_path root formal-bindings)"
	assert_eq "formal report path" "/tmp/r/full/formal-bindings" "$(profile_report_dir full root /tmp/r formal-bindings)"

	args="$(build_formal_mutation_args full advisory "" '^(A|B)$' 42 /tmp/g/baseline.json)"
	assert_contains "formal full passes the group match" '--match=^(A|B)$' "$args"
	assert_contains "formal full uses the formal exec" "--exec=$SCRIPT_DIR/mutation-formal-exec.sh" "$args"
	assert_contains "formal full passes the max per-file timeout" "--exec-timeout=42" "$args"
	assert_contains "formal full uses the formal baseline" "--baseline=$baseline" "$args"
	assert_contains "formal full writes agentic JSON" "--logger-agentic-json" "$args"
	assert_not_contains "formal full skips coverage" "--coverage" "$args"
	assert_not_contains "formal full skips per-test" "--per-test" "$args"
	assert_not_contains "formal full passes no test flags" "--test-flags=-short" "$args"
	assert_not_contains "formal full uses no timeout coefficient" "--timeout-coefficient=3" "$args"
	assert_not_contains "advisory formal full does not fail on escapes" "--fail-on-escaped" "$args"
	args="$(build_formal_mutation_args full blocking "" '^(A)$' 10 /tmp/g/baseline.json)"
	assert_contains "blocking formal full fails on escapes" "--fail-on-escaped" "$args"

	args="$(build_formal_mutation_args baseline-update advisory "" '^(A)$' 10 /tmp/g/baseline.json)"
	assert_contains "formal baseline-update writes a group baseline" "--baseline=/tmp/g/baseline.json" "$args"
	assert_contains "formal baseline-update updates" "--update-baseline" "$args"

	args="$(build_formal_mutation_args rerun advisory abc '^(A)$' 10 /tmp/g/baseline.json)"
	assert_contains "formal rerun targets the mutant" "--run-mutant-id=abc" "$args"

	args="$(build_formal_mutation_args dry-run advisory "" '^(A)$' 0 /tmp/g/baseline.json)"
	assert_contains "formal dry-run is a dry run" "--dry-run" "$args"
	assert_not_contains "formal dry-run runs no exec" "--exec=$SCRIPT_DIR/mutation-formal-exec.sh" "$args"
}

test_formal_target_set_rejections() {
	local output
	local status

	set +e
	output="$( (main pr --target-set formal-bindings) 2>&1)"
	status=$?
	set -e
	assert_eq "pr rejects the formal-bindings target set" "1" "$status"
	if grep -Fq "not available on the pr profile" <<<"$output"; then record_pass; else record_fail "pr rejection is actionable" "$output"; fi

	set +e
	output="$( (main full --target-set nope) 2>&1)"
	status=$?
	set -e
	assert_eq "unknown target sets are rejected" "1" "$status"
}

test_formal_rerun_evidence() {
	local tmp

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	printf '{"killedCount":0,"escapedCount":1,"errorCount":0,"skippedCount":0}\n' >"$tmp/summary.json"
	printf 'A' >"$tmp/a.go"
	printf '{"version": 1, "root": "%s", "inputs": ["a.go"]}\n' "$tmp" >"$tmp/plan.json"
	formal_mutation rerun-record --plan "$tmp/plan.json" --summary "$tmp/summary.json" --id abc --reruns "$tmp/reruns.jsonl" >/dev/null
	formal_mutation rerun-record --plan "$tmp/plan.json" --summary "$tmp/summary.json" --id abc --reruns "$tmp/reruns.jsonl" >/dev/null
	assert_eq "rerun evidence appends one record per rerun" "2" "$(wc -l <"$tmp/reruns.jsonl" | tr -d ' ')"
	assert_file_contains "rerun evidence records the status" '"status": "escaped"' "$tmp/reruns.jsonl"
	assert_file_contains "rerun evidence records the input digest" \
		"\"digest\": \"$(printf 'a.go\0A' | sha256sum | cut -d' ' -f1)\"" "$tmp/reruns.jsonl"
}

test_formal_dirty_path_policy() {
	assert_path_allowed "allows the formal baseline" "tools/mutation/baselines/formal-bindings-baseline.json"
	assert_path_allowed "allows the formal triage ledger" "tools/mutation/triage/formal-bindings.toml"
	assert_path_allowed "allows the formal rerun evidence" "tools/mutation/triage/formal-bindings-reruns.jsonl"
	assert_path_rejected "rejects other triage files" "tools/mutation/triage/current-full-scan.md"
}


test_paths
test_command_construction
test_interrupt_status_detection
test_untracked_cleanup_preserves_existing_files
test_tool_report_collection
test_tool_report_collection_before_untracked_cleanup
test_dirty_path_policy
test_root_target_resolution
test_rapid_determinism_env
test_formal_exec_result_mapping
test_formal_exec_requires_determinism
test_formal_preflight
test_formal_command_construction
test_formal_target_set_rejections
test_formal_rerun_evidence
test_formal_dirty_path_policy

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
