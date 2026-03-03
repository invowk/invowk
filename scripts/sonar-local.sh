#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SONAR_HOST_URL="${SONAR_HOST_URL:-https://sonarcloud.io}"
SONAR_ORGANIZATION="${SONAR_ORGANIZATION:-invowk}"
SONAR_PROJECT_KEY="${SONAR_PROJECT_KEY:-invowk}"
SONAR_BRANCH="${SONAR_BRANCH:-}"
SONAR_TIMEOUT_SECONDS="${SONAR_TIMEOUT_SECONDS:-180}"
SONAR_POLL_INTERVAL_SECONDS="${SONAR_POLL_INTERVAL_SECONDS:-2}"
SONAR_PAGE_SIZE="${SONAR_PAGE_SIZE:-500}"

REPORT_DIR=".sonar/reports"
CHECKSTYLE_REPORT="$REPORT_DIR/golangci-checkstyle.xml"
SCANNER_LOG="$REPORT_DIR/sonar-scanner.log"
CE_TASK_JSON="$REPORT_DIR/ce-task.json"
ISSUES_NDJSON="$REPORT_DIR/issues.ndjson"
ISSUES_JSON="$REPORT_DIR/issues.json"

fail() {
	echo "ERROR: $*" >&2
	exit 1
}

info() {
	echo "==> $*"
}

require_cmd() {
	local cmd="$1"
	command -v "$cmd" >/dev/null 2>&1 || fail "Required command not found: $cmd"
}

read_task_value() {
	local key="$1"
	local file="$2"
	grep -E "^${key}=" "$file" | sed -E "s/^${key}=//" | head -n1 || true
}

get_current_branch() {
	local branch
	branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
	if [[ "$branch" == "HEAD" ]]; then
		echo ""
		return
	fi
	echo "$branch"
}

run_sonar_scanner() {
	local branch="$1"
	local -a args

	args=(
		"-Dsonar.host.url=${SONAR_HOST_URL}"
		"-Dsonar.organization=${SONAR_ORGANIZATION}"
		"-Dsonar.projectKey=${SONAR_PROJECT_KEY}"
		"-Dsonar.go.golangci-lint.reportPaths=${CHECKSTYLE_REPORT}"
	)

	if [[ -n "$branch" ]]; then
		args+=("-Dsonar.branch.name=${branch}")
	fi

	SONAR_TOKEN="$SONAR_TOKEN" sonar-scanner "${args[@]}" 2>&1 | tee "$SCANNER_LOG"
}

wait_for_compute_engine_task() {
	local ce_task_url="$1"
	local attempt=0
	local max_attempts
	local response status error_message

	if (( SONAR_POLL_INTERVAL_SECONDS <= 0 )); then
		fail "SONAR_POLL_INTERVAL_SECONDS must be > 0"
	fi

	max_attempts=$((SONAR_TIMEOUT_SECONDS / SONAR_POLL_INTERVAL_SECONDS))
	if (( max_attempts < 1 )); then
		max_attempts=1
	fi

	while (( attempt < max_attempts )); do
		response="$(curl -fsS -u "${SONAR_TOKEN}:" "$ce_task_url")"
		status="$(jq -r '.task.status // ""' <<<"$response")"

		case "$status" in
		SUCCESS)
			printf '%s\n' "$response" >"$CE_TASK_JSON"
			return 0
			;;
		PENDING | IN_PROGRESS)
			sleep "$SONAR_POLL_INTERVAL_SECONDS"
			;;
		FAILED | CANCELED)
			error_message="$(jq -r '.task.errorMessage // "unknown compute-engine error"' <<<"$response")"
			fail "Sonar compute engine task ${status}: ${error_message}"
			;;
		*)
			fail "Unexpected Sonar compute engine task status: ${status:-<empty>}"
			;;
		esac

		attempt=$((attempt + 1))
	done

	fail "Timed out waiting for Sonar compute engine task (${SONAR_TIMEOUT_SECONDS}s)"
}

fetch_unresolved_issues() {
	local branch="$1"
	local page=1
	local total=0
	local page_count=0
	local retrieved=0
	local response
	local -a curl_args

	: >"$ISSUES_NDJSON"

	while :; do
		curl_args=(
			-fsS
			-u "${SONAR_TOKEN}:"
			--get "${SONAR_HOST_URL%/}/api/issues/search"
			--data-urlencode "componentKeys=${SONAR_PROJECT_KEY}"
			--data-urlencode "resolved=false"
			--data-urlencode "p=${page}"
			--data-urlencode "ps=${SONAR_PAGE_SIZE}"
		)
		if [[ -n "$branch" ]]; then
			curl_args+=(--data-urlencode "branch=${branch}")
		fi

		response="$(curl "${curl_args[@]}")"
		if (( page == 1 )); then
			total="$(jq -r '.paging.total // .total // 0' <<<"$response")"
		fi

		jq -c '.issues[]?' <<<"$response" >>"$ISSUES_NDJSON"
		page_count="$(jq -r '.issues | length' <<<"$response")"
		retrieved=$((retrieved + page_count))

		if (( page_count == 0 || retrieved >= total )); then
			break
		fi

		page=$((page + 1))
	done

	jq -s '.' "$ISSUES_NDJSON" >"$ISSUES_JSON"
}

print_issue_table() {
	local issue_count="$1"
	local table_rows

	if (( issue_count == 0 )); then
		echo "No unresolved issues found."
		return 0
	fi

	printf '%s\n' ""
	printf '%s\n' "SEVERITY\tTYPE\tRULE\tFILE:LINE\tMESSAGE"
	table_rows="$(
		jq -r '
			.[] |
			[
				(.severity // "-"),
				(.type // "-"),
				(.rule // "-"),
				(
					(
						(.component // "-")
						| split(":")
						| if length > 1 then .[1] else .[0] end
					)
					+ ":" +
					((.line // "-") | tostring)
				),
				((.message // "-") | gsub("[\r\n\t]+"; " "))
			]
			| @tsv
		' "$ISSUES_JSON"
	)"

	if command -v column >/dev/null 2>&1; then
		printf '%s\n' "$table_rows" | column -t -s $'\t'
	else
		printf '%s\n' "$table_rows"
	fi
}

main() {
	local branch dashboard_url ce_task_url default_dashboard_url issue_count

	require_cmd golangci-lint
	require_cmd sonar-scanner
	require_cmd curl
	require_cmd jq

	if [[ -z "${SONAR_TOKEN:-}" ]]; then
		fail "SONAR_TOKEN is required"
	fi

	if (( SONAR_PAGE_SIZE <= 0 )); then
		fail "SONAR_PAGE_SIZE must be > 0"
	fi

	mkdir -p "$REPORT_DIR"
	rm -f "$CHECKSTYLE_REPORT" "$SCANNER_LOG" "$CE_TASK_JSON" "$ISSUES_NDJSON" "$ISSUES_JSON"

	branch="$SONAR_BRANCH"
	if [[ -z "$branch" ]]; then
		branch="$(get_current_branch)"
	fi

	info "Generating golangci-lint checkstyle report from .golangci.toml"
	golangci-lint run --config .golangci.toml --output.checkstyle.path "$CHECKSTYLE_REPORT" ./...

	if [[ -n "$branch" ]]; then
		info "Running sonar-scanner for branch '$branch'"
	else
		info "Running sonar-scanner for default branch context"
	fi
	if ! run_sonar_scanner "$branch"; then
		if [[ -n "$branch" ]] && grep -Eiq 'branch|edition|not authorized|unknown parameter' "$SCANNER_LOG"; then
			info "Branch analysis is unavailable. Retrying without branch parameter."
			branch=""
			run_sonar_scanner "$branch"
		else
			fail "sonar-scanner failed. See $SCANNER_LOG"
		fi
	fi

	if [[ ! -f ".scannerwork/report-task.txt" ]]; then
		fail "Missing .scannerwork/report-task.txt after sonar-scanner run"
	fi

	ce_task_url="$(read_task_value "ceTaskUrl" ".scannerwork/report-task.txt")"
	dashboard_url="$(read_task_value "dashboardUrl" ".scannerwork/report-task.txt")"
	[[ -n "$ce_task_url" ]] || fail "Could not read ceTaskUrl from .scannerwork/report-task.txt"

	default_dashboard_url="${SONAR_HOST_URL%/}/dashboard?id=${SONAR_PROJECT_KEY}"
	if [[ -n "$dashboard_url" ]]; then
		echo "Sonar dashboard: $dashboard_url"
	else
		echo "Sonar dashboard: $default_dashboard_url"
	fi

	info "Waiting for Sonar compute engine to finish analysis"
	wait_for_compute_engine_task "$ce_task_url"

	info "Fetching unresolved Sonar issues"
	fetch_unresolved_issues "$branch"
	issue_count="$(jq -r 'length' "$ISSUES_JSON")"

	echo "Unresolved Sonar issues: $issue_count"
	echo "Issues JSON report: $ISSUES_JSON"
	print_issue_table "$issue_count"
}

main "$@"
