#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# shellcheck source=bencher-registry-login.sh
source "$SCRIPT_DIR/bencher-registry-login.sh"

jwt_for_subject() {
	local subject="$1"
	local payload

	payload="$(jq -cn --arg subject "$subject" '{sub: $subject}' | base64 -w0 | tr '+/' '-_' | tr -d '=')"
	printf 'header.%s.signature\n' "$payload"
}

assert_eq() {
	local desc="$1"
	local expected="$2"
	local actual="$3"

	if [[ "$expected" != "$actual" ]]; then
		printf 'FAIL: %s\n  expected: %s\n  actual:   %s\n' "$desc" "$expected" "$actual" >&2
		exit 1
	fi
}

assert_rejects() {
	local desc="$1"
	shift

	if "$@" >/dev/null 2>&1; then
		printf 'FAIL: %s\n  command unexpectedly succeeded\n' "$desc" >&2
		exit 1
	fi
}

test_subject_extraction() {
	local token
	local subject

	token="$(jwt_for_subject 'ci@example.com')"
	subject="$(BENCHER_API_TOKEN="$token" bencher_api_token_subject)"
	assert_eq "extracts JWT subject" "ci@example.com" "$subject"
}

test_invalid_tokens_fail() {
	assert_rejects "rejects malformed JWT" env BENCHER_API_TOKEN="not-a-jwt" bash -c \
		'source "$BENCHER_LOGIN_SCRIPT"; bencher_api_token_subject'

	assert_rejects "rejects JWT without subject" env BENCHER_API_TOKEN="header.$(printf '{}' | base64 -w0).signature" bash -c \
		'source "$BENCHER_LOGIN_SCRIPT"; bencher_api_token_subject'
}

test_registry_login_uses_subject() {
	local tmp
	local token
	local args_file
	local stdin_file
	local mock_docker

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	args_file="$tmp/docker.args"
	stdin_file="$tmp/docker.stdin"
	mock_docker="$tmp/docker"

	cat >"$mock_docker" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$DOCKER_ARGS_FILE"
cat > "$DOCKER_STDIN_FILE"
MOCK
	chmod +x "$mock_docker"

	token="$(jwt_for_subject 'registry@example.com')"
	DOCKER_ARGS_FILE="$args_file" \
		DOCKER_STDIN_FILE="$stdin_file" \
		PATH="$tmp:$PATH" \
		BENCHER_API_TOKEN="$token" \
		bencher_registry_login >/dev/null

	assert_eq "docker login args" "login registry.bencher.dev -u registry@example.com --password-stdin" "$(cat "$args_file")"
	assert_eq "docker login stdin" "$token" "$(cat "$stdin_file")"
}

test_subject_extraction
export BENCHER_LOGIN_SCRIPT="$SCRIPT_DIR/bencher-registry-login.sh"
test_invalid_tokens_fail
test_registry_login_uses_subject

echo "bencher registry login tests passed"
