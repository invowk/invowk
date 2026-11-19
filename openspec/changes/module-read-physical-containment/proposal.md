## Why

Two recorded formal findings of `ModulePathContainment` are real escapes from a module's directory (SC-01):

- **F13:** a module's `script.file` and a custom-check `file` are contained only lexically. A link under `invowk_modules/` (which `Load` does not scan) can make Invowk read a file outside the module.
- **F17:** a module's `env.files` entry has no containment at all, so the same link leaks an outside file into the command's environment.

Both have model counterexamples and Go replays that reproduce them today. This change closes both with a physically contained read, as maintainer decision 46 (2026-11-19) directs.

## What Changes

- **Module script reads are physically contained.** In module context, `Implementation.ResolveScriptWithFSAndModule` and `CustomCheckScript.ResolveWithFSAndModule` first check, through an `os.Root` opened on the module directory, that the script path resolves inside the module. An escaping path or an unresolvable link (dangling or a cycle) is denied with `ErrScriptPathTraversal`; an operating-system error that keeps the path itself from being inspected (for example `EACCES` on a directory, where `Lstat` fails too) is returned as a script read error. Either way the injected reader is not called.
  - The check runs whatever reader the caller injects, so every production path is covered: `invowk cmd` on every runtime, custom checks, and `invowk validate`'s interpreter diagnostics.
- **Module env files are physically contained.** `env.files` of an invowkfile loaded from a module are read through `os.Root`. In those reads:
  - an absolute path is denied;
  - an escaping link fails;
  - a dangling link or a link cycle fails;
  - an escape reported by the contained read is a containment error (`ErrEnvFileOutsideModule`), never a missing file;
  - an optional `?` file is skipped only when the contained read reports that nothing exists at that path.
- **Plain invowkfiles keep their current env-file behaviour.** They are not module context, so a symlinked `.env` (for example a link to `~/secrets/.env`) keeps working.
- **Links are followed only when their target is relative.** A relative link whose target stays inside the module is still followed. A link with an absolute target is denied even when that target is inside the module, because `os.Root` rejects absolute link targets. The model's correspondence records this deliberate strictness.
- **Removed `pkg/invowkmod` API:** `Module.ValidateScriptPath` and its helper `checkSymlinkSafety` are deleted. They had no production caller.
- **Formal artefacts:**
  - the model's `accepts` uses physical containment for script and env reads;
  - the golden is regenerated and replayed twice, with relative and with absolute link targets; the absolute pass classifies in-module absolute-target links explicitly as denied;
  - the shrink-only `[correspondence] unresolved_elements` and `[triage] binding_gaps` lists lose the entries that the removed or renamed rows make stale; nothing is added to them;
  - the F13 and F17 replays are inverted and renamed;
  - each finding command becomes a `mutantPreFixF<n>…` regression mutant;
  - the README Findings rows become "Fixed".
- **F16 (Windows junction) is re-decided from evidence.** The F16 leg's script-read arm is expected to flip to "contained". Every arm's outcome on windows-latest is reported, and F16 is reclassified from that evidence.
- **Release note:** security fix (SC-01), with no breaking marker (decision 46). Text: "Module script files, custom-check files and env files are now read only if they resolve inside the module directory. Linking a sibling module, or any outside path, into `invowk_modules/` and reading a script or env file through that link no longer works. Vendor real copies with `invowk module vendor` instead."

## Capabilities

### New Capabilities

- `module-read-containment`: script files, custom-check files and env files that a module makes Invowk read are physically contained in the module directory.

### Modified Capabilities

- `module-path-containment-model`: F13 and F17 move from recorded findings to fixed behaviour with pre-fix regression mutants. The dead-code rows for `ValidateScriptPath` and `checkSymlinkSafety` go away. The replay, rapid and F16-leg requirements now state the contained verdicts.

## Impact

- **Code:**
  - `pkg/invowkfile`: `implementation.go`, `dependency.go`, and a new contained-read helper;
  - `internal/runtime`: `dotenv.go` and `env_builder.go`;
  - `pkg/invowkmod/invowkmod.go`, where the API is removed, with `invowkmod_mutation_test.go`, `operations_test.go` (`TestModule_ValidateScriptPath`) and its entry in `tests/coverage/pathmatrix_coverage_test.go`.
- **Tests:**
  - unit tests in `pkg/invowkfile` and `internal/runtime`;
  - the module path containment golden, rapid and formal replays in `internal/app/moduleops`;
  - an `internal/testutil/fstree` option;
  - a new testscript pair under `tests/cli/testdata/`;
  - new tests in `cmd/invowk` (validate interpreter diagnostics) and `internal/app/deps` (custom checks).
- **Formal:** `formal/alloy/ModulePathContainment.als`, `formal/manifest.toml`, the regenerated golden, `formal/README.md`, `.goplint/exceptions.toml`, and the formal-bindings ledger entries of the rewritten and deleted symbols.
- **Docs:**
  - `website/docs/modules/validating.mdx` and `creating-modules.mdx`, with their pt-BR copies;
  - `.agents/skills/module-security/` (SC-01 and its check catalog);
  - `website/docs/environment/env-files.mdx`, with its pt-BR copy;
  - `README.md`, where it describes `env.files` resolution;
  - `docs/next/cross-platform-windows-improvements.md`.
- **Not changed:** `internal/audit`'s own script-read containment (`scan_files.go:scriptReadPath`) keeps its `EvalSymlinks` comparison; whether to align it is an open question for the maintainer (design Open questions).
- **Users:** a module that reads a script or env file through a link to outside the module, or through a link with an absolute target, now fails with a containment error. Reads of real files, including real vendored copies under `invowk_modules/`, keep working.
