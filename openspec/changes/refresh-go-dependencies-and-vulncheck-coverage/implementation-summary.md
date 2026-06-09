## Implementation Summary

Completed on 2026-06-09.

## Updated Versions

Root module:

- `charm.land/bubbletea/v2`: `v2.0.7`
- `github.com/coder/acp-go-sdk`: `v0.13.5`
- `github.com/rogpeppe/go-internal`: `v1.15.0`
- `golang.org/x/sys`: `v0.46.0`
- `golang.org/x/term`: `v0.44.0`
- `golang.org/x/net`: `v0.55.0`

Nested `tools/goplint` module:

- `golang.org/x/net`: `v0.55.0`

Intentionally unchanged in this bounded batch:

- `github.com/openai/openai-go`: `v1.12.0`
- `github.com/jonbaldie/go-mutesting/v2`: `v2.7.0`

OpenAI SDK v3 migration remains covered by
`openspec/changes/migrate-openai-sdk-v3/`. go-mutesting output-label follow-up
work remains covered by `openspec/changes/update-go-mutesting-output-labels/`.

## Vulnerability Scan Coverage

Added `scripts/govulncheck-all.sh` and `make vulncheck` so local and CI
vulnerability scans discover tracked Go modules and run `govulncheck ./...` in
each module. The scan logs each module before invoking govulncheck, including
`==> govulncheck: .` and `==> govulncheck: tools/goplint`.

Updated the CI govulncheck job to download modules for each tracked Go module
and invoke the shared `make vulncheck` path.

## Deferred Dependency Findings

The bounded update fixed the reachable nested `golang.org/x/net` vulnerability
without broad transitive churn. The following deprecated transitive findings
remain visible for separate migration work:

- Root: `cloud.google.com/go/pubsub v1.3.1`, deprecated in favor of
  `cloud.google.com/go/pubsub/v2`; the module notice says v1 receives bug fixes
  and security patches until 2026-12-31.
- Root: `github.com/cncf/udpa/go v0.0.0-20191209042840-269d4d468f6f`,
  deprecated in favor of `github.com/cncf/xds/go`.
- Root: `github.com/golang/protobuf v1.5.4`, deprecated in favor of
  `google.golang.org/protobuf`.
- Root: `golang.org/x/tools/go/expect v0.1.1-deprecated`.
- Root: `golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated`.
- `tools/goplint`: `github.com/golang/protobuf v1.5.0`, deprecated in favor of
  `google.golang.org/protobuf`.

## Verification

Passed:

- `go mod tidy -diff` in the root module.
- `go mod tidy -diff` in `tools/goplint`.
- `govulncheck ./...` in the root module.
- `govulncheck ./...` in `tools/goplint`.
- `make vulncheck`.
- `bash scripts/test_govulncheck_all.sh`.
- `go test ./internal/acpclient`.
- `go test ./internal/tui ./internal/app/commandadapters ./internal/container`.
- `go test ./...` in `tools/goplint`.
- `make test-cli`.
- `make check-agent-docs`.
- `make test-scripts`.
- `make test`.
- `make lint`.

Also run:

- `make lint-scripts`, which exited successfully after reporting that
  `shellcheck` is not installed in the current environment.
