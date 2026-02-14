# Plan: Add t.Parallel() to 19+ Test Files (Phase 1)

## Overview

Add `t.Parallel()` to test files that have no global state dependencies, following the project's testing rules:
- `t.Parallel()` as first statement in each `Test*` function
- `t.Parallel()` as first statement in each `t.Run()` callback
- If ANY subtest cannot be parallel, the parent ALSO cannot be parallel (tparallel linter rule)

## File-by-File Plan

### Group 1: cmd/invowk/ (4 files)

**1. `cmd/invowk/cmd_args_test.go`** — 12 Test* functions, all pure table-driven
- Add `t.Parallel()` to all 12 top-level tests
- Add `t.Parallel()` to all `t.Run()` callbacks in table-driven tests

**2. `cmd/invowk/cmd_source_test.go`** — 6 Test* functions, pure function tests
- Add `t.Parallel()` to all 6 top-level tests
- Add `t.Parallel()` to `t.Run()` callbacks in `TestNormalizeSourceName`

**3. `cmd/invowk/cmd_deps_caps_env_test.go`** — 14 Test* functions, local struct creation
- Add `t.Parallel()` to all 14 top-level tests
- No subtests in this file

**4. `cmd/invowk/cmd_deps_filepath_test.go`** — ~20 Test* functions, uses t.TempDir()
- Add `t.Parallel()` to all top-level tests
- No subtests in this file

### Group 2: internal/config/ (1 file)

**5. `internal/config/sync_test.go`** — 11 Test* functions, CUE schema validation
- Add `t.Parallel()` to all 11 top-level tests
- Add `t.Parallel()` to all `t.Run()` callbacks in table-driven tests

### Group 3: internal/discovery/ (1 file)

**6. `internal/discovery/validation_test.go`** — 10 Test* functions, pure function tests
- Add `t.Parallel()` to all 10 top-level tests
- No subtests in this file

### Group 4: internal/issue/ (2 files)

**7. `internal/issue/actionable_test.go`** — 10 Test* functions, pure error type tests
- Add `t.Parallel()` to all 10 top-level tests
- Add `t.Parallel()` to all `t.Run()` callbacks

**8. `internal/issue/issue_test.go`** — PARTIAL (12 of 16 tests)
- Add `t.Parallel()` to: `TestId_Constants`, `TestIssue_Id`, `TestIssue_MarkdownMsg`, `TestIssue_DocLinks`, `TestIssue_ExtLinks`, `TestGet`, `TestValues`, `TestMarkdownMsg_Type`, `TestHttpLink_Type`, `TestAllIssuesHaveContent`, `TestIssuesMapCompleteness`, `TestIssueTemplates_NoStaleGuidance`
- SKIP (touch `render` global): `TestIssue_Render`, `TestIssue_Render_WithLinks`, `TestIssue_Render_NoLinks`, `TestAllIssuesAreRenderable`
- Add `t.Parallel()` to `t.Run()` in `TestGet` (table-driven)

### Group 5: internal/runtime/ (3 files)

**9. `internal/runtime/env_builder_test.go`** — 6 Test* functions
- Add `t.Parallel()` to all 6 top-level tests

**10. `internal/runtime/interactive_test.go`** — 5 Test* functions
- Add `t.Parallel()` to all 5 top-level tests
- Add `t.Parallel()` to all `t.Run()` callbacks

**11. `internal/runtime/runtime_virtual_integration_test.go`** — 2 Test* + subtests
- `TestVirtualRuntime_Integration` has subtests calling named helper functions. Add `t.Parallel()` to the parent AND inside each helper function (they each create their own `t.TempDir()`)
- `TestVirtualRuntime_ScriptFileFromSubdir` — Add `t.Parallel()` (uses own `t.TempDir()`)

### Group 6: internal/sshserver/ (1 file)

**12. `internal/sshserver/server_test.go`** — 14 Test* functions
- Add `t.Parallel()` to all 14 top-level tests
- Add `t.Parallel()` to `t.Run()` in `TestServerStateString` and `TestIsClosedConnError`
- Each test creates its own server with port 0 (auto-select), so parallel is safe

### Group 7: internal/testutil/ (2 files)

**13. `internal/testutil/clock_test.go`** — 13 Test* functions
- Add `t.Parallel()` to all 13 top-level tests
- No subtests

**14. `internal/testutil/invowkfiletest/command_test.go`** — 16 Test* functions
- Add `t.Parallel()` to all 16 top-level tests
- No subtests

### Group 8: pkg/invowkfile/ (1 file)

**15. `pkg/invowkfile/validation_types_test.go`** — 9 Test* functions
- Add `t.Parallel()` to all 9 top-level tests
- Add `t.Parallel()` to all `t.Run()` callbacks

### Group 9: pkg/invowkmod/ (1 file)

**16. `pkg/invowkmod/operations_packaging_test.go`** — 2 Test* functions with subtests
- `TestArchive`: Has 3 subtests. The "archive with default output path" subtest uses `testutil.MustChdir` (process-wide). Per tparallel rule, TestArchive CANNOT be parallel.
- `TestUnpack`: Has 6 subtests, all safe. Add `t.Parallel()` to parent and all subtests.

### Group 10: pkg/platform/ (1 file)

**17. `pkg/platform/windows_test.go`** — 2 Test* functions
- Add `t.Parallel()` to both
- Add `t.Parallel()` to `t.Run()` in `TestIsWindowsReservedName`

### Group 11: internal/provision/ (3 files)

**18. `internal/provision/helpers_test.go`** — 16 Test* functions
- Add `t.Parallel()` to all 16 top-level tests
- No subtests

**19. `internal/provision/provisioner_test.go`** — ~8 Test* functions with subtests
- Add `t.Parallel()` to all top-level tests and subtests

**20. `internal/provision/layer_provisioner_test.go`** — All tests EXCEPT `TestDefaultConfigReadsTagSuffixFromEnv`
- SKIP: `TestDefaultConfigReadsTagSuffixFromEnv` (uses `os.Setenv`)
- Add `t.Parallel()` to all other tests and their subtests

## Verification Strategy

After modifying each package group, run:
```bash
go test -race -count=2 -v ./path/to/package/...
```

## Commit Strategy

One commit per package group:
1. `test(cli): add t.Parallel() to cmd args/source/deps test files`
2. `test(config): add t.Parallel() to sync_test.go`
3. `test(discovery): add t.Parallel() to validation_test.go`
4. `test(issue): add t.Parallel() to actionable and partial issue tests`
5. `test(runtime): add t.Parallel() to env_builder, interactive, and integration tests`
6. `test(sshserver): add t.Parallel() to server_test.go`
7. `test(testutil): add t.Parallel() to clock and command builder tests`
8. `test(invowkfile): add t.Parallel() to validation_types_test.go`
9. `test(invowkmod): add t.Parallel() to operations_packaging_test.go`
10. `test(platform): add t.Parallel() to windows_test.go`
11. `test(provision): add t.Parallel() to helpers, provisioner, and layer tests`
