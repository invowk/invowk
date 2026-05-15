#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

bencher_api_token_subject() {
	local token="${BENCHER_API_TOKEN:?BENCHER_API_TOKEN is required}"
	local payload
	local subject

	if [[ "$token" != *.*.* ]]; then
		echo "::error::BENCHER_API_TOKEN must be a JWT with three segments." >&2
		return 1
	fi

	payload="${token#*.}"
	payload="${payload%%.*}"
	case $((${#payload} % 4)) in
		0) ;;
		2) payload="${payload}==" ;;
		3) payload="${payload}=" ;;
		*)
			echo "::error::BENCHER_API_TOKEN has an invalid JWT payload length." >&2
			return 1
			;;
	esac

	if ! subject="$(printf '%s' "$payload" |
		tr '_-' '/+' |
		base64 -d 2>/dev/null |
		jq -er '.sub | select(type == "string" and length > 0)' 2>/dev/null)"; then
		echo "::error::BENCHER_API_TOKEN JWT payload must include a non-empty sub claim." >&2
		return 1
	fi

	printf '%s\n' "$subject"
}

bencher_registry_login() {
	local subject

	subject="$(bencher_api_token_subject)"
	echo "$BENCHER_API_TOKEN" | docker login registry.bencher.dev -u "$subject" --password-stdin
}
