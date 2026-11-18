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
		# rg exits 1 when nothing matches; only real errors (exit 2) fail.
		rg -o -- "$regex" "$file" || [[ $? -eq 1 ]]
	else
		grep -oE -- "$regex" "$file" || true
	fi
}

# search_docs scans rules, skills, runbooks, memorials and workflow templates
# (never AGENTS.md).
search_docs() {
	local pattern="$1"
	if [[ "$has_rg" -eq 1 ]]; then
		rg -n -g '!**/speckit.*/SKILL.md' -- "$pattern" \
			.agents/rules \
			.agents/runbooks \
			.agents/memorials \
			.agents/workflows \
			.agents/skills/*/SKILL.md || [[ $? -eq 1 ]]
	else
		grep -nE -- "$pattern" \
			.agents/rules/*.md \
			.agents/runbooks/*.md \
			.agents/memorials/*.md \
			.agents/workflows/*.*js \
			.agents/skills/*/SKILL.md \
			| grep -vE '\.agents/skills/speckit\.[^/]+/SKILL\.md:' || true
	fi
}

# search_policy_docs adds AGENTS.md to the scanned set for the advisory
# duplicate-policy scan, so a rule copy of an AGENTS.md invariant is flagged.
search_policy_docs() {
	local pattern="$1"
	search_docs "$pattern"
	if [[ "$has_rg" -eq 1 ]]; then
		rg -n --with-filename -- "$pattern" AGENTS.md || [[ $? -eq 1 ]]
	else
		grep -nHE -- "$pattern" AGENTS.md || true
	fi
}

check_set_equality() {
	local name="$1"
	local expected_file="$2"
	local actual_file="$3"

	local index_name="${4:-AGENTS.md}"

	local missing_file extra_file dup_file unique_file
	missing_file="$(new_tmp)"
	extra_file="$(new_tmp)"
	dup_file="$(new_tmp)"
	unique_file="$(new_tmp)"

	uniq -d "$actual_file" >"$dup_file"
	uniq "$actual_file" >"$unique_file"
	comm -23 "$expected_file" "$unique_file" >"$missing_file"
	comm -13 "$expected_file" "$unique_file" >"$extra_file"

	if [[ -s "$dup_file" ]]; then
		echo "ERROR: ${name} lists entries more than once in ${index_name}:"
		sed 's/^/    - /' "$dup_file"
		errors=1
	fi

	if [[ -s "$missing_file" || -s "$extra_file" ]]; then
		echo "ERROR: ${name} is out of sync."
		if [[ -s "$missing_file" ]]; then
			echo "  Missing from ${index_name} index:"
			sed 's/^/    - /' "$missing_file"
		fi
		if [[ -s "$extra_file" ]]; then
			echo "  Listed in ${index_name} index but missing on disk:"
			sed 's/^/    - /' "$extra_file"
		fi
		errors=1
	elif [[ ! -s "$dup_file" ]]; then
		echo "OK: ${name} is in sync."
	fi
}

# check_file_index compares the *.md files of a directory with the bracketed
# entries of an index file. Arguments: name, directory, index file, entry regex,
# sed expression that turns a match into a repo-relative path, extra find args.
check_file_index() {
	local name="$1" dir="$2" index_file="$3" regex="$4" strip="$5"
	shift 5
	local on_disk indexed
	on_disk="$(new_tmp)"
	indexed="$(new_tmp)"
	find "$dir" -maxdepth 1 -type f -name '*.md' "$@" | sed 's#^\./##' | sort >"$on_disk"
	extract_matches "$regex" "$index_file" | sed -E "$strip" | sort >"$indexed"
	check_set_equality "$name" "$on_disk" "$indexed" "$index_file"
}

echo "Checking agent docs integrity..."

check_file_index "Rules index (.agents/rules)" .agents/rules AGENTS.md \
	"\\[\`\\.agents/rules/[^\`]+\`\\]" "s/^\\[\`(.+)\`\\]\$/\\1/"
check_file_index "Agents index (.agents/agents)" .agents/agents AGENTS.md \
	"\\[\`\\.agents/agents/[^\`]+\`\\]" "s/^\\[\`(.+)\`\\]\$/\\1/"
check_file_index "Commands index (.agents/commands)" .agents/commands AGENTS.md \
	"\\[\`\\.agents/commands/[^\`]+\`\\]" "s/^\\[\`(.+)\`\\]\$/\\1/"

# Workflow templates (.agents/workflows/*.js and *.mjs) are indexed like commands.
workflows_on_disk="$(new_tmp)"
workflows_indexed="$(new_tmp)"
if [[ -d .agents/workflows ]]; then
	find .agents/workflows -maxdepth 1 -type f \( -name '*.js' -o -name '*.mjs' \) | sed 's#^\./##' | sort >"$workflows_on_disk"
fi
extract_matches "\\[\`\\.agents/workflows/[^\`]+\`\\]" AGENTS.md \
	| sed -E "s/^\\[\`(.+)\`\\]\$/\\1/" \
	| sort >"$workflows_indexed"
check_set_equality "Workflows index (.agents/workflows)" "$workflows_on_disk" "$workflows_indexed"
if [[ -d .agents/workflows && "$(readlink .claude/workflows 2>/dev/null || true)" != "../.agents/workflows" ]]; then
	echo "ERROR: .claude/workflows must be a symlink to ../.agents/workflows."
	errors=1
fi

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

# Runbooks and memorials are not auto-loaded; each directory keeps its own
# README index of bare [`file.md`] entries, and AGENTS.md must point at both.
for doc_dir in .agents/runbooks .agents/memorials; do
	if [[ ! -f "$doc_dir/README.md" ]]; then
		echo "ERROR: $doc_dir/README.md is missing."
		errors=1
		continue
	fi
	check_file_index "README index ($doc_dir)" "$doc_dir" "$doc_dir/README.md" \
		"\\[\`[^\`/]+\\.md\`\\]" "s#^\\[\`(.+)\`\\]\$#${doc_dir}/\\1#" \
		! -name README.md
	if ! grep -qF "$doc_dir/README.md" AGENTS.md; then
		echo "ERROR: AGENTS.md does not reference $doc_dir/README.md."
		errors=1
	fi
done

alias_refs="$(new_tmp)"
search_docs '\.claude/(rules|skills|agents|commands|workflows)' >"$alias_refs" || true
if [[ -s "$alias_refs" ]]; then
	echo "ERROR: Found non-canonical .claude alias references in rules/skills/runbooks/memorials."
	sed 's/^/  /' "$alias_refs"
	echo "  Use .agents/* canonical paths in documentation."
	errors=1
else
	echo "OK: No non-canonical .claude alias references in rules/skills/runbooks/memorials."
fi

echo
echo "Advisory duplicate-policy scan (manual review signal):"
for pattern in \
	'commands and subcommands MUST remain hidden' \
	'container runtime runs Linux containers only' \
	'Virtual runtimes are not a sandbox' \
	"All new test functions MUST call \`t\.Parallel\(\)\`"
do
	pattern_matches="$(new_tmp)"
	search_policy_docs "$pattern" >"$pattern_matches" || true
	match_count="$(wc -l <"$pattern_matches" | tr -d ' ')"
	if [[ "$match_count" -gt 1 ]]; then
		echo "  - $pattern ($match_count matches)"
	fi
done

echo
echo "Validating skill package contracts..."
python3 scripts/check-skill-packages.py

echo
echo "Running skill package validator tests..."
python3 scripts/test_check_skill_packages.py

echo
echo "Validating review-docs deterministic contract..."
python3 .agents/skills/review-docs/scripts/review_docs.py validate

echo
echo "Running review-docs tooling tests..."
python3 .agents/skills/review-docs/scripts/test_review_docs.py

if [[ "$errors" -ne 0 ]]; then
	echo
	echo "Agent docs integrity check failed."
	exit 1
fi

echo
echo "Agent docs integrity check passed."
