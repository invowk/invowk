#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
EVENT="${GOPLINT_SOUNDNESS_EVENT:-pre_commit}"
BASE="${GOPLINT_SOUNDNESS_BASE:-}"
HEAD="${GOPLINT_SOUNDNESS_HEAD:-HEAD}"
route_args=(
  -root ../..
  -manifest spec/soundness-ownership.v1.json
  -event "$EVENT"
  -format profile
)
if [[ "$EVENT" == "pre_commit" ]]; then
  route_args+=(-staged)
elif [[ -n "$BASE" ]]; then
  route_args+=(-base "$BASE" -head "$HEAD")
fi

cd "$MODULE_DIR"
profile="$(go run ./cmd/soundness-profile "${route_args[@]}")"
echo "Selected goplint soundness profile: $profile"
go run ./cmd/soundness-gate \
  -root ../.. \
  -manifest tools/goplint/spec/soundness-gate.v1.json \
  -profile "$profile"
