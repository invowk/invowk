# Proposal: Minimum `invowk_version` Constraint & Fail-Closed CWD Discovery

## Why

Teams pin `invowk` in CI while local workstations or toolboxes may run older binaries, and project repositories (such as `cst.invowkmod/`) can have local syntax/schema edits. Currently, `#Invowkmod` and `#Invowkfile` are `close({...})` schemas decoded in a single pass—so an older binary fails with an opaque `field not allowed` error—and invalid current-directory (`cwd`) files are skipped during discovery, allowing global commands (`~/.invowk/cmds`) to shadow broken local projects or surface misleading `command not found` messages.

## What Changes

- Add an optional `invowk_version` SemVer constraint field to `#Invowkmod` (`invowkmod.cue`) and top-level `#Invowkfile` (`invowkfile.cue`) using the existing `invowkmod.SemVerConstraint` grammar.
- Evaluate `invowk_version` in a lightweight schema-less CUE pre-pass **before** `cueutil.ParseAndDecodeCompiled` so old binaries fail cleanly with an actionable upgrade message even when the file also uses newer schema fields. Resolve binary version via `getVersionString`/ldflags + `debug.ReadBuildInfo()` before treating a build as `dev` (true `(devel)` builds warn and continue).
- Append a contextual hint (`this file may require a newer version of invowk`) whenever CUE strict schema decoding returns `field not allowed`.
- **BREAKING**: Fail closed (exit status `1` with the underlying parse, schema, or `invowk_version` error) whenever a current-directory source (`./invowkfile.cue` or `./*.invowkmod`) fails validation while `--ivk-no-cwd` / `INVOWK_NO_CWD` is off, preventing silent shadowing by lower-precedence global or included commands. Non-cwd sources (`~/.invowk/cmds/`, non-target `includes`) retain resilient skip + `SkippedSourcesError` reporting.

## Migration notes

- Workspaces containing an invalid `./invowkfile.cue` or invalid sibling `./*.invowkmod` will now fail `invowk cmd` immediately with exit status `1` and the file's parse/version error instead of falling back to global commands in `~/.invowk/cmds` or reporting `command not found`. Fix the broken local file, or pass `--ivk-no-cwd` / `INVOWK_NO_CWD=1` to intentionally bypass current-directory sources.
- Existing resilient-skip testscript suites (`tests/cli/testdata/{virtual_diagnostics_footer,virtual_verbose,native_verbose,virtual_not_found_skipped,native_not_found_skipped}.txtar`) that currently place `broken.invowkmod` in `$WORK` (cwd) will be migrated to place the broken module in a non-cwd source (`~/.invowk/cmds` / `includes`) where skipped-source handling applies.

## Capabilities

### New Capabilities

- `minimum-invowk-version`: Defines `invowk_version` SemVer constraint declaration on `invowkmod.cue` and `invowkfile.cue`, pre-pass evaluation before strict schema decoding, `dev` build handling via `debug.ReadBuildInfo()`, and `field not allowed` upgrade hints.

### Modified Capabilities

- `cli-exit-codes`: Updates `A command missing because a source failed to load exits with status 1` so current-directory parse/version failures abort immediately with status `1` and their real error, while non-cwd skipped sources retain `SkippedSourcesError` behavior.
- `cwd-discovery-scope`: Adds fail-closed behavior for invalid current-directory sources when `--ivk-no-cwd` is off, complementing the switch-on skip behavior already specified.

## Impact

- Schema & Parsers: `pkg/invowkmod/invowkmod_schema.cue`, `pkg/invowkmod/parse.go`, `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkfile/invowkfile.go`, `pkg/invowkfile/parse.go`.
- Discovery & CLI: `internal/discovery/discovery_files.go`, `internal/discovery/discovery_commands.go`, `cmd/invowk/root.go` (version helper sharing), `cmd/invowk/cmd.go`, `cmd/invowk/usage_error.go`.
- Tests & Docs: schema-sync tests, pre-pass/SemVer tests, discovery & testscript fail-closed tests (`tests/cli/`), and `website/` schema/CLI docs.
