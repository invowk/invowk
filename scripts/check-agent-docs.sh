#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

errors=0

cleanup_files=()
cleanup() {
	local file
	for file in "${cleanup_files[@]}"; do
		rm -f "$file"
	done
}
trap cleanup EXIT

new_tmp() {
	local file
	file="$(mktemp)"
	cleanup_files+=("$file")
	printf '%s\n' "$file"
}

has_rg=0
if command -v rg >/dev/null 2>&1; then
	has_rg=1
fi
if [[ "${INVOWK_CHECK_AGENT_DOCS_FORCE_GREP:-0}" == "1" ]]; then
	has_rg=0
fi

extract_matches() {
	local regex="$1"
	local file="$2"
	if [[ "$has_rg" -eq 1 ]]; then
		rg -o "$regex" "$file"
	else
		grep -oE -- "$regex" "$file" || true
	fi
}

search_docs() {
	local pattern="$1"
	if [[ "$has_rg" -eq 1 ]]; then
		rg -n -g '!**/speckit.*/SKILL.md' -- "$pattern" \
			.agents/rules \
			.agents/skills/*/SKILL.md \
			tools/goplint/AGENTS.md
	else
		grep -nE -- "$pattern" \
			.agents/rules/*.md \
			.agents/skills/*/SKILL.md \
			tools/goplint/AGENTS.md \
			| grep -vE '\.agents/skills/speckit\.[^/]+/SKILL\.md:' || true
	fi
}

check_set_equality() {
	local name="$1"
	local expected_file="$2"
	local actual_file="$3"

	local missing_file extra_file
	missing_file="$(new_tmp)"
	extra_file="$(new_tmp)"

	comm -23 "$expected_file" "$actual_file" >"$missing_file"
	comm -13 "$expected_file" "$actual_file" >"$extra_file"

	if [[ -s "$missing_file" || -s "$extra_file" ]]; then
		echo "ERROR: ${name} is out of sync."
		if [[ -s "$missing_file" ]]; then
			echo "  Missing from AGENTS.md index:"
			sed 's/^/    - /' "$missing_file"
		fi
		if [[ -s "$extra_file" ]]; then
			echo "  Listed in AGENTS.md index but missing on disk:"
			sed 's/^/    - /' "$extra_file"
		fi
		errors=1
	else
		echo "OK: ${name} is in sync."
	fi
}

echo "Checking agent docs integrity..."

rules_on_disk="$(new_tmp)"
rules_indexed="$(new_tmp)"
rules_regex="\\[\`\\.agents/rules/[^\`]+\`\\]"
find .agents/rules -maxdepth 1 -type f -name '*.md' | sed 's#^\./##' | sort >"$rules_on_disk"
extract_matches "$rules_regex" AGENTS.md \
	| sed -E "s/^\[\`(.+)\`\]$/\1/" \
	| sort >"$rules_indexed"
check_set_equality "Rules index (.agents/rules)" "$rules_on_disk" "$rules_indexed"

skills_on_disk="$(new_tmp)"
skills_indexed="$(new_tmp)"
skills_regex="\\[\`\\.agents/skills/[^\`]+\`\\]"
find .agents/skills -mindepth 1 -maxdepth 1 -type d \
	| while read -r dir; do
		if [[ -f "$dir/SKILL.md" ]]; then
			echo "$dir"
		fi
	done \
	| sed 's#^\./##' \
	| sort >"$skills_on_disk"
extract_matches "$skills_regex" AGENTS.md \
	| sed -E "s/^\[\`(.+)\`\]$/\1/; s#/\$##" \
	| sort >"$skills_indexed"
check_set_equality "Skills index (.agents/skills)" "$skills_on_disk" "$skills_indexed"

alias_refs="$(new_tmp)"
search_docs '\.claude/(rules|skills|agents)' >"$alias_refs" || true
if [[ -s "$alias_refs" ]]; then
	echo "ERROR: Found non-canonical .claude alias references in rules/skills."
	sed 's/^/  /' "$alias_refs"
	echo "  Use .agents/* canonical paths in documentation."
	errors=1
else
	echo "OK: No non-canonical .claude alias references in rules/skills."
fi

echo
echo "Advisory duplicate-policy scan (manual review signal):"
for pattern in \
	'internal \* commands MUST remain hidden' \
	'container runtime ONLY supports Linux containers' \
	"All new test functions MUST call \`t\.Parallel\(\)\`"
do
	pattern_matches="$(new_tmp)"
	search_docs "$pattern" >"$pattern_matches" || true
	match_count="$(wc -l <"$pattern_matches" | tr -d ' ')"
	if [[ "$match_count" -gt 1 ]]; then
		echo "  - $pattern ($match_count matches)"
	fi
done

if [[ "$errors" -ne 0 ]]; then
	echo
	echo "Agent docs integrity check failed."
	exit 1
fi

echo
echo "Agent docs integrity check passed."
