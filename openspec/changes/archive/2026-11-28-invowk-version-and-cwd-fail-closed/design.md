# Design: Minimum `invowk_version` Constraint & Fail-Closed CWD Discovery

## Context

Current behavior in `main`:
- `pkg/invowkmod/invowkmod_schema.cue:41-82` (`#Invowkmod`) and `pkg/invowkfile/invowkfile_schema.cue:597-624` (`#Invowkfile`) are `close({...})` CUE definitions decoded in one pass via `cueutil.ParseAndDecodeCompiled` (`pkg/invowkmod/parse.go:46-50`, `pkg/invowkfile/parse.go:72-76`). Any unknown field added by a newer Invowk release fails with CUE `field not allowed`.
- `cmd/invowk/root.go:18-24,118-128` sets `Version = "dev"` by default and resolves `debug.ReadBuildInfo().Main.Version` when `version == "dev"`. Packages under `pkg/` cannot read `cmd.Version` directly without injecting/exporting the current Invowk version.
- `internal/discovery/discovery_files.go:160-177` discovers `./invowkfile.cue` (`SourceCurrentDir`) and sibling `./*.invowkmod` directories (`SourceModule` via `discoverCurrentDirModule` / `discoverModulesInDirWithDiagnostics`).
- `internal/discovery/discovery_commands.go:229-240` records parse errors as `CodeInvowkfileParseSkipped` warnings (`SeverityWarning`) instead of failing, and `cmd/invowk/usage_error.go:48-117` turns missing commands into `SkippedSourcesError` (exit `1`) only if the requested command was not found anywhere else—meaning a broken `./invowkfile.cue` is silently shadowed if `~/.invowk/cmds` defines the same command name.

## Goals / Non-Goals

**Goals:**
- Catch minimum Invowk version incompatibilities cleanly before strict `close({...})` decoding rejects newer schema fields.
- Properly distinguish `go install ...@vX.Y.Z` binaries (`debug.ReadBuildInfo().Main.Version`) from local `(devel)` builds before applying dev-build leniency.
- Make broken local workspace definitions (`./invowkfile.cue` or `./*.invowkmod`) fail loudly and immediately with exit `1` when `--ivk-no-cwd` is off, while keeping global/non-cwd sources resilient.

**Non-Goals:**
- Introducing CUE schema migration or automatic downgrading of newer `invowkfile.cue` syntax.
- Making broken `~/.invowk/cmds/` modules block unrelated projects when the user is running commands from a healthy local project.

## Decisions

### D1. Schema-less CUE pre-pass for `invowk_version` + `field not allowed` hint
- Add optional `invowk_version?: #SemVerConstraint` (matching the existing SemVer constraint pattern in `pkg/invowkmod/invowkmod_schema.cue:24`) to `#Invowkmod` (`invowkmod_schema.cue:41`) and `#Invowkfile` (`invowkfile_schema.cue:597`), with matching `InvowkVersion invowkmod.SemVerConstraint` (`json:"invowk_version,omitempty"`) fields on Go structs (`pkg/invowkmod/invowkmod.go`, `pkg/invowkfile/invowkfile.go:56`).
- Before running `cueutil.ParseAndDecodeCompiled` in `ParseInvowkmodBytes` (`pkg/invowkmod/parse.go:40`) and `parseBytes` (`pkg/invowkfile/parse.go:62`), run a tiny schema-less CUE compile/lookup on the input bytes looking up `invowk_version` (supporting CUE expressions/variables/comments, unlike fragile regex scraping, and rejecting malformed/non-string `invowk_version` values cleanly).
- Normalize leading `"v"` and build metadata from `debug.ReadBuildInfo().Main.Version` before evaluating `invowk_version` against the active Invowk version (checking ldflags `Version` first and `debug.ReadBuildInfo().Main.Version` second before treating as true `(devel)`):
  - If true `(devel)` build: emit a warning/diagnostic and continue to schema decode.
  - Otherwise: return a typed `UnsupportedInvowkVersionError` (with required constraint, current version, and file path) so the caller exits `1` without hitting confusing downstream `field not allowed` noise.
- If the subsequent strict `ParseAndDecodeCompiled` fails with a CUE `field not allowed` error, wrap/annotate the error message with `"hint: this file may require a newer version of invowk"`.

### D2. Scoped fail-closed discovery in the `invowk cmd` consumer layer
- Tag whether each discovered file/diagnostic came from the current working directory (`d.baseDir` — i.e., `discoverCurrentDirModule`, `discoverInDir(d.baseDir, ...)`, or `discoverModulesInDirWithDiagnostics(d.baseDir)` in `internal/discovery/discovery_files.go:160-177`).
- **Important layering constraint**: `DiscoverCommandSet` and `DiscoverAndValidateCommandSet` (`internal/discovery/discovery_commands.go:206-317`) MUST continue returning all collected files and tagged diagnostics without aborting early, because `invowk validate` (`cmd/invowk/validate.go:152-157`) relies on `DiscoverAndValidateCommandSet` to report all workspace diagnostics across files rather than stopping at the first broken cwd file.
- When `--ivk-no-cwd` / `INVOWK_NO_CWD` is OFF and any tagged `cwd` source (`./invowkfile.cue` or `./*.invowkmod`) failed parse, schema, or `invowk_version` validation:
  - In `invowk cmd` execution/resolution/listing (`cmd/invowk/cmd.go`, `internal/app/commandsvc/`), **fail closed immediately** with exit status `1` and surface the underlying parse/version error directly on stderr instead of skipping it and falling back to `~/.invowk/cmds` or returning `"command not found"`.
  - Keep policy skips (such as reserved module name or symlink-root skips) and **non-cwd** load failures (`~/.invowk/cmds`, `includes` when not the active target) on the existing diagnostic + `SkippedSourcesError` path (`cmd/invowk/usage_error.go:93-117`) so broken global modules never brick healthy local workspaces.
  - Migrate existing resilient-skip testscript fixtures (`tests/cli/testdata/{virtual_diagnostics_footer,virtual_verbose,native_verbose,virtual_not_found_skipped,native_not_found_skipped}.txtar`) so their `broken.invowkmod` lives in a non-cwd source (`~/.invowk/cmds` or `includes`) rather than `$WORK`.

## Risks / Trade-offs

- **Risk**: A repository has an unrelated broken `scratch.invowkmod` sitting in `cwd` while a user tries to run a global command in `~/.invowk/cmds`.
  -> **Mitigation**: Intentional per fail-closed cwd semantics; `--ivk-no-cwd` / `INVOWK_NO_CWD=1` (`openspec/specs/cwd-discovery-scope`) explicitly exists for running global/included commands without loading `cwd` sources.
- **Risk**: `ParseBytes` / `ParseInvowkmodBytes` unit tests run without `cmd` ldflags set.
  -> **Mitigation**: Centralize version detection in a clean helper (e.g., `pkg/version` or build-info provider with test override) defaulting cleanly for `(devel)` test binaries while respecting injected/ldflags versions.

## Maintainer decisions

- **2026-11-27**: Confirmed `invowk_version` in both `invowkmod.cue` and `invowkfile.cue` evaluated in a lightweight schema-less CUE pre-pass, checking `debug.ReadBuildInfo()` before treating a build as `dev`, adding the `field not allowed` upgrade hint, and scoping fail-closed exit-1 errors strictly to `cwd` sources when `--ivk-no-cwd` is off.

## Open questions

- Confirm whether `invowk_version` pre-pass checks also run inside `invowk module tidy`/`sync` or `invowk validate` uniformly (yes if placed inside `ParseInvowkmodBytes` / `parseBytes`, with `invowk validate` surfacing the error cleanly per file).
