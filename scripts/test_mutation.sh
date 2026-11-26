#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INVOWK_MUTATION_TESTING=1
export INVOWK_MUTATION_TESTING
# Fixture repositories must never resolve to this checkout through an
# inherited git environment (for example when run from a git hook).
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

# shellcheck source=mutation.sh
source "$SCRIPT_DIR/mutation.sh"

PASS=0
FAIL=0

# sha256_stdin prints the SHA-256 of stdin (macOS has shasum, not sha256sum).
sha256_stdin() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum
	else
		shasum -a 256
	fi | cut -d' ' -f1
}

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

# The restore helpers revert tracked sources and delete files, so every test
# that exercises them runs against a throwaway repository (make_fixture_repo)
# through a function-local REPO_ROOT, never against this checkout.
# make_fixture_repo DIR creates a git repository with one committed file under
# each mutation source path (cmd, internal, pkg).
make_fixture_repo() {
	local dir="$1"

	mkdir -p "$dir/cmd/app" "$dir/internal/agentcmd" "$dir/pkg/lib"
	printf 'package main\n' >"$dir/cmd/app/main.go"
	printf 'package agentcmd\n' >"$dir/internal/agentcmd/agentcmd.go"
	printf 'package lib\n' >"$dir/pkg/lib/lib.go"
	git -C "$dir" init -q
	git -C "$dir" config user.email test@example.com
	git -C "$dir" config user.name Test
	git -C "$dir" config commit.gpgsign false
	git -C "$dir" config core.hooksPath /dev/null
	git -C "$dir" add -A
	git -C "$dir" commit -qm init
}

# make_dirty_fixture_repo DIR adds uncommitted work under every source path:
# an unstaged edit, a staged edit, and an untracked file.
make_dirty_fixture_repo() {
	local dir="$1"

	make_fixture_repo "$dir"
	printf '// unstaged work\n' >>"$dir/cmd/app/main.go"
	printf '// staged work\n' >>"$dir/pkg/lib/lib.go"
	git -C "$dir" add pkg/lib/lib.go
	printf 'untracked work\n' >"$dir/internal/agentcmd/notes.txt"
}

# repo_state DIR prints everything a destructive restore could change.
repo_state() {
	local dir="$1"

	git -C "$dir" diff --binary
	git -C "$dir" diff --cached --binary
	git -C "$dir" status --porcelain --untracked-files=all
}

test_untracked_cleanup_preserves_existing_files() {
	local tmp
	local REPO_ROOT
	local MUTATION_CLEANUP_DIR
	local MUTATION_RESTORE_ARMED_ROOT=""
	local existing_path="internal/agentcmd/.mutation-existing"
	local new_path="internal/agentcmd/.mutation-generated"

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	REPO_ROOT="$tmp/repo"
	MUTATION_CLEANUP_DIR="$tmp/cleanup"
	mkdir -p "$MUTATION_CLEANUP_DIR"
	make_fixture_repo "$REPO_ROOT"
	ensure_clean_tracked_worktree_for_mutation

	printf 'existing\n' >"$REPO_ROOT/$existing_path"
	snapshot_untracked_paths root
	printf 'generated\n' >"$REPO_ROOT/$new_path"
	remove_new_untracked_paths root

	assert_file_exists "untracked cleanup preserves pre-existing files" "$REPO_ROOT/$existing_path"
	assert_file_missing "untracked cleanup removes generated files" "$REPO_ROOT/$new_path"
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
	local REPO_ROOT
	local MUTATION_CLEANUP_DIR
	local MUTATION_RESTORE_ARMED_ROOT=""
	local report_dir
	local report

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	REPO_ROOT="$tmp/repo"
	MUTATION_CLEANUP_DIR="$tmp/cleanup"
	report_dir="$tmp/reports"
	report="$REPO_ROOT/go-mutesting-summary.json"
	mkdir -p "$MUTATION_CLEANUP_DIR" "$report_dir"
	make_fixture_repo "$REPO_ROOT"
	ensure_clean_tracked_worktree_for_mutation
	assert_eq "a clean checkout arms the restore" "$(cd "$REPO_ROOT" && pwd -P)" "$MUTATION_RESTORE_ARMED_ROOT"

	snapshot_untracked_paths root
	# What a mutation run leaves behind: a mutated tracked source, a new
	# untracked file under a source path, and the tool's root-level report.
	printf '// mutant\n' >>"$REPO_ROOT/internal/agentcmd/agentcmd.go"
	printf 'generated\n' >"$REPO_ROOT/pkg/lib/.mutation-generated"
	printf 'outside the source paths\n' >"$REPO_ROOT/notes.txt"
	printf '{"totalMutantsCount":1}\n' >"$report"
	restore_tracked_mutation_paths root
	collect_tool_reports "$REPO_ROOT" "$report_dir"
	remove_new_untracked_paths root

	assert_file_exists "collects module report before cleanup" "$report_dir/go-mutesting-summary.json"
	assert_file_missing "removes module workdir report after collection" "$report"
	assert_file_contains "preserves module report content" '"totalMutantsCount":1' "$report_dir/go-mutesting-summary.json"
	assert_eq "restore reverts the mutated tracked source" "package agentcmd" "$(cat "$REPO_ROOT/internal/agentcmd/agentcmd.go")"
	assert_file_missing "untracked cleanup removes files new under a source path" "$REPO_ROOT/pkg/lib/.mutation-generated"
	assert_eq "restore and cleanup leave only untracked files outside the source paths" \
		"?? notes.txt" "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=all)"
}

# run_guard_probe REPO SNIPPET runs SNIPPET in a fresh bash that sourced
# mutation.sh with REPO_ROOT=REPO, so a hard failure (exit) cannot end this
# test run. It prints the exit status; stderr goes to $GUARD_PROBE_STDERR. The
# probe's temporary directories (mutation.sh's cleanup dir, which a refused
# cleanup leaves behind) go under $GUARD_PROBE_TMPDIR, inside the caller's
# temporary directory, so no run leaks one into the system TMPDIR.
run_guard_probe() {
	local repo="$1"
	local snippet="$2"
	local status

	mkdir -p "$GUARD_PROBE_TMPDIR"
	set +e
	TMPDIR="$GUARD_PROBE_TMPDIR" INVOWK_MUTATION_TESTING=1 bash -c '
		set -euo pipefail
		source "$1"
		REPO_ROOT="$2"
		eval "$3"
	' guard-probe "$SCRIPT_DIR/mutation.sh" "$repo" "$snippet" 2>"$GUARD_PROBE_STDERR"
	status=$?
	set -e
	printf '%s\n' "$status"
}

test_restore_guard_refuses_unarmed_checkouts() {
	local tmp
	local dirty
	local clean
	local before
	local GUARD_PROBE_STDERR
	local GUARD_PROBE_TMPDIR

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	GUARD_PROBE_STDERR="$tmp/stderr"
	GUARD_PROBE_TMPDIR="$tmp/probe-tmp"
	dirty="$tmp/dirty"
	clean="$tmp/clean"
	make_dirty_fixture_repo "$dirty"
	make_fixture_repo "$clean"
	before="$(repo_state "$dirty")"
	# Stands in for the system TMPDIR: a refused cleanup must not leak its
	# cleanup directory into it.
	local -x TMPDIR="$tmp/outer-tmp"
	mkdir -p "$TMPDIR"

	assert_eq "unarmed tracked restore fails hard" "1" \
		"$(run_guard_probe "$dirty" 'restore_tracked_mutation_paths root; echo reached >&2')"
	assert_file_contains "unarmed tracked restore says why" "refusing to restore tracked root sources" "$GUARD_PROBE_STDERR"
	if grep -Fq reached "$GUARD_PROBE_STDERR"; then
		record_fail "unarmed tracked restore stops the caller" "execution continued after the refusal"
	else
		record_pass
	fi

	# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
	assert_eq "unarmed untracked cleanup fails hard" "1" \
		"$(run_guard_probe "$dirty" 'snapshot_untracked_paths root; printf new >"$REPO_ROOT/cmd/app/new.go"; remove_new_untracked_paths root')"
	assert_file_contains "unarmed untracked cleanup says why" "refusing to remove new untracked root files" "$GUARD_PROBE_STDERR"
	assert_file_exists "unarmed untracked cleanup deletes nothing" "$dirty/cmd/app/new.go"
	assert_eq "a refused cleanup leaks no temporary directory" "" "$(ls -A "$TMPDIR")"
	rm -f "$dirty/cmd/app/new.go"

	assert_eq "a dirty checkout refuses to arm" "1" \
		"$(run_guard_probe "$dirty" 'ensure_clean_tracked_worktree_for_mutation')"
	assert_eq "the environment cannot pre-arm the restore" "1" \
		"$(MUTATION_RESTORE_ARMED_ROOT="$(cd "$dirty" && pwd -P)" run_guard_probe "$dirty" 'restore_tracked_mutation_paths root')"
	assert_eq "arming one checkout does not arm another" "1" \
		"$(run_guard_probe "$clean" "ensure_clean_tracked_worktree_for_mutation; REPO_ROOT='$dirty'; restore_tracked_mutation_paths root")"
	assert_eq "unarmed calls change nothing in a dirty checkout" "$before" "$(repo_state "$dirty")"

	ln -s "$clean" "$tmp/clean-link"
	# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
	assert_eq "the armed root is compared after resolving symlinks" "0" \
		"$(run_guard_probe "$tmp/clean-link" 'ensure_clean_tracked_worktree_for_mutation; printf "// mutant\n" >>"$REPO_ROOT/cmd/app/main.go"; restore_tracked_mutation_paths root')"
	assert_eq "an armed restore reverts the mutated source" "package main" "$(cat "$clean/cmd/app/main.go")"
}

test_cleanup_trap_preserves_exit_status() {
	local tmp
	local dirty
	local clean
	local before
	local GUARD_PROBE_STDERR
	local GUARD_PROBE_TMPDIR
	# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
	local trap_setup='MUTATION_RESTORE_MODULES=(root); MUTATION_CLEANUP_DIR="$(mktemp -d)"; trap cleanup_mutation_paths EXIT'

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	GUARD_PROBE_STDERR="$tmp/stderr"
	GUARD_PROBE_TMPDIR="$tmp/probe-tmp"
	dirty="$tmp/dirty"
	clean="$tmp/clean"
	make_dirty_fixture_repo "$dirty"
	make_fixture_repo "$clean"
	before="$(repo_state "$dirty")"

	assert_eq "an unarmed trap keeps the exit status" "7" \
		"$(run_guard_probe "$dirty" "$trap_setup; exit 7")"
	assert_file_contains "an unarmed trap logs a loud no-op" "mutation cleanup did NOT restore tracked root sources" "$GUARD_PROBE_STDERR"
	assert_eq "an unarmed trap changes nothing" "$before" "$(repo_state "$dirty")"
	assert_eq "an unarmed trap keeps a zero exit status" "0" \
		"$(run_guard_probe "$dirty" "$trap_setup; exit 0")"

	# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
	assert_eq "an armed trap keeps the exit status" "7" \
		"$(run_guard_probe "$clean" 'ensure_clean_tracked_worktree_for_mutation; '"$trap_setup"'; printf "// mutant\n" >>"$REPO_ROOT/pkg/lib/lib.go"; exit 7')"
	assert_eq "an armed trap restores the mutated source" "package lib" "$(cat "$clean/pkg/lib/lib.go")"
	assert_eq "the cleanup trap removes its temporary directory" "" "$(ls -A "$GUARD_PROBE_TMPDIR")"
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
{"version": 2, "root": "$dir", "files": ["a.go"],
 "groups": [{"match": "^(Decide|Stop)$", "files": ["a.go"]}],
 "functions": [
  {"file": "a.go", "symbol": "Decide", "name": "Decide", "start": 3, "end": 8,
   "rows": [{"model": "M", "element": "decide", "killers": ["TestCross", "TestDecide"]}],
   "killers": ["TestCross", "TestDecide"], "packages": ["example.com/a", "example.com/b"], "package": "example.com/a"},
  {"file": "a.go", "symbol": "Stop", "name": "Stop", "start": 10, "end": 12,
   "rows": [{"model": "M", "element": "stop", "killers": ["TestStop"]}], "killers": ["TestStop"],
   "packages": ["example.com/a"], "package": "example.com/a"}
 ],
 "killer_packages": {"TestCross": "example.com/b", "TestDecide": "example.com/a", "TestStop": "example.com/a"},
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
	printf '{"version": 2, "root": "%s", "inputs": ["a.go"]}\n' "$tmp" >"$tmp/plan.json"
	formal_mutation rerun-record --plan "$tmp/plan.json" --summary "$tmp/summary.json" --id abc --reruns "$tmp/reruns.jsonl" >/dev/null
	formal_mutation rerun-record --plan "$tmp/plan.json" --summary "$tmp/summary.json" --id abc --reruns "$tmp/reruns.jsonl" >/dev/null
	assert_eq "rerun evidence appends one record per rerun" "2" "$(wc -l <"$tmp/reruns.jsonl" | tr -d ' ')"
	assert_file_contains "rerun evidence records the status" '"status": "escaped"' "$tmp/reruns.jsonl"
	assert_file_contains "rerun evidence records the input digest" \
		"\"digest\": \"$(printf 'a.go\0A' | sha256_stdin)\"" "$tmp/reruns.jsonl"
}

# test_formal_group_status drives the run_formal_profile loop (dry-run, so no
# clean-tree check, pre-flight or exit restore) over a three-group fixture plan
# with stubbed planning and group runs, in a temporary REPO_ROOT, and checks
# that every group records its exit status (mutation-partial-refresh design D2).
test_formal_group_status() {
	local tmp
	local metadata

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	cat >"$tmp/plan.json" <<'JSON'
{"version": 2, "root": "/nowhere", "files": ["a.go", "b.go", "c.go"],
 "groups": [{"match": "^(A)$", "files": ["a.go"]}, {"match": "^(B)$", "files": ["b.go"]}, {"match": "^(C)$", "files": ["c.go"]}],
 "functions": [], "killer_packages": {}, "unbound": [], "preflight": {}}
JSON
	(
		REPO_ROOT="$tmp/repo"
		mkdir -p "$REPO_ROOT"
		unset GITHUB_STEP_SUMMARY
		resolve_formal_targets() {
			mkdir -p "$1"
			cp "$tmp/plan.json" "$1/$FORMAL_PLAN_FILE"
			printf 'a.go\nb.go\nc.go\n' | tee "$1/resolved-targets.txt"
		}
		# Group 0 reports escapes, group 1 fails, group 2 passes.
		run_formal_group() {
			case "$1" in
				*/group-0) return 4 ;;
				*/group-1) return 3 ;;
			esac
			return 0
		}
		run_formal_profile dry-run advisory "$tmp/reports"
	) >/dev/null 2>&1 || true
	metadata="$(cat "$tmp/reports/dry-run/formal-bindings/run-metadata.txt" 2>/dev/null || true)"
	assert_contains "a group that reports escapes records status 4" "group_status[0]=4" "$metadata"
	assert_contains "a failing group records its status" "group_status[1]=3" "$metadata"
	assert_contains "a passing group records status 0" "group_status[2]=0" "$metadata"
	assert_eq "one status line per group" "3" "$(grep -c '^group_status\[' <<<"$metadata" || true)"
}

test_formal_dirty_path_policy() {
	assert_path_allowed "allows the formal baseline" "tools/mutation/baselines/formal-bindings-baseline.json"
	assert_path_allowed "allows the formal triage ledger" "tools/mutation/triage/formal-bindings.toml"
	assert_path_allowed "allows the formal rerun evidence" "tools/mutation/triage/formal-bindings-reruns.jsonl"
	assert_path_rejected "rejects other triage files" "tools/mutation/triage/current-full-scan.md"
}

test_formal_scope_and_jobs_validation() {
	local output
	local status

	set +e
	output="$( (main full --target-set root --symbol Foo) 2>&1)"
	status=$?
	set -e
	assert_eq "root target set rejects --symbol" "1" "$status"
	if grep -Fq "supported only on dry-run and full with --target-set formal-bindings" <<<"$output"; then
		record_pass
	else
		record_fail "root --symbol error is actionable" "$output"
	fi

	set +e
	output="$( (main baseline-update --target-set formal-bindings --stale) 2>&1)"
	status=$?
	set -e
	assert_eq "baseline-update rejects --stale" "1" "$status"

	set +e
	output="$( (main dry-run --target-set formal-bindings --jobs 0) 2>&1)"
	status=$?
	set -e
	assert_eq "jobs=0 is rejected" "1" "$status"
	if grep -Fq "positive integer" <<<"$output"; then
		record_pass
	else
		record_fail "jobs=0 error mentions positive integer" "$output"
	fi

	set +e
	output="$( (main full --target-set root --jobs 2) 2>&1)"
	status=$?
	set -e
	assert_eq "root target set rejects --jobs" "1" "$status"
}

test_formal_parallel_shards_and_workers_with_stubs() {
	local tmp
	local wt_count
	local metadata

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	mkdir -p "$tmp/repo" "$tmp/bin"
	git -C "$tmp/repo" init -q
	git -C "$tmp/repo" config user.name "Test"
	git -C "$tmp/repo" config user.email "test@example.com"
	git -C "$tmp/repo" config commit.gpgsign false

	write_formal_fixture "$tmp/repo"
	sed 's/func Decide/func Validate/; s/func Stop/func Unused/' "$tmp/repo/a.go" >"$tmp/repo/b.go"
	cat >"$tmp/repo/plan.json" <<JSON
{"version": 2, "root": "$tmp/repo", "files": ["a.go", "b.go"], "inputs": ["a.go", "b.go"],
 "groups": [{"match": "^(Decide|Stop|Validate)$", "files": ["a.go", "b.go"]}],
 "functions": [
  {"file": "a.go", "symbol": "Decide", "name": "Decide", "start": 3, "end": 8,
   "rows": [{"model": "M", "element": "decide", "killers": ["TestCross", "TestDecide"]}],
   "killers": ["TestCross", "TestDecide"], "packages": ["example.com/a", "example.com/b"], "package": "example.com/a"},
  {"file": "b.go", "symbol": "Validate", "name": "Validate", "start": 3, "end": 8,
   "rows": [{"model": "M", "element": "validate", "killers": ["TestStop"]}],
   "killers": ["TestStop"], "packages": ["example.com/a"], "package": "example.com/a"}
 ],
 "killer_packages": {"TestCross": "example.com/b", "TestDecide": "example.com/a", "TestStop": "example.com/a"},
 "unbound": [], "preflight": {}}
JSON
	git -C "$tmp/repo" add a.go b.go
	git -C "$tmp/repo" commit -q -m "fixture"

	cat >"$tmp/bin/go-mutesting" <<'GM'
#!/usr/bin/env bash
set -euo pipefail
worker_dir="$(pwd -P)"
printf '{"killedCount":1,"escapedCount":1,"errorCount":0,"skippedCount":0}\n' >go-mutesting-summary.json
printf '{"mutants":[{"id":"m1"}]}\n' >go-mutesting-agentic.json
cat >report.json <<JSON
{"killed":[],"escaped":[{"mutator":{"originalFilePath":"$worker_dir/a.go","mutatorName":"statement/remove","diff":"--- a\\n+++ b\\n"}}],"skipped":[],"errored":[]}
JSON
exit 4
GM
	chmod +x "$tmp/bin/go-mutesting"

	# Verify create_formal_workers resolves symlinked TMPDIR physically (macOS /var -> /private/var).
	mkdir -p "$tmp/real-tmp"
	ln -s "$tmp/real-tmp" "$tmp/link-tmp"
	local real_tmp_canon
	real_tmp_canon="$(cd -P "$tmp/real-tmp" && pwd)"

	(
		REPO_ROOT="$tmp/repo"
		GO_MUTESTING_BIN="$tmp/bin/go-mutesting"
		GO_CMD="$tmp/repo/bin/go"
		STUB_GO_MODE="json-pass"
		STUB_GO_ARGS="$tmp/go-args"
		TMPDIR="$tmp/link-tmp"
		export GO_CMD STUB_GO_MODE STUB_GO_ARGS TMPDIR
		unset GITHUB_STEP_SUMMARY

		# Parallel pre-flight across 2 files.
		export_rapid_determinism_env
		MUTATION_FORMAL_ACTIVE_JOBS=2 run_formal_preflight "$tmp/repo/plan.json" "$tmp/reports/preflight-par"
		# Parallel per-file shards across 2 detached worker worktrees.
		mapfile -t shard_rows < <(formal_mutation show --plan "$tmp/repo/plan.json" --per-file shards)
		mkdir -p "$tmp/reports/shard-run"
		cp "$tmp/repo/plan.json" "$tmp/reports/shard-run/$FORMAL_PLAN_FILE"
		: >"$tmp/reports/shard-run/go-mutesting.log"
		run_formal_shards full advisory "$tmp/reports/shard-run" "$tmp/repo/plan.json" 10 2 "${shard_rows[@]}" || true
		formal_mutation merge-reports --report-dir "$tmp/reports/shard-run" >/dev/null
		cleanup_mutation_paths
	) >/dev/null 2>&1

	assert_file_contains "parallel preflight recorded a.go timeout" '"a.go":' "$tmp/repo/plan.json"
	assert_file_contains "parallel preflight recorded b.go timeout" '"b.go":' "$tmp/repo/plan.json"
	assert_file_exists "shard-0 wrote worker-root marker" "$tmp/reports/shard-run/shard-0/.worker-root"
	assert_file_exists "shard-1 wrote worker-root marker" "$tmp/reports/shard-run/shard-1/.worker-root"
	assert_file_exists "shard-0 collected report.json" "$tmp/reports/shard-run/shard-0/report.json"
	assert_file_exists "shard-1 collected report.json" "$tmp/reports/shard-run/shard-1/report.json"
	assert_file_contains "worker-root resolves symlinked TMPDIR physically" "$real_tmp_canon" "$tmp/reports/shard-run/shard-0/.worker-root"
	assert_file_contains "merged report normalizes worker prefix to plan root" "$tmp/repo/a.go" "$tmp/reports/shard-run/report.json"
	metadata="$(cat "$tmp/reports/shard-run/run-metadata.txt")"
	assert_contains "shard-0 status recorded" "shard_status[shard-0]=4" "$metadata"
	assert_contains "shard-1 status recorded" "shard_status[shard-1]=4" "$metadata"
	wt_count="$(git -C "$tmp/repo" worktree list --porcelain | grep -c '^worktree ')"
	assert_eq "worker worktrees cleaned up cleanly" "1" "$wt_count"
}


test_paths
test_command_construction
test_interrupt_status_detection
test_untracked_cleanup_preserves_existing_files
test_tool_report_collection
test_tool_report_collection_before_untracked_cleanup
test_restore_guard_refuses_unarmed_checkouts
test_cleanup_trap_preserves_exit_status
test_dirty_path_policy
test_root_target_resolution
test_rapid_determinism_env
test_formal_exec_result_mapping
test_formal_exec_requires_determinism
test_formal_preflight
test_formal_command_construction
test_formal_target_set_rejections
test_formal_rerun_evidence
test_formal_group_status
test_formal_dirty_path_policy
test_formal_scope_and_jobs_validation
test_formal_parallel_shards_and_workers_with_stubs

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
