## 1. Dependency Updates

- [x] 1.1 Re-run the dependency audit commands for the root module and `tools/goplint` to confirm the selected update set is still current.
- [x] 1.2 Update `tools/goplint` so `golang.org/x/net` resolves to `v0.55.0` or newer, then run nested `go mod tidy`.
- [x] 1.3 Update the root direct dependencies `charm.land/bubbletea/v2`, `github.com/coder/acp-go-sdk`, `github.com/rogpeppe/go-internal`, `golang.org/x/sys`, and `golang.org/x/term` to the audited versions.
- [x] 1.4 Run root `go mod tidy` and confirm `go mod tidy -diff` is clean in both Go modules.

## 2. Vulnerability Scan Coverage

- [x] 2.1 Add a shared local command or script that discovers tracked Go modules and runs `govulncheck ./...` in each module.
- [x] 2.2 Update the CI `govulncheck` job to invoke the shared all-modules scan path instead of scanning only the root module.
- [x] 2.3 Update `.agents/skills/dep-audit`, `.agents/rules/commands.md`, or related guidance if the user-facing vulnerability scan command changes.
- [x] 2.4 Ensure vulnerability scan logs identify the module being scanned before each `govulncheck` invocation.

## 3. Verification

- [x] 3.1 Run `govulncheck ./...` from the root module and from `tools/goplint`.
- [x] 3.2 Run the new shared all-modules vulnerability scan command locally.
- [x] 3.3 Run targeted ACP client tests after the ACP SDK patch update.
- [x] 3.4 Run targeted TUI and terminal-sensitive tests after Bubble Tea, `x/sys`, and `x/term` updates.
- [x] 3.5 Run the CLI testscript suite or equivalent focused CLI tests after the `go-internal` update.
- [x] 3.6 Run `make test`, `make lint`, and `make check-agent-docs` if agent-facing guidance changed.

## 4. Review

- [x] 4.1 Confirm deprecated transitive modules found by the audit are either removed by the bounded update or documented as deferred findings.
- [x] 4.2 Confirm OpenAI SDK v3 and go-mutesting remain separate workstreams and are not pulled into this dependency batch.
- [x] 4.3 Record the final updated versions and verification results in the implementation summary.
