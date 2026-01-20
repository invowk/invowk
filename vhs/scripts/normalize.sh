#!/usr/bin/env bash
# normalize.sh - Normalize variable output for deterministic comparison
#
# This script filters out variable content (timestamps, paths, hostnames, versions)
# to make VHS test output deterministic across different environments and runs.
#
# Usage: ./normalize.sh <input_file>

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <input_file>" >&2
    exit 1
fi

input_file="$1"

if [[ ! -f "$input_file" ]]; then
    echo "Error: File not found: $input_file" >&2
    exit 1
fi

sed -E \
    -e 's/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[A-Z]?/[TIMESTAMP]/g' \
    -e 's|/home/[a-zA-Z0-9_-]+|[HOME]|g' \
    -e 's|/var/home/[a-zA-Z0-9_-]+|[HOME]|g' \
    -e 's|/Users/[a-zA-Z0-9_-]+|[HOME]|g' \
    -e 's|/tmp/[a-zA-Z0-9._-]+|[TMPDIR]|g' \
    -e 's|/var/tmp/[a-zA-Z0-9._-]+|[TMPDIR]|g' \
    -e 's/hostname: [a-zA-Z0-9._-]+/hostname: [HOSTNAME]/g' \
    -e 's/invowk v[0-9]+\.[0-9]+\.[0-9]+[^ ]*/invowk [VERSION]/g' \
    -e 's/invowk version [0-9]+\.[0-9]+\.[0-9]+[^ ]*/invowk version [VERSION]/g' \
    -e 's/USER = '"'"'[a-zA-Z0-9_-]+'"'"'/USER = '"'"'[USER]'"'"'/g' \
    -e 's/HOME = '"'"'[^'"'"']+'"'"'/HOME = '"'"'[HOME]'"'"'/g' \
    -e 's/PATH = '"'"'[^'"'"']+'"'"'/PATH = '"'"'[PATH]'"'"'/g' \
    -e 's/PATH = '"'"'[^'"'"']+'"'"' \(truncated\)/PATH = '"'"'[PATH]'"'"' (truncated)/g' \
    -e 's/\x1b\[[0-9;]*[a-zA-Z]//g' \
    -e '/^$/d' \
    "$input_file" | uniq
