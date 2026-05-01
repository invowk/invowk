# Cross-Platform (Windows) Regression Prevention

## Status

- **A**: Cross-platform-paths goplint analyzer — **planned + implementing in this batch**.
- **B**: `windows-testing` skill auto-triggers expanded — **planned + implementing in this batch**.
- **C**: `GOOS=windows` cross-compile pre-commit gate — **planned + implementing in this batch**.
- **D**: Cross-platform path-resolution test matrix helper — **deferred** (see below).

## Problem statement

The v0.10.0 release surfaced four Windows-only test failures across two consecutive
release attempts:

1. `TestValidateFilepathAlternatives/missing_path_returns_error` — `filepath.IsAbs("/foo")` returns false on Windows; the resolved path got joined with the invowkfile dir, so a probe keyed by the displayed path never matched.
2. `TestGetContainerWorkDir/container_absolute_workdir_stays_absolute` — same root cause via `GetEffectiveWorkDir`'s `filepath.FromSlash + filepath.IsAbs` chain.
3. `TestGetContainerWorkDir/container_absolute_CLI_override_stays_absolute` — same root cause as #2.
4. `TestPrepareCommandIncludesProvisionedEnvVars` — `RunOptions.Validate()` rejected the auto-mount string `<host>:/workspace` because `<host>` contained Windows backslashes.

All four bugs were already documented in `.agents/rules/windows.md`. The pattern of
"every refactor surfaces a new Windows path bug" indicates the issue is **recall**
during refactor work, not lack of knowledge.

The strongest interventions are ones that *force* the rules to be consulted —
either by an automated check at commit time or by an automatic skill load when
diff content matches a known-risky pattern.

---

## A. Cross-platform-paths goplint analyzer

**Status:** Implementing in this batch.

### Goal

Statically detect the canonical Windows path bug pattern at edit/commit time on
Linux, before it can reach Windows CI.

### Scope (V1)

Flag any `filepath.IsAbs(...)` call whose argument resolves through
`filepath.FromSlash(x)` (directly or via a recently-assigned variable). This
exact pattern is the root cause of failures 1, 2, and 3 above.

```go
// FLAGGED — the canonical bug
nativePath := filepath.FromSlash(workdir)
if filepath.IsAbs(nativePath) { ... }

// FLAGGED — same bug, single-line form
if filepath.IsAbs(filepath.FromSlash(workdir)) { ... }

// NOT FLAGGED — `strings.HasPrefix("/")` precedes the FromSlash chain
if strings.HasPrefix(workdir, "/") { return ... }
nativePath := filepath.FromSlash(workdir)
if filepath.IsAbs(nativePath) { ... }
```

### Why this scope (and not "any `filepath.IsAbs`")

A naive "flag every `filepath.IsAbs`" rule has high false-positive risk because
many `IsAbs` calls operate on *true host paths* where platform-native semantics
are correct. The `FromSlash` precondition narrows the rule to exactly the
"convert to native, then check abs" anti-pattern that bit us four times.

### Suppression

- `//goplint:ignore` on the function declaration.
- TOML key `pkg.FuncName.cross-platform-path` in `tools/goplint/exceptions.toml`.

### Wiring

- Flag: `--check-cross-platform-paths` (opt-in initially via `--check-all`).
- Category: `cross-platform-path`.
- Includes: implementation file, integration test, fixture under
  `testdata/src/cross_platform_path/`, unit tests for helpers,
  `make check-types-all` integration.

### Latent bugs found by the analyzer

The analyzer detects three additional latent bug sites in production code that
have the same Windows pattern but were never exercised by the Windows-short test
suite. These are **fixed in this batch** as part of the analyzer rollout:

- `pkg/invowkmod/invowkmod.go:573-577` — `Module.ResolveScriptPath`
- `pkg/invowkmod/invowkmod.go:592-595` — `Module.ValidateScriptPath`
- `pkg/invowkfile/implementation.go:329-330` — `Implementation.GetScriptFilePathWithModule`

### Future enhancements (out of scope for V1)

- Detect `<host>:/<literal>` volume-mount string concatenation without a prior
  `filepath.ToSlash`. This caught failure #4 but is harder to detect statically
  and has a higher false-positive risk; defer until we see another regression.
- Type-aware detection: opt-in `//goplint:cross-platform-path` directive on a
  type to flag any `filepath.IsAbs(value-of-this-type)` call. Useful for
  `WorkDir`, `FilesystemPath`, but adds analyzer complexity. Defer until we
  have a second class of cross-platform path bug not caught by the FromSlash
  rule.

---

## B. `windows-testing` skill auto-trigger expansion

**Status:** Implementing in this batch.

### Problem

The current `windows-testing` skill description is reactive: it lists triggers
like "debugging Windows-only test failures" and "writing platform-split tests".
Agents only consult it *after* a Windows failure, not *while* refactoring code
that could introduce one.

### Change

Expand `.agents/skills/windows-testing/SKILL.md` description to include
proactive triggers — specifically, refactoring or adding new code that touches:

- `filepath.IsAbs`, `filepath.Join`, `filepath.FromSlash`, `filepath.ToSlash`
- Volume-mount string construction (`":" + path` patterns)
- `FilesystemPath`, `WorkDir`, `SubdirectoryPath`, `ScriptPath` types
- Any function in `internal/runtime/`, `internal/container/`, `internal/app/deps/`
  whose body resolves user-fed paths

Add a procedural "Refactoring Path-Touching Code" section that walks the agent
through the canonical pre-flight checklist:

1. Is the input a CUE-fed string or a host-side path?
2. If CUE-fed: is `strings.HasPrefix(input, "/")` checked before any
   `filepath.FromSlash` or `filepath.IsAbs`?
3. If you're constructing a volume mount: is the host portion `filepath.ToSlash`-ed?
4. Is there a Windows test fixture exercising the case where input is `/foo`?

### Trade-off

Auto-trigger is a heuristic — agents can ignore it. But the harness is good at
matching specific keywords, and the skill description tweak is essentially
free.

---

## C. `GOOS=windows` cross-compile pre-commit gate

**Status:** Implementing in this batch.

### Goal

Catch Windows-build-time regressions on every commit, on Linux, in seconds.

### What it catches

- Imports that are Linux-only (e.g., `golang.org/x/sys/unix`)
- Build tags that exclude Windows accidentally
- Type signatures that depend on platform-specific types
- Syscall regressions that compile on Linux but not on Windows

### What it does NOT catch

- Runtime-only path bugs like `filepath.IsAbs("/foo")` returning different values.
  Those are A's territory.

### Implementation

- `scripts/check-windows-build.sh` — runs `GOOS=windows go build ./...` and
  `GOOS=windows go vet ./...` for both modules (`./...` and
  `tools/goplint/...`).
- Pre-commit hook in `.pre-commit-config.yaml` triggered on `*.go` changes.
- Makefile target `check-windows-build` for parity with other gates.
- Docs entry in `.agents/rules/commands.md` Quick Reference table.

### Cost

- Cold cross-compile: ~10-20 seconds depending on hardware.
- Warm rebuild: 1-3 seconds.
- Negligible CI cost since it's already part of the lint job's compile work.

---

## D. Cross-platform path-resolution test matrix helper

**Status:** Deferred.

### Goal

Codify the "Cross-Platform Path Validator Matrix" pattern (already documented
in `.agents/rules/testing.md` — Cross-Platform Path Validator Matrix) as a
callable helper that any path-resolving function can use to exercise all 7
canonical input vectors:

- Unix absolute (`/absolute/path`)
- Windows drive absolute (`C:\absolute\path`)
- Windows rooted/UNC (`\absolute\path`, `\\server\share`)
- Slash traversal (`a/../../escape`)
- Backslash traversal (`a\..\..\escape`)
- Valid relative (`tools`, `modules/tools`, `./tools`)

```go
// Hypothetical API
testutil.PathResolutionMatrix(t, func(input string) (string, error) {
    return myResolver(input)
}, testutil.PathMatrixExpectations{
    UnixAbsolute:       "/absolute/path",       // pass-through
    WindowsAbsolute:    "C:\\absolute\\path",   // pass-through
    UNC:                "rejected",
    SlashTraversal:     "rejected",
    BackslashTraversal: "rejected",
    ValidRelative:      "<base>/tools",
})
```

### Why deferred

- A + B + C combined would have caught **every failure** in the v0.10.0 incident.
- D requires retrofitting many existing path-resolving functions and adds
  test infrastructure; the ROI is unclear until we see another bug class that
  A doesn't cover.
- Revisit in 3-6 months after seeing what bug classes A misses.

### Possible future trigger

Promote D to "implementing" if any of the following becomes true:

- A's false-negative rate (Windows bugs reaching CI despite analyzer being
  enabled) exceeds 1 per quarter.
- A new bug class emerges that A's `FromSlash → IsAbs` rule cannot detect.
- The number of cross-platform-relevant path resolvers exceeds 10 (currently
  ~5) and ad-hoc test coverage becomes hard to audit.

---

## Combined ROI

After A + B + C land, the v0.10.0-style failure flow looks like:

| Stage | Catches |
|-------|---------|
| Editor (when Codex/Claude runs goplint via LSP-like setup, future) | A — `FromSlash → IsAbs` chains |
| `git commit` (pre-commit hooks) | A (baseline gate), C (Windows compile), existing lint/baseline gates |
| Skill consultation during refactor | B — windows-testing skill loads automatically |
| Windows CI | Last line of defense for runtime bugs A can't predict |

The four v0.10.0 failures break down:

- **#1, #2, #3 (FromSlash + IsAbs pattern)**: Would have been caught by A on
  the developer's Linux machine, before commit.
- **#4 (volume-mount backslash)**: Would NOT have been caught by A directly,
  but **would have been caught by skill B** if the skill loaded automatically
  on a diff that touched `:/workspace` literal — the procedural checklist
  asks "is the host portion `filepath.ToSlash`-ed?".
- **All four** would still have been caught by Windows CI as a final safety net.

The combined intervention shifts catch-time from "release matrix run" to
"developer's editor or pre-commit hook" for the most common bug class, and from
"Windows CI" to "skill consultation during the refactor" for the harder
pattern-recognition cases.
