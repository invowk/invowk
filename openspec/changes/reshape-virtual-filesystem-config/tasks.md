## 1. OpenSpec Contract Cleanup

- [x] 1.1 Update `rename-virtual-and-add-lua` proposal/design/spec text so it describes only `platforms[].virtual.filesystem.{access,paths}` and no implementation-level `allowed_paths`.
- [x] 1.2 Update `rename-virtual-and-add-lua` tasks so remaining work references the final virtual filesystem shape and not the old shape.
- [x] 1.3 Run `openspec validate rename-virtual-and-add-lua --strict` after artifact cleanup.
- [x] 1.4 Run `openspec validate reshape-virtual-filesystem-config --strict` after artifact cleanup.

## 2. CUE Schema And Go Model

- [x] 2.1 Replace implementation-level `allowed_paths` in `pkg/invowkfile/invowkfile_schema.cue` with `platforms[].virtual.filesystem.access` and `platforms[].virtual.filesystem.paths`.
- [x] 2.2 Add CUE enum constraints for virtual filesystem access values `"restricted"` and `"full"`, defaulting omitted access to `"restricted"`.
- [x] 2.3 Model `virtual.filesystem.paths` as a map of safe logical names to non-empty string path values, with no platform-keyed nested object support.
- [x] 2.4 Add Go value types/struct fields for platform virtual filesystem config and remove the legacy implementation-level path field from `Implementation`.
- [x] 2.5 Update Go validation so platform virtual filesystem configs validate access mode, path names, path values, and default restricted behavior.
- [x] 2.6 Remove old implementation-level allowed-path helpers or rename/rehome them so no public Go API, comments, errors, or tests preserve the old field name.
- [x] 2.7 Update CUE generation and round-trip parsing so generated invowkfiles emit `platforms[].virtual.filesystem` and cannot emit `allowed_paths`.
- [x] 2.8 Update schema sync and behavioral sync tests for the new platform virtual filesystem shape and the removed implementation field.

## 3. Runtime And Execution Wiring

- [x] 3.1 Rewire virtual path resolution to read filesystem config from the selected platform entry instead of legacy implementation-level path state.
- [x] 3.2 Implement restricted mode so implicit roots plus `virtual.filesystem.paths` roots are the only allowed VM-controlled filesystem roots.
- [x] 3.3 Implement full mode so VM-controlled filesystem operations may access host filesystem paths after normalization and resolver checks.
- [x] 3.4 Preserve anchor resolution and script bridge exposure for `INVOWK_ANCHOR_*`, `INVOWK_PATH_*`, `invowk.path`, and Lua file I/O.
- [x] 3.5 Update virtual-sh interactive subprocess handoff so path handles and filesystem access mode survive the subprocess boundary.
- [x] 3.6 Update virtual-lua interactive subprocess handoff so path handles and filesystem access mode survive the subprocess boundary.
- [x] 3.7 Confirm `runtimes[].allowed_binaries` and `runtimes[].binary_lookup_mode` remain runtime-scoped and unaffected by filesystem access mode.

## 4. CLI, Dry Run, And Audit

- [x] 4.1 Update dry-run planning and rendering to show virtual filesystem access mode and named paths under a virtual filesystem section.
- [x] 4.2 Ensure dry-run output distinguishes filesystem access from host-binary policy.
- [x] 4.3 Update deterministic audit checks to flag or surface `virtual.filesystem.access: "full"` as broad host filesystem access.
- [x] 4.4 Update Lua and virtual runtime audit discovery/reporting to use `virtual.filesystem.paths` terminology.
- [x] 4.5 Update LLM audit prompts, module-security guidance, and agent/security docs so reviews understand restricted vs full filesystem access.

## 5. Tests And Fixtures

- [x] 5.1 Add parser tests accepting platform `virtual.filesystem.access: "restricted"` with `paths`.
- [x] 5.2 Add parser tests accepting platform `virtual.filesystem.access: "full"` with and without `paths`.
- [x] 5.3 Keep parser tests focused on current virtual filesystem fields and current-field validation failures.
- [x] 5.4 Add parser rejection tests for platform-keyed nested path values under `virtual.filesystem.paths`.
- [x] 5.5 Add parser rejection tests for invalid access values, invalid path names, empty path values, and misplaced binary-policy fields under `platforms[].virtual`.
- [x] 5.6 Add generation tests proving output uses `platforms[].virtual.filesystem`.
- [x] 5.7 Add virtual-sh runtime tests for restricted mode path allow/deny behavior and full mode host path access.
- [x] 5.8 Add virtual-lua runtime tests for restricted mode path allow/deny behavior and full mode host path access.
- [x] 5.9 Add u-root utility tests proving restricted/full filesystem modes are respected by built-in file operations.
- [x] 5.10 Add dry-run tests for restricted mode, full mode, named paths, and separate host-binary policy rendering.
- [x] 5.11 Add audit tests for full filesystem access and updated virtual filesystem terminology.
- [x] 5.12 Update CLI testscript fixtures, samples, generated invowkfiles, and benchmark fixtures to use the final shape.

## 6. Documentation And Examples

- [x] 6.1 Update `README.md` runtime, security, environment, schema, and examples sections for `virtual.filesystem.access` and `virtual.filesystem.paths`.
- [x] 6.2 Update website current docs for runtime modes, configuration/reference pages, implementation concepts, security/audit pages, and interpreter/Lua examples.
- [x] 6.3 Update Portuguese current i18n docs for the same affected surfaces.
- [x] 6.4 Update website snippet data so all virtual filesystem examples use `platforms[].virtual.filesystem`.
- [x] 6.5 Update `samples/invowkmods/` and any generated/example invowkfiles to remove `allowed_paths`.
- [x] 6.6 Update relevant `.agents/skills/` or agent guidance if virtual runtime authoring/audit guidance mentions the old shape.
- [x] 6.7 Run `make check-agent-docs` if any `.agents/` files are changed.

## 7. Stale Shape Removal

- [x] 7.1 Search CUE, Go, tests, docs, snippets, samples, OpenSpec artifacts, and agent docs for `allowed_paths` and remove every active-contract reference.
- [x] 7.2 Search for legacy Go path-mapping symbols and remove or rename every old-contract symbol.
- [x] 7.3 Confirm any remaining `allowed_paths` references are only historical or explanatory OpenSpec text, not active product docs, samples, snippets, or test fixtures.
- [x] 7.4 Confirm no compatibility shim, alias, dual-read decode path, migration warning, ignored field, or tombstone remains for implementation-level `allowed_paths`.

## 8. Verification

- [x] 8.1 Run targeted `go test` for `pkg/invowkfile`, `internal/runtime`, `internal/app/commandsvc`, `internal/app/commandadapters`, `internal/audit`, and `cmd/invowk`.
- [x] 8.2 Run CLI testscript coverage for virtual-sh, virtual-lua, dry-run, validation, audit, and relevant runtime override fixtures.
- [x] 8.3 Run `make test-cli`.
- [x] 8.4 Run `make check-baseline`.
- [x] 8.5 Run `make lint`.
- [x] 8.6 Run `make test`.
- [x] 8.7 Run docs validation/build/typecheck commands required by the docs workflow after website changes.
- [x] 8.8 Run `openspec validate reshape-virtual-filesystem-config --strict`.
- [x] 8.9 Run `openspec validate rename-virtual-and-add-lua --strict`.
- [x] 8.10 Run `openspec validate --specs --strict`.
- [x] 8.11 Run `git diff --check`.
