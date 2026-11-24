## Why

Under `restricted` filesystem access, the virtual runtimes (`virtual-sh`, `virtual-lua`) must accept a file operation only when its path resolves physically inside an allowed root. One function, `normalizeExistingOrParent`, produces the path that every consumer compares, and it falls back to the unresolved (lexical) path in three cases. Each one lets a write land outside the allowed roots:

- **F14 (recorded, open):** a path with two or more missing components below a symlink that leaves the root. `mkdir -p link/a/b` creates directories outside.
- **F18 (reproduced on windows-latest, not yet recorded):** a path through a Windows junction. `filepath.EvalSymlinks` does not follow junctions, so an existing file, a new file and a deep new path below a junction to outside are all accepted.
- **F19 (found by this change's spec stage, reproduced on Linux at c2fe96ff):** a dangling symlink whose target lies outside. `echo x > link` is accepted and creates the target file outside the root. No deep path is needed.

Maintainer decisions 43, 45 and 53 schedule F18 and F14 as one change (B2). F19 has the same cause and the same fix, so it is closed here too (see design.md, "Open questions", for the bookkeeping the maintainer may revise).

## What Changes

- **One physical resolver for every consumer.** A path is resolved component by component: every symbolic link and every Windows junction is followed, the deepest existing ancestor is found physically, and the missing components are joined back onto it. No lexical fallback remains.
  - Consumers: the validator behind shell redirection, u-root built-ins and Lua file I/O; Lua `invowk.path`; Lua `require`; the allowed-root list; and declared `virtual.filesystem.paths`.
- **Fail closed.** A path that cannot be resolved is refused: a link cycle, an unreadable link, a component that cannot be inspected, a file used as a directory, or a volume that does not exist (a missing drive or UNC share).
- **No new denials for links that stay inside a root.** A link or junction whose target is inside an allowed root keeps working, whether its target is relative or absolute. A root that is itself reached through a link or junction keeps working (macOS `/tmp` and `/var`, a Windows `%TEMP%` or project behind a junction).
- **Paths accepted now that were wrongly denied.** A request two or more missing levels below a root that is reached through a link (for example `/tmp/new1/new2` on macOS, or a path under a Windows temp directory written with an 8.3 short name) is now compared in the same form as the root and accepted.
- **Path forms that change, each deliberate** (design.md D3 lists them): deep missing paths, paths through a dangling link and Windows paths through a junction are now reported in resolved form, in error messages, `invowk.path` results and `INVOWK_PATH_*` values. Every other path keeps its current form.
- **A declared `virtual.filesystem.paths` entry that cannot be resolved fails the command** before the script runs, with an error that names the entry. Before, it was kept unresolved.
- **Formal artefacts.** The `VirtualPathHarness` model gains the link kind (symlink or junction) and dangling links. F14 becomes fixed; F18 and F19 are recorded as fixed after an observed escape. Each keeps a pre-fix regression mutant and a replay that asserts the denial and that nothing is created outside. The golden replay compares the code with the model's fixed decision on Linux, macOS and Windows, junction vectors included.
- **SonarCloud go:S8209** at `internal/runtime/virtual_policy.go:96` and `:124`: the two signatures group their `string` parameters. Formatting only.
- **Not changed:** F15 (the workdir as an allowed root), `full` access semantics, host binaries (they run as host processes and are not path-checked), and the race between the check and the open (design.md, Risks).

**Release note (security fix, no breaking marker):** "Virtual runtimes: under restricted filesystem access, paths are now resolved through every symbolic link and Windows junction before they are checked. Three ways to write outside the allowed roots are closed: a deep new path below a link that leaves the root, any path through a Windows junction, and a dangling link to a file outside. Links whose targets stay inside an allowed root keep working. On macOS, new nested paths under `/tmp` and `/var` (for example `/tmp/a/b`) are no longer denied by mistake. The virtual runtimes are still not a sandbox: allowed host binaries run as normal host processes; use the container runtime for isolation."

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `virtual-runtime-sandbox`: new requirements next to "Path sanitization for virtual runtimes": physical resolution through links and junctions with the deepest-existing-ancestor rule, the fail-closed cases, the in-root link guarantees, and the resolved path as the checked and reported path.
- `module-path-containment-model`: the `VirtualPathHarness` requirements describe the new normalisation, link kinds and dangling links; F14, F18 and F19 are fixed findings with pre-fix mutants; the harness golden replays junction vectors on Windows; the harness rapid property loses its F14 exemption.
- `formal-verification-toolchain`: "Defect fixes are separate from finding records" gains one stated exception (a finding recorded as fixed, with observed-escape evidence, in a fix change the maintainer directed); "A rebinding that fails on unmodified code is a finding" no longer names F18 as the next free id.

## Impact

- **Code:** `internal/runtime/virtual_policy.go` (the resolver and its callers), `internal/runtime/lua_io.go` (Lua `require` keeps calling the same function).
- **Tests:** `internal/runtime` (harness golden, rapid and formal replays, consumer tests, a resolver unit test over a fake filesystem), `internal/testutil/fstree` only if the golden replay needs a helper, a new `tests/cli/testdata/virtual_*.txtar` with an entry in `tests/cli/runtime_mirror_exemptions.json`.
- **Formal:** `formal/alloy/VirtualPathHarness.als`, `formal/manifest.toml`, the regenerated `internal/runtime/testdata/formal/virtual_path_harness_golden.json.gz`, `formal/README.md`, and the formal-bindings ledger entries of the rewritten function (fix stage).
- **Docs:** `website/docs/runtime-modes/virtual.mdx` and `virtual-lua.mdx` with their pt-BR copies; `.agents/skills/windows-testing/references/filesystem-pitfalls.md` (its claim that `EvalSymlinks` follows junctions is wrong since Go 1.23); `.agents/skills/formal-verification/SKILL.md` and `.agents/skills/virtual-lua/SKILL.md` where they describe the harness.
- **Users:** scripts that relied on writing through a link or junction to a place outside the allowed roots now fail with a path-denied error. They can declare the target in `virtual.filesystem.paths`, or set `virtual.filesystem.access: "full"`.
- **Dependencies:** none added.
