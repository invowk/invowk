#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GO_CMD="${GO_CMD:-go}"
GO_MUTESTING_TOOL="go-mutesting"
GO_MUTESTING_MODULE="github.com/jonbaldie/go-mutesting/v2"
GO_MUTESTING_VERSION="v2.8.3"
readonly MUTATION_RAPID_SEED=20260924
readonly MUTATION_RAPID_SHRINKTIME=2s
DEFAULT_REPORT_DIR="artifacts/mutation"
QUALITY_GATE_EXIT_CODE=4
TARGET_SET_ROOT="root"
TARGET_SET_FORMAL="formal-bindings"
FORMAL_MUTATION_PY="$SCRIPT_DIR/formal_mutation.py"
FORMAL_EXEC_SCRIPT="$SCRIPT_DIR/mutation-formal-exec.sh"
FORMAL_PLAN_FILE="formal-plan.json"
# The same interpreter mutation-formal-exec.sh uses, so one Python builds and
# reads the plan.
PYTHON="${PYTHON:-python3}"

GO_MUTESTING_BIN=""
MUTATION_RESTORE_MODULES=()
MUTATION_CLEANUP_DIR=""

usage() {
	cat <<'EOF'
Usage: scripts/mutation.sh <profile> [options]

Profiles:
  dry-run          Count candidate mutants without executing mutated tests.
  pr               Run changed-line mutation testing against a diff base.
  full             Run curated broad package manifests.
  baseline-update  Regenerate accepted-survivor baselines intentionally.
  rerun            Rerun one stable escaped-mutant ID.

Options:
  --module root               Module profile to run (default: root).
  --base REF                  Diff base for the pr profile (default: origin/main).
  --mode advisory|blocking    Gate behavior for escaped mutants (default: advisory).
  --mutant-id ID              Stable mutant id for the rerun profile.
  --target-set root|formal-bindings
                              Mutation targets (default: root). formal-bindings
                              mutates only the Go functions named by bound rows
                              of the formal models' correspondence tables and
                              runs only their binding tests; it is available on
                              dry-run, full, baseline-update, and rerun (not pr).
  --report-dir DIR            Report root (default: artifacts/mutation).
  --help                      Show this help.

Environment:
  MUTATION_MODULE, MUTATION_BASE_REF, MUTATION_MODE, MUTATION_MUTANT_ID,
  MUTATION_REPORT_DIR, MUTATION_WORKERS, MUTATION_TARGET_SET, GO_CMD.
  MUTATION_FORMAL_EXEC_TIMEOUT  Per-mutant go test timeout in seconds for
                                formal-bindings (default: max(10, 5 x the
                                file's measured clean pre-flight time)).
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

warn() {
	printf 'WARNING: %s\n' "$*" >&2
}

is_absolute_path() {
	local path="$1"
	[[ "$path" == /* ]]
}

repo_path() {
	local path="$1"

	if is_absolute_path "$path"; then
		printf '%s\n' "$path"
	else
		printf '%s/%s\n' "$REPO_ROOT" "$path"
	fi
}

trim_manifest_line() {
	local line="$1"

	line="${line%%#*}"
	line="${line#"${line%%[![:space:]]*}"}"
	line="${line%"${line##*[![:space:]]}"}"
	printf '%s\n' "$line"
}

read_manifest_entries() {
	local manifest="$1"
	local raw
	local line

	[[ -f "$manifest" ]] || die "manifest not found: $manifest"
	while IFS= read -r raw || [[ -n "$raw" ]]; do
		line="$(trim_manifest_line "$raw")"
		[[ -n "$line" ]] && printf '%s\n' "$line"
	done <"$manifest"
}

is_go_file_target() {
	local target="$1"

	[[ "$target" == *.go ]]
}

module_workdir() {
	local module="$1"

	case "$module" in
		root)
			printf '%s\n' "$REPO_ROOT"
			;;
		*)
			die "unknown mutation module: $module"
			;;
	esac
}

target_manifest_path() {
	local module="$1"

	case "$module" in
		root)
			printf '%s/tools/mutation/root-packages.txt\n' "$REPO_ROOT"
			;;
		*)
			die "unknown mutation module: $module"
			;;
	esac
}

exclude_manifest_path() {
	local module="$1"

	case "$module" in
		root)
			printf '%s/tools/mutation/root-exclude-packages.txt\n' "$REPO_ROOT"
			;;
		*)
			die "unknown mutation module: $module"
			;;
	esac
}

baseline_path() {
	local module="$1"
	local target_set="${2:-$TARGET_SET_ROOT}"

	if [[ "$target_set" == "$TARGET_SET_FORMAL" ]]; then
		printf '%s/tools/mutation/baselines/%s-baseline.json\n' "$REPO_ROOT" "$TARGET_SET_FORMAL"
		return 0
	fi
	case "$module" in
		root)
			printf '%s/tools/mutation/baselines/%s-baseline.json\n' "$REPO_ROOT" "$module"
			;;
		*)
			die "unknown mutation module: $module"
			;;
	esac
}

profile_report_dir() {
	local profile="$1"
	local module="$2"
	local report_root="${3:-$DEFAULT_REPORT_DIR}"
	local target_set="${4:-$TARGET_SET_ROOT}"
	local name="$module"

	[[ "$target_set" == "$TARGET_SET_FORMAL" ]] && name="$TARGET_SET_FORMAL"
	printf '%s/%s/%s\n' "$(repo_path "$report_root")" "$profile" "$name"
}

exclusion_reason_for() {
	local exclusions="$1"
	local import_path="$2"
	local raw
	local line
	local package_path
	local reason

	[[ -f "$exclusions" ]] || return 1
	while IFS= read -r raw || [[ -n "$raw" ]]; do
		line="$(trim_manifest_line "$raw")"
		[[ -z "$line" ]] && continue
		IFS='|' read -r package_path reason <<<"$line"
		if [[ "$package_path" == "$import_path" ]]; then
			printf '%s\n' "${reason:-excluded by manifest}"
			return 0
		fi
	done <"$exclusions"

	return 1
}

resolve_targets() {
	local module="$1"
	local report_dir="$2"
	local workdir
	local manifest
	local exclusions
	local candidates_file
	local resolved_file
	local excluded_file
	local not_covered_file
	local pattern
	local file_target
	local normalized_file
	local package_dir
	local package_pattern
	local package_info
	local file_import_path
	local import_path
	local go_files
	local test_files
	local xtest_files
	local test_count
	local reason
	local -a patterns=()
	local -a package_patterns=()
	local -a file_targets=()

	workdir="$(module_workdir "$module")"
	manifest="$(target_manifest_path "$module")"
	exclusions="$(exclude_manifest_path "$module")"
	mkdir -p "$report_dir"

	mapfile -t patterns < <(read_manifest_entries "$manifest")
	((${#patterns[@]} > 0)) || die "no target patterns found in $manifest"

	for pattern in "${patterns[@]}"; do
		if is_go_file_target "$pattern"; then
			file_targets+=("$pattern")
		else
			package_patterns+=("$pattern")
		fi
	done

	candidates_file="$report_dir/package-candidates.txt"
	resolved_file="$report_dir/resolved-targets.txt"
	excluded_file="$report_dir/excluded-packages.txt"
	not_covered_file="$report_dir/not-covered-packages.txt"

	: >"$candidates_file"
	: >"$resolved_file"
	: >"$excluded_file"
	: >"$not_covered_file"

	if ((${#package_patterns[@]} > 0)); then
		(cd "$workdir" && "$GO_CMD" list -f '{{.ImportPath}}	{{len .GoFiles}}	{{len .TestGoFiles}}	{{len .XTestGoFiles}}' "${package_patterns[@]}") >"$candidates_file"
	fi

	while IFS=$'\t' read -r import_path go_files test_files xtest_files; do
		[[ -z "$import_path" ]] && continue
		if reason="$(exclusion_reason_for "$exclusions" "$import_path")"; then
			printf '%s\t%s\n' "$import_path" "$reason" >>"$excluded_file"
			continue
		fi
		if ((go_files == 0)); then
			printf '%s\t%s\n' "$import_path" "no production Go files" >>"$excluded_file"
			continue
		fi

		test_count=$((test_files + xtest_files))
		if ((test_count == 0)); then
			printf '%s\t%s\n' "$import_path" "included but no local Go tests were discovered" >>"$not_covered_file"
		fi
		printf '%s\n' "$import_path" >>"$resolved_file"
	done <"$candidates_file"

	for file_target in "${file_targets[@]}"; do
		normalized_file="${file_target#./}"
		if is_absolute_path "$normalized_file"; then
			die "file mutation target must be relative to module workdir: $file_target"
		fi
		if [[ "$normalized_file" == *_test.go ]]; then
			printf '%s\t%s\n' "$file_target" "test file target is excluded" >>"$excluded_file"
			continue
		fi
		[[ -f "$workdir/$normalized_file" ]] || die "file mutation target not found: $file_target"

		package_dir="$(dirname "$normalized_file")"
		if [[ "$package_dir" == "." ]]; then
			package_pattern="."
		else
			package_pattern="./$package_dir"
		fi
		package_info="$(cd "$workdir" && "$GO_CMD" list -f '{{.ImportPath}}	{{len .TestGoFiles}}	{{len .XTestGoFiles}}' "$package_pattern")"
		IFS=$'\t' read -r file_import_path test_files xtest_files <<<"$package_info"
		test_count=$((test_files + xtest_files))
		printf '%s\t%s\t%s\t%s\n' "$file_target" "file target in $file_import_path" 1 "$test_count" >>"$candidates_file"
		if ((test_count == 0)); then
			printf '%s\t%s\n' "$file_target" "included but no local Go tests were discovered" >>"$not_covered_file"
		fi
		printf '%s\n' "$file_target" >>"$resolved_file"
	done

	[[ -s "$resolved_file" ]] || die "no mutation targets resolved for module $module"
	cat "$resolved_file"
}

dirty_path_is_allowed() {
	local path="$1"

	case "$path" in
		tools/mutation/baselines/root-baseline.json)
			return 0
			;;
		tools/mutation/baselines/formal-bindings-baseline.json | \
			tools/mutation/triage/formal-bindings.toml | \
			tools/mutation/triage/formal-bindings-reruns.jsonl)
			return 0
			;;
		artifacts/mutation/*)
			return 0
			;;
		*)
			return 1
			;;
	esac
}

ensure_clean_tracked_worktree_for_mutation() {
	local path
	local -a dirty_paths=()

	while IFS= read -r path; do
		[[ -z "$path" ]] && continue
		if ! dirty_path_is_allowed "$path"; then
			dirty_paths+=("$path")
		fi
	done < <({ git -C "$REPO_ROOT" diff --name-only; git -C "$REPO_ROOT" diff --cached --name-only; } | sort -u)

	if ((${#dirty_paths[@]} > 0)); then
		printf 'Mutation testing rewrites package sources while it runs.\n' >&2
		printf 'Commit, stash, or move these tracked changes before mutating:\n' >&2
		printf '  %s\n' "${dirty_paths[@]}" >&2
		exit 1
	fi
}

register_restore_module() {
	local module="$1"
	local existing

	for existing in "${MUTATION_RESTORE_MODULES[@]}"; do
		[[ "$existing" == "$module" ]] && return 0
	done
	snapshot_untracked_paths "$module"
	MUTATION_RESTORE_MODULES+=("$module")
}

ensure_cleanup_dir() {
	if [[ -z "$MUTATION_CLEANUP_DIR" || ! -d "$MUTATION_CLEANUP_DIR" ]]; then
		MUTATION_CLEANUP_DIR="$(mktemp -d)"
	fi
}

mutation_source_paths() {
	local module="$1"

	case "$module" in
		root)
			printf '%s\n' cmd internal pkg
			;;
	esac
}

snapshot_untracked_paths() {
	local module="$1"
	local snapshot
	local -a source_paths=()

	ensure_cleanup_dir
	snapshot="$MUTATION_CLEANUP_DIR/$module.untracked.before"
	mapfile -t source_paths < <(mutation_source_paths "$module")
	git -C "$REPO_ROOT" ls-files --others --exclude-standard -- "${source_paths[@]}" | sort >"$snapshot"
}

remove_new_untracked_paths() {
	local module="$1"
	local snapshot
	local current
	local path
	local -a source_paths=()

	[[ -n "$MUTATION_CLEANUP_DIR" ]] || return 0
	snapshot="$MUTATION_CLEANUP_DIR/$module.untracked.before"
	[[ -f "$snapshot" ]] || return 0
	current="$MUTATION_CLEANUP_DIR/$module.untracked.after"
	mapfile -t source_paths < <(mutation_source_paths "$module")
	git -C "$REPO_ROOT" ls-files --others --exclude-standard -- "${source_paths[@]}" | sort >"$current"

	while IFS= read -r path; do
		[[ -n "$path" ]] || continue
		rm -f -- "$REPO_ROOT/$path"
	done < <(comm -13 "$snapshot" "$current")
}

restore_tracked_mutation_paths() {
	local module="$1"

	case "$module" in
		root)
			git -C "$REPO_ROOT" restore --worktree -- cmd internal pkg >/dev/null 2>&1 || true
			;;
	esac
}

restore_mutation_paths() {
	local module="$1"

	restore_tracked_mutation_paths "$module"
	remove_new_untracked_paths "$module"
}

cleanup_mutation_paths() {
	local module

	for module in "${MUTATION_RESTORE_MODULES[@]}"; do
		restore_mutation_paths "$module"
	done
	if [[ -n "$MUTATION_CLEANUP_DIR" ]]; then
		rm -rf "$MUTATION_CLEANUP_DIR"
	fi
}

interrupted_status() {
	local status="$1"

	[[ "$status" =~ ^[0-9]+$ ]] && ((status >= 128))
}

go_mutesting_binary() {
	(cd "$REPO_ROOT" && "$GO_CMD" tool -n "$GO_MUTESTING_TOOL")
}

verify_go_mutesting_version() {
	local binary
	local resolved_version

	binary="$(go_mutesting_binary)" || die "failed to resolve $GO_MUTESTING_TOOL from the root go.mod tool directive"
	[[ -x "$binary" ]] || die "resolved $GO_MUTESTING_TOOL is not executable: $binary"

	resolved_version="$("$GO_CMD" version -m "$binary" | awk -v module="$GO_MUTESTING_MODULE" '$1 == "mod" && $2 == module {print $3; found=1} END {if (!found) exit 1}')" ||
		die "failed to read embedded module version from $binary"

	if [[ "$resolved_version" != "$GO_MUTESTING_VERSION" ]]; then
		die "expected $GO_MUTESTING_MODULE $GO_MUTESTING_VERSION, got $resolved_version"
	fi

	GO_MUTESTING_BIN="$binary"
}

# Property-based tests (pgregory.net/rapid) must decide each mutant the same way
# on every run: a fixed seed keeps kill/escape status stable against the
# stable-ID baseline, no failure files land in package testdata/, and shrinking
# a killed mutant's counterexample is bounded. Callers may override the seed and
# shrink time; failure files are always disabled.
export_rapid_determinism_env() {
	export RAPID_SEED="${RAPID_SEED:-$MUTATION_RAPID_SEED}"
	export RAPID_NOFAILFILE=1
	export RAPID_SHRINKTIME="${RAPID_SHRINKTIME:-$MUTATION_RAPID_SHRINKTIME}"
}

common_mutation_args() {
	local module="$1"
	local baseline
	local workers="${MUTATION_WORKERS:-0}"

	baseline="$(baseline_path "$module")"
	printf '%s\n' \
		"--baseline=$baseline" \
		"--coverage" \
		"--per-test" \
		"--test-flags=-short" \
		"--timeout-coefficient=3" \
		"--workers=$workers" \
		"--logger-summary-json" \
		"--logger-agentic-json" \
		"--quiet" \
		"--no-diffs"
}

build_go_mutesting_args() {
	local profile="$1"
	local module="$2"
	local mode="${3:-advisory}"
	local base_ref="${4:-origin/main}"
	local mutant_id="${5:-}"
	local baseline
	local -a args=()

	baseline="$(baseline_path "$module")"
	case "$profile" in
		dry-run)
			args=("--baseline=$baseline" "--dry-run")
			;;
		pr)
			mapfile -t args < <(common_mutation_args "$module")
			args+=("--git-diff-lines" "--git-diff-base=$base_ref" "--ignore-msi-with-no-mutations" "--logger-github")
			[[ "$mode" == "blocking" ]] && args+=("--fail-on-escaped")
			;;
		full)
			mapfile -t args < <(common_mutation_args "$module")
			[[ "$mode" == "blocking" ]] && args+=("--fail-on-escaped")
			;;
		baseline-update)
			mapfile -t args < <(common_mutation_args "$module")
			args+=("--update-baseline")
			;;
		rerun)
			[[ -n "$mutant_id" ]] || die "rerun profile requires --mutant-id or MUTATION_MUTANT_ID"
			mapfile -t args < <(common_mutation_args "$module")
			args+=("--run-mutant-id=$mutant_id" "--output-statuses=e")
			;;
		*)
			die "unknown mutation profile: $profile"
			;;
	esac

	printf '%s\n' "${args[@]}"
}

remove_stale_tool_reports() {
	local workdir="$1"
	local report

	for report in \
		report.json \
		go-mutesting-summary.json \
		go-mutesting-agentic.json \
		go-mutesting-gitlab.json \
		go-mutesting-report.html; do
		rm -f "$workdir/$report"
	done
}

collect_tool_reports() {
	local workdir="$1"
	local report_dir="$2"
	local report

	for report in \
		report.json \
		go-mutesting-summary.json \
		go-mutesting-agentic.json \
		go-mutesting-gitlab.json \
		go-mutesting-report.html; do
		if [[ -f "$workdir/$report" ]]; then
			mv -f "$workdir/$report" "$report_dir/$report"
		fi
	done
}

write_run_metadata() {
	local profile="$1"
	local module="$2"
	local mode="$3"
	local base_ref="$4"
	local report_dir="$5"
	local target_set="${6:-$TARGET_SET_ROOT}"
	local targets_file="$report_dir/resolved-targets.txt"
	local target_count=0

	if [[ -f "$targets_file" ]]; then
		target_count="$(wc -l <"$targets_file" | tr -d ' ')"
	fi

	{
		printf 'profile=%s\n' "$profile"
		printf 'module=%s\n' "$module"
		printf 'target_set=%s\n' "$target_set"
		printf 'mode=%s\n' "$mode"
		printf 'base_ref=%s\n' "$base_ref"
		printf 'tool_module=%s\n' "$GO_MUTESTING_MODULE"
		printf 'tool_version=%s\n' "$GO_MUTESTING_VERSION"
		printf 'target_count=%s\n' "$target_count"
		printf 'baseline=%s\n' "$(baseline_path "$module" "$target_set")"
	} >"$report_dir/run-metadata.txt"
}

append_step_summary() {
	local profile="$1"
	local module="$2"
	local report_dir="$3"
	local summary_file="$report_dir/go-mutesting-summary.json"

	[[ -n "${GITHUB_STEP_SUMMARY:-}" ]] || return 0

	{
		printf '### Mutation testing: %s / %s\n\n' "$profile" "$module"
		# shellcheck disable=SC2016 # backticks are literal Markdown code spans
		printf '- Reports: `%s`\n' "${report_dir#"$REPO_ROOT/"}"
		# shellcheck disable=SC2016 # backticks are literal Markdown code spans
		printf '- Targets: `%s`\n' "${report_dir#"$REPO_ROOT/"}/resolved-targets.txt"
		if [[ -f "$summary_file" ]]; then
			printf '\n```json\n'
			cat "$summary_file"
			printf '\n```\n'
		fi
		printf '\n'
	} >>"$GITHUB_STEP_SUMMARY"
}

run_module_profile() {
	local profile="$1"
	local module="$2"
	local mode="$3"
	local base_ref="$4"
	local mutant_id="$5"
	local report_root="$6"
	local report_dir
	local workdir
	local status
	local -a targets=()
	local -a args=()

	report_dir="$(profile_report_dir "$profile" "$module" "$report_root")"
	workdir="$(module_workdir "$module")"
	mkdir -p "$report_dir"

	mapfile -t targets < <(resolve_targets "$module" "$report_dir")
	mapfile -t args < <(build_go_mutesting_args "$profile" "$module" "$mode" "$base_ref" "$mutant_id")
	write_run_metadata "$profile" "$module" "$mode" "$base_ref" "$report_dir"
	remove_stale_tool_reports "$workdir"

	if [[ "$profile" != "dry-run" ]]; then
		ensure_clean_tracked_worktree_for_mutation
		register_restore_module "$module"
	fi

	printf 'Running mutation profile %s for %s (%s targets)\n' "$profile" "$module" "${#targets[@]}"

	set +e
	(
		export_rapid_determinism_env
		cd "$workdir" && "$GO_MUTESTING_BIN" "${args[@]}" "${targets[@]}"
	) 2>&1 | tee "$report_dir/go-mutesting.log"
	status=${PIPESTATUS[0]}
	set -e

	if [[ "$profile" != "dry-run" ]]; then
		restore_tracked_mutation_paths "$module"
	fi
	collect_tool_reports "$workdir" "$report_dir"
	if [[ "$profile" != "dry-run" ]]; then
		remove_new_untracked_paths "$module"
	fi
	append_step_summary "$profile" "$module" "$report_dir"

	if [[ "$status" -eq "$QUALITY_GATE_EXIT_CODE" && "$mode" == "advisory" ]]; then
		warn "escaped mutants were reported for $module, but advisory mode is non-blocking"
		return 0
	fi

	return "$status"
}

# ---------------------------------------------------------------------------
# formal-bindings target set (plan and ledger logic: scripts/formal_mutation.py)

formal_mutation() {
	"$PYTHON" "$FORMAL_MUTATION_PY" "$@"
}

resolve_formal_targets() {
	local report_dir="$1"

	mkdir -p "$report_dir"
	formal_mutation plan --out "$report_dir" >&2 || die "formal-bindings plan generation failed; no mutant has run"
	cat "$report_dir/resolved-targets.txt"
}

elapsed_seconds() {
	local start="$1"
	local end="$2"

	awk -v start="${start/,/.}" -v end="${end/,/.}" 'BEGIN { printf "%.3f\n", end - start }'
}

# Clean-code pre-flight: go-mutesting ignores --noop together with --exec, so
# every target file's killer tests (or only FILE... when given) run once on
# unmutated code first. Each run
# must pass with every killer reported as passed (not skipped); the measured
# wall time sets the file's exec timeout in the plan.
run_formal_preflight() {
	local plan="$1"
	local report_dir="$2"
	shift 2
	local file
	local json
	local start
	local status
	local -a files=("$@")

	mkdir -p "$report_dir/preflight"
	((${#files[@]} > 0)) || mapfile -t files < <(formal_mutation show --plan "$plan" files)
	((${#files[@]} > 0)) || die "formal-bindings plan lists no target files"
	for file in "${files[@]}"; do
		json="$report_dir/preflight/${file//\//_}.json"
		: >"$json"
		start="$EPOCHREALTIME"
		set +e
		MUTATION_FORMAL_PLAN="$plan" \
			MUTATE_ORIGINAL="$REPO_ROOT/$file" \
			MUTATE_CHANGED="$REPO_ROOT/$file" \
			MUTATION_FORMAL_PREFLIGHT_JSON="$json" \
			"$FORMAL_EXEC_SCRIPT"
		status=$?
		set -e
		formal_mutation preflight-record --plan "$plan" --file "$file" --json "$json" \
			--status "$status" --seconds "$(elapsed_seconds "$start" "$EPOCHREALTIME")" ||
			die "formal-bindings pre-flight failed for $file; no mutant has run"
	done
}

build_formal_mutation_args() {
	local profile="$1"
	local mode="$2"
	local mutant_id="$3"
	local match="$4"
	local exec_timeout="$5"
	local group_baseline="$6"
	local baseline
	local -a args=()

	[[ "$FORMAL_EXEC_SCRIPT" != *[[:space:]]* ]] || die "go-mutesting splits --exec on whitespace: $FORMAL_EXEC_SCRIPT"
	baseline="$(baseline_path root "$TARGET_SET_FORMAL")"
	if [[ "$profile" == "dry-run" ]]; then
		printf '%s\n' "--match=$match" "--baseline=$baseline" "--dry-run"
		return 0
	fi
	# No --coverage, --per-test, --test-flags, or --timeout-coefficient: the
	# executor runs the mutated function's binding tests itself.
	args=(
		"--match=$match"
		"--exec=$FORMAL_EXEC_SCRIPT"
		"--exec-timeout=$exec_timeout"
		"--logger-summary-json"
		"--logger-agentic-json"
		"--quiet"
		"--no-diffs"
	)
	case "$profile" in
		full)
			args+=("--baseline=$baseline")
			[[ "$mode" == "blocking" ]] && args+=("--fail-on-escaped")
			;;
		baseline-update)
			args+=("--baseline=$group_baseline" "--update-baseline")
			;;
		rerun)
			[[ -n "$mutant_id" ]] || die "rerun profile requires --mutant-id or MUTATION_MUTANT_ID"
			args+=("--baseline=$baseline" "--run-mutant-id=$mutant_id" "--output-statuses=e")
			;;
		*)
			die "the $TARGET_SET_FORMAL target set does not support the $profile profile"
			;;
	esac
	printf '%s\n' "${args[@]}"
}

# run_formal_group GROUP_DIR LOG ARGS... FILES are passed after a literal "--".
run_formal_group() {
	local group_dir="$1"
	local log="$2"
	local plan="$3"
	shift 3
	local -a args=()
	local status

	while (($# > 0)) && [[ "$1" != "--" ]]; do
		args+=("$1")
		shift
	done
	shift
	mkdir -p "$group_dir"
	set +e
	(
		export_rapid_determinism_env
		export MUTATION_FORMAL_PLAN="$plan"
		cd "$REPO_ROOT" && "$GO_MUTESTING_BIN" "${args[@]}" "$@"
	) 2>&1 | tee -a "$log"
	status=${PIPESTATUS[0]}
	set -e
	collect_tool_reports "$REPO_ROOT" "$group_dir"
	return "$status"
}

# Focused reruns: MUTATION_MUTANT_ID may list several ids separated by commas.
# Each id is scoped to its file through the committed baseline or the last full
# report (the stable id hashes the file), so only that file is pre-flighted and
# mutated; an unknown id is rerun across every group. Each outcome is appended
# to the rerun evidence file.
run_formal_rerun() {
	local mode="$1"
	local mutant_ids="$2"
	local report_root="$3"
	local report_dir
	local plan
	local mutant
	local group
	local file
	local match
	local exec_timeout
	local status=0
	local id_status
	local -a ids=()
	local -a scope=()
	local -a preflight_files=()
	local -a groups=()
	local -a files=()
	local -a args=()

	report_dir="$(profile_report_dir rerun root "$report_root" "$TARGET_SET_FORMAL")"
	rm -rf "$report_dir"
	mkdir -p "$report_dir"
	resolve_formal_targets "$report_dir" >/dev/null
	plan="$report_dir/$FORMAL_PLAN_FILE"
	write_run_metadata rerun root "$mode" "" "$report_dir" "$TARGET_SET_FORMAL"
	remove_stale_tool_reports "$REPO_ROOT"
	ensure_clean_tracked_worktree_for_mutation
	register_restore_module root

	IFS=',' read -r -a ids <<<"$mutant_ids"
	mapfile -t scope < <(formal_mutation rerun-scope --plan "$plan" \
		--hint "$(baseline_path root "$TARGET_SET_FORMAL")" \
		--hint "$(repo_path "$report_root")/full/$TARGET_SET_FORMAL/go-mutesting-agentic.json" "${ids[@]}")
	((${#scope[@]} == ${#ids[@]})) || die "could not scope the rerun ids"
	printf '%s\n' "${scope[@]}" >"$report_dir/rerun-scope.tsv"
	if ! grep -q $'\t-\t' "$report_dir/rerun-scope.tsv"; then
		mapfile -t preflight_files < <(cut -f3 "$report_dir/rerun-scope.tsv" | sort -u)
	fi
	(
		export_rapid_determinism_env
		run_formal_preflight "$plan" "$report_dir" "${preflight_files[@]}"
	) || return 1
	exec_timeout="$(formal_mutation show --plan "$plan" max-timeout)"

	while IFS=$'\t' read -r mutant group file; do
		if [[ "$group" == "-" ]]; then
			mapfile -t groups < <(seq 0 $(($(formal_mutation show --plan "$plan" group-count) - 1)))
		else
			groups=("$group")
		fi
		for group in "${groups[@]}"; do
			match="$(formal_mutation show --plan "$plan" --group "$group" match)"
			if [[ "$file" == "-" ]]; then
				mapfile -t files < <(formal_mutation show --plan "$plan" --group "$group" files)
			else
				files=("$file")
			fi
			mapfile -t args < <(build_formal_mutation_args rerun "$mode" "$mutant" "$match" "$exec_timeout" "")
			id_status=0
			run_formal_group "$report_dir/$mutant/group-$group" "$report_dir/$mutant/go-mutesting.log" "$plan" \
				"${args[@]}" -- "${files[@]}" || id_status=$?
			interrupted_status "$id_status" && return "$id_status"
		done
		if formal_mutation merge-reports --report-dir "$report_dir/$mutant" >/dev/null &&
			formal_mutation rerun-record --plan "$plan" --summary "$report_dir/$mutant/go-mutesting-summary.json" --id "$mutant"; then
			:
		else
			status=1
		fi
	done <"$report_dir/rerun-scope.tsv"

	restore_tracked_mutation_paths root
	remove_new_untracked_paths root
	return "$status"
}

run_formal_profile() {
	local profile="$1"
	local mode="$2"
	local report_root="$3"
	local report_dir
	local plan
	local groups
	local group
	local group_dir
	local match
	local exec_timeout=0
	local status=0
	local group_status
	local -a targets=()
	local -a files=()
	local -a args=()

	report_dir="$(profile_report_dir "$profile" root "$report_root" "$TARGET_SET_FORMAL")"
	rm -rf "$report_dir"
	mkdir -p "$report_dir"
	mapfile -t targets < <(resolve_formal_targets "$report_dir")
	((${#targets[@]} > 0)) || die "no formal-bindings targets resolved"
	plan="$report_dir/$FORMAL_PLAN_FILE"
	write_run_metadata "$profile" root "$mode" "" "$report_dir" "$TARGET_SET_FORMAL"
	remove_stale_tool_reports "$REPO_ROOT"

	if [[ "$profile" != "dry-run" ]]; then
		ensure_clean_tracked_worktree_for_mutation
		register_restore_module root
		(
			export_rapid_determinism_env
			run_formal_preflight "$plan" "$report_dir"
		) || return 1
		exec_timeout="$(formal_mutation show --plan "$plan" max-timeout)"
	fi

	groups="$(formal_mutation show --plan "$plan" group-count)"
	printf 'Running mutation profile %s for %s (%s files in %s match group(s))\n' \
		"$profile" "$TARGET_SET_FORMAL" "${#targets[@]}" "$groups"
	: >"$report_dir/go-mutesting.log"
	for ((group = 0; group < groups; group++)); do
		group_dir="$report_dir/group-$group"
		match="$(formal_mutation show --plan "$plan" --group "$group" match)"
		mapfile -t files < <(formal_mutation show --plan "$plan" --group "$group" files)
		mapfile -t args < <(build_formal_mutation_args "$profile" "$mode" "" "$match" "$exec_timeout" "$group_dir/baseline.json")
		group_status=0
		run_formal_group "$group_dir" "$report_dir/go-mutesting.log" "$plan" "${args[@]}" -- "${files[@]}" || group_status=$?

		if ((group_status != 0)) && { ((status == 0)) || ((status == QUALITY_GATE_EXIT_CODE)); }; then
			status="$group_status"
		fi
		interrupted_status "$group_status" && break
	done

	if [[ "$profile" != "dry-run" ]]; then
		restore_tracked_mutation_paths root
		remove_new_untracked_paths root
		# go-mutesting writes no reports with --update-baseline. merge-reports
		# also appends the per-file timeout-kill counts to run-metadata.txt.
		if [[ "$profile" != "baseline-update" ]]; then
			formal_mutation merge-reports --report-dir "$report_dir" || status=1
		fi
	fi
	if ((status == 0)); then
		if [[ "$profile" == "baseline-update" ]]; then
			formal_mutation merge-baselines --report-dir "$report_dir" &&
				formal_mutation triage --check || status=1
		fi
	fi
	append_step_summary "$profile" "$TARGET_SET_FORMAL" "$report_dir"

	if [[ "$status" -eq "$QUALITY_GATE_EXIT_CODE" && "$mode" == "advisory" ]]; then
		warn "escaped mutants were reported for $TARGET_SET_FORMAL, but advisory mode is non-blocking"
		return 0
	fi
	return "$status"
}

modules_to_run() {
	local module="$1"

	case "$module" in
		root)
			printf '%s\n' "$module"
			;;
		all)
			printf '%s\n' root
			;;
		*)
			die "unknown mutation module: $module"
			;;
	esac
}

main() {
	local profile="${1:-}"
	local module="${MUTATION_MODULE:-root}"
	local base_ref="${MUTATION_BASE_REF:-origin/main}"
	local mode="${MUTATION_MODE:-advisory}"
	local mutant_id="${MUTATION_MUTANT_ID:-}"
	local report_root="${MUTATION_REPORT_DIR:-$DEFAULT_REPORT_DIR}"
	local target_set="${MUTATION_TARGET_SET:-$TARGET_SET_ROOT}"
	local selected_module
	local status=0
	local module_status=0

	if [[ -z "$profile" || "$profile" == "--help" || "$profile" == "-h" ]]; then
		usage
		return 0
	fi
	shift

	while (($# > 0)); do
		case "$1" in
			--module)
				shift
				(($# > 0)) || die "--module requires a value"
				module="$1"
				;;
			--module=*)
				module="${1#--module=}"
				;;
			--base)
				shift
				(($# > 0)) || die "--base requires a value"
				base_ref="$1"
				;;
			--base=*)
				base_ref="${1#--base=}"
				;;
			--mode)
				shift
				(($# > 0)) || die "--mode requires a value"
				mode="$1"
				;;
			--mode=*)
				mode="${1#--mode=}"
				;;
			--mutant-id)
				shift
				(($# > 0)) || die "--mutant-id requires a value"
				mutant_id="$1"
				;;
			--mutant-id=*)
				mutant_id="${1#--mutant-id=}"
				;;
			--report-dir)
				shift
				(($# > 0)) || die "--report-dir requires a value"
				report_root="$1"
				;;
			--report-dir=*)
				report_root="${1#--report-dir=}"
				;;
			--target-set)
				shift
				(($# > 0)) || die "--target-set requires a value"
				target_set="$1"
				;;
			--target-set=*)
				target_set="${1#--target-set=}"
				;;
			--help|-h)
				usage
				return 0
				;;
			*)
				die "unknown option: $1"
				;;
		esac
		shift
	done

	case "$profile" in
		dry-run|pr|full|baseline-update|rerun)
			;;
		*)
			die "unknown mutation profile: $profile"
			;;
	esac
	case "$mode" in
		advisory|blocking)
			;;
		*)
			die "unknown mutation mode: $mode"
			;;
	esac
	case "$target_set" in
		"$TARGET_SET_ROOT")
			;;
		"$TARGET_SET_FORMAL")
			if [[ "$profile" == "pr" ]]; then
				die "the $TARGET_SET_FORMAL target set is not available on the pr profile; use dry-run, full, baseline-update, or rerun (make mutation-formal*)"
			fi
			;;
		*)
			die "unknown mutation target set: $target_set (expected $TARGET_SET_ROOT or $TARGET_SET_FORMAL)"
			;;
	esac
	if [[ "$profile" == "rerun" && -z "$mutant_id" ]]; then
		die "rerun profile requires --mutant-id or MUTATION_MUTANT_ID"
	fi
	if [[ "$profile" == "baseline-update" ]]; then
		warn "baseline-update intentionally rewrites accepted-survivor baseline files"
	fi

	verify_go_mutesting_version
	trap cleanup_mutation_paths EXIT INT TERM

	if [[ "$target_set" == "$TARGET_SET_FORMAL" ]]; then
		if [[ "$profile" == "rerun" ]]; then
			run_formal_rerun "$mode" "$mutant_id" "$report_root"
		else
			run_formal_profile "$profile" "$mode" "$report_root"
		fi
		return
	fi

	while IFS= read -r selected_module; do
		[[ -z "$selected_module" ]] && continue
		if run_module_profile "$profile" "$selected_module" "$mode" "$base_ref" "$mutant_id" "$report_root"; then
			:
		else
			module_status=$?
			status="$module_status"
			if interrupted_status "$module_status"; then
				return "$status"
			fi
		fi
	done < <(modules_to_run "$module")

	return "$status"
}

if [[ "${INVOWK_MUTATION_TESTING:-0}" != "1" ]]; then
	main "$@"
fi
