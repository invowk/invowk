#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../../../.." && pwd)"
cd "${repo_root}"

isvalid_file="$(mktemp)"
prim_file="$(mktemp)"
alias_file="$(mktemp)"
catalog_file="$(mktemp)"

cleanup() {
  rm -f "${isvalid_file}" "${prim_file}" "${alias_file}" "${catalog_file}"
}
trap cleanup EXIT

rg -n "func \([^)]+\) IsValid\(\) \(bool, \[\]error\)" pkg internal -g '*.go' |
  awk -F: '
    {
      file=$1
      line=$2
      txt=$0
      sub(/^.*func \(/, "", txt)
      sub(/\) IsValid\(\) \(bool, \[\]error\).*/, "", txt)
      n=split(txt, a, " ")
      t=a[n]
      gsub(/^\*/, "", t)
      print t "|" file "|" line
    }
  ' |
  sort -u > "${isvalid_file}"

awk '
  function opens(s) { gsub(/[^\{]/, "", s); return length(s) }
  function closes(s) { gsub(/[^\}]/, "", s); return length(s) }
  function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); return s }

  BEGIN { inTypeBlock=0; braceDepth=0 }

  FNR == 1 {
    inTypeBlock=0
    braceDepth=0
  }

  {
    line=$0
    if (line ~ /^type[[:space:]]*\([[:space:]]*$/) {
      inTypeBlock=1
      braceDepth=0
      next
    }

    if (inTypeBlock == 1 && braceDepth == 0 && line ~ /^\)[[:space:]]*$/) {
      inTypeBlock=0
      next
    }

    if (inTypeBlock == 1) {
      if (braceDepth == 0 && match(line, /^[[:space:]]*([A-Z][A-Za-z0-9_]*)[[:space:]]+(.+)$/, m)) {
        name=m[1]
        rhs=trim(m[2])
        if (rhs ~ /^(string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|bool|float32|float64|rune|byte|time\.Duration|types\.[A-Z][A-Za-z0-9_]*)$/) {
          print name "|" rhs "|" FILENAME ":" FNR
        }
      }
      braceDepth += opens(line)
      braceDepth -= closes(line)
      if (braceDepth < 0) {
        braceDepth = 0
      }
      next
    }

    if (match(line, /^type[[:space:]]+([A-Z][A-Za-z0-9_]*)[[:space:]]+(.+)$/, m)) {
      name=m[1]
      rhs=trim(m[2])
      if (rhs ~ /^(string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|bool|float32|float64|rune|byte|time\.Duration|types\.[A-Z][A-Za-z0-9_]*)$/) {
        print name "|" rhs "|" FILENAME ":" FNR
      }
    }
  }
' $(rg --files pkg internal -g '*.go' | sort) |
  sort -u > "${prim_file}"

awk '
  BEGIN { inTypeBlock=0; braceDepth=0 }

  function opens(s) { gsub(/[^\{]/, "", s); return length(s) }
  function closes(s) { gsub(/[^\}]/, "", s); return length(s) }
  function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); return s }

  FNR == 1 {
    inTypeBlock=0
    braceDepth=0
  }

  {
    line=$0
    if (line ~ /^type[[:space:]]*\([[:space:]]*$/) {
      inTypeBlock=1
      braceDepth=0
      next
    }

    if (inTypeBlock == 1 && braceDepth == 0 && line ~ /^\)[[:space:]]*$/) {
      inTypeBlock=0
      next
    }

    if (inTypeBlock == 1) {
      if (braceDepth == 0 && match(line, /^[[:space:]]*([A-Z][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*([^[:space:]].*)$/, m)) {
        print m[1] "|" trim(m[2]) "|" FILENAME ":" FNR
      }
      braceDepth += opens(line)
      braceDepth -= closes(line)
      if (braceDepth < 0) {
        braceDepth = 0
      }
      next
    }

    if (match(line, /^type[[:space:]]+([A-Z][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*([^[:space:]].*)$/, m)) {
      print m[1] "|" trim(m[2]) "|" FILENAME ":" FNR
    }
  }
' $(rg --files pkg internal -g '*.go' | sort) |
  sort -u > "${alias_file}"

awk -F'|' '
  NR == FNR {
    split($3, parts, ":")
    primitive[$1 "|" parts[1]] = 1
    next
  }
  {
    key=$1 "|" $2
    kind = (primitive[key] ? "primitive-wrapper" : "composite-validator")
    print $1 "|" $2 "|" $3 "|" kind
  }
' "${prim_file}" "${isvalid_file}" |
  sort -t'|' -k2,2 -k1,1 > "${catalog_file}"

total="$(wc -l < "${catalog_file}" | tr -d ' ')"
primitive_count="$(awk -F'|' '$4=="primitive-wrapper" {c++} END{print c+0}' "${catalog_file}")"
composite_count="$(awk -F'|' '$4=="composite-validator" {c++} END{print c+0}' "${catalog_file}")"
primitive_all_count="$(wc -l < "${prim_file}" | tr -d ' ')"
alias_count="$(wc -l < "${alias_file}" | tr -d ' ')"

cat <<HEADER
# Invowk Value-Type Catalog

This catalog is generated from repository source and documents current type-system coverage.

## Coverage Summary

- \`IsValid()\` value types: ${total}
- Primitive-wrapper + \`IsValid()\` types: ${primitive_count}
- Composite validator + \`IsValid()\` types: ${composite_count}
- Primitive-wrapper declarations (all): ${primitive_all_count}
- Alias/re-export type declarations: ${alias_count}

## All Types With \`IsValid() (bool, []error)\`

| Type | Kind | Source |
| --- | --- | --- |
HEADER

awk -F'|' '{printf("| `%s` | `%s` | `%s:%s` |\n", $1, $4, $2, $3)}' "${catalog_file}"

cat <<MID

## Primitive-Wrapper Value Types (All Declarations)

| Type | Underlying | Source |
| --- | --- | --- |
MID

awk -F'|' '{printf("| `%s` | `%s` | `%s` |\n", $1, $2, $3)}' "${prim_file}"

cat <<TAIL

## Alias/Re-export Types

| Alias | Target | Source |
| --- | --- | --- |
TAIL

awk -F'|' '{printf("| `%s` | `%s` | `%s` |\n", $1, $2, $3)}' "${alias_file}"

cat <<END_NOTE

## Regeneration

Run:

\`\`\`bash
.agents/skills/invowk-typesystem/scripts/extract_value_types.sh > .agents/skills/invowk-typesystem/references/type-catalog.md
\`\`\`
END_NOTE
