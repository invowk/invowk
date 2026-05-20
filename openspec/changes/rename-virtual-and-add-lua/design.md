## Context

Invowk currently exposes a single `virtual` runtime backed by mvdan/sh. That runtime is portable but not isolated: u-root built-ins are Go-native, while unknown commands fall through to host binaries. Adding Lua makes the old name misleading and creates a chance to define a coherent virtual runtime family.

The intended model is not a kernel jail. `virtual-*` runtimes are a Go-native safety harness: Invowk controls VM logic, u-root built-ins, bridge APIs, path resolution, and whether host binaries may launch. A launched host binary remains a normal host process. Users who need process-level isolation must use the `container` runtime.

## Goals / Non-Goals

**Goals:**
- Make runtime names explicit: `virtual-sh` and `virtual-lua`.
- Remove `virtual` completely as a clean break, with no compatibility alias.
- Replace the old shell-only config namespace with a family-level `virtual` config namespace.
- Give `virtual-sh` and `virtual-lua` the same safety model.
- Keep common cross-platform scripts ergonomic through implicit safe roots, standard anchors, and logical mappings.
- Deny host binary execution by default while allowing explicit escape hatches.
- Preserve seamless binary lookup by default for allowed binaries, with an opt-in strict lookup mode.
- Add a Lua runtime with a small Invowk-native bridge, controlled stdlib access, fresh VM execution, and optional Lua resource limits.
- Include Lua in deterministic and LLM-assisted module security audits.

**Non-Goals:**
- No backward compatibility for `virtual`.
- No migration guide requirement. Examples and docs must simply use the new shape.
- No kernel-level filesystem or process isolation for host binaries launched from `virtual-*`.
- No Lua REPL as part of this change. Interactive mode attaches script I/O; it does not open an implicit REPL.
- No nested runtime-engine syntax such as `name: "virtual", engine: "lua"`.
- No separate utility toggles for `virtual-sh` and `virtual-lua` unless a future feature creates genuinely different utility sets.

## Decisions

### Decision 1: Explicit virtual runtime names

`virtual` is removed. `virtual-sh` names the embedded shell runtime. `virtual-lua` names the embedded Lua runtime.

Rationale: The runtime name should identify the interpreter because virtual runtimes may have interpreter-specific configuration and validation. A compatibility alias would keep the ambiguous name alive and weaken the clean-break contract.

Alternatives considered:
- Keep `virtual` as shell and add `virtual-lua`: rejected because the generic name becomes misleading.
- Add an `engine` field under `virtual`: rejected because Lua-specific fields would become awkward and less strongly typed.
- Keep `virtual` as an alias to `virtual-sh`: rejected because this change is intentionally not backward-compatible.

### Decision 2: Safety-harness model for all `virtual-*`

Both virtual runtimes share:
- A path validator for VM-controlled I/O.
- An execution gater for host binaries.
- Anchor and logical-path resolution.
- Internal state injection.

The safety boundary is explicit:
- Go-native operations are enforceable.
- u-root built-ins are Go-native and enforceable.
- Host binaries are gated before launch, but not sandboxed after launch.
- Container runtime remains the answer for process-level isolation.

Rationale: Giving Lua stronger safety than shell would make `virtual-*` incoherent. Giving both runtimes a shared Go-native harness makes the runtime family understandable without pretending to provide a kernel jail.

### Decision 3: Family-level virtual config namespace

The existing `virtual_shell` config namespace is removed. The new config namespace is:

```cue
virtual: {
    utilities: {
        enabled: true
    }
}
```

`virtual` is valid only as a config namespace. It remains invalid as a runtime selector in invowkfiles, config `default_runtime`, CLI overrides, generated runtime lists, docs, samples, snippets, and tests.

`virtual.utilities.enabled` controls Invowk-provided Go-native utility commands shared by both `virtual-sh` and `virtual-lua`. The utility command set is currently backed by u-root, but the field name describes the user-facing behavior rather than the implementation library. When disabled:
- `virtual-sh` does not resolve Invowk-provided external-style utility commands through the virtual utility set.
- `virtual-lua` does not expose those utility commands through `invowk.cmd` or `invowk.capture`.
- Shell grammar, shell language built-ins, Lua language features, and bridge APIs that are not utility commands remain governed by their own runtime rules.
- Host binaries are still governed only by `allowed_binaries` and `binary_lookup_mode`; disabling utilities does not grant host binary fallback.

Rationale: The same Go-native utility set is reachable from both virtual interpreters. A shell-specific namespace would make the config lie, and separate per-interpreter toggles would invite drift without a current use case.

Alternatives considered:
- `virtual_sh.enable_uroot_utils`: rejected because the setting also affects Lua bridge command helpers.
- `virtual.uroot_utils.enabled`: rejected because u-root is implementation detail and a future utility backend should not force another config rename.
- Separate `virtual_sh.utilities.enabled` and `virtual_lua.utilities.enabled`: rejected because the utility set is shared and should behave consistently by default.

### Decision 4: Host binaries are denied by default

`allowed_binaries` belongs to virtual runtime config and defaults to no host binaries. `allowed_binaries: ["*"]` is the explicit opt-out for users who want old wide-open execution behavior. u-root built-ins are not host binaries and do not need to be listed.

Rationale: This makes the safe behavior the default and makes every escape from the Go-native harness visible in CUE.

### Decision 5: `binary_lookup_mode` controls lookup, not permission

`allowed_binaries` answers "may this binary run?" `binary_lookup_mode` answers "where do we resolve it?"

- `"host"`: default. Resolve allowed bare binary names through the effective host `PATH` after env inheritance/filtering.
- `"strict"`: resolve allowed bare binary names only through hardcoded platform system paths.
- Absolute allowed binary paths are exact executable selections and do not use PATH lookup.

Rationale: Most users expect an explicitly allowed `git` or `python` to resolve as it does in their terminal. Strict mode is available for modules that want deterministic system-only lookup.

### Decision 6: Hybrid path model with implementation-scoped mappings

Implicit safe roots:
- Effective work directory.
- Source module directory when the invowkfile comes from an invowkmod.
- OS temp directory.
- App-scoped anchors: `@config`, `@data`, `@cache`, `@state`.

`@home` resolves to the user's home directory for metadata and explicit mapping convenience, but it does not grant blanket recursive home access by default. Full or partial home access requires an explicit `allowed_paths` mapping.

`allowed_paths` belongs on implementation config, not runtime config, because platform selection belongs to implementations. It supports:
- A common string value for all selected platforms.
- A platform-keyed value with `linux`, `macos`, and/or `windows` entries.

Rationale: Platform-specific paths should not force duplicate implementation blocks. Scripts should consume logical names and anchors, while CUE carries platform-specific host path details.

### Decision 7: Standard anchor mapping

Anchors resolve to app-scoped paths unless otherwise stated:

| Anchor | Linux | macOS | Windows |
| --- | --- | --- | --- |
| `@config` | `$XDG_CONFIG_HOME/invowk` or `~/.config/invowk` | `~/Library/Application Support/invowk` | `%APPDATA%\\invowk\\config` |
| `@data` | `$XDG_DATA_HOME/invowk` or `~/.local/share/invowk` | `~/Library/Application Support/invowk` | `%LOCALAPPDATA%\\invowk\\data` |
| `@cache` | `$XDG_CACHE_HOME/invowk` or `~/.cache/invowk` | `~/Library/Caches/invowk` | `%LOCALAPPDATA%\\invowk\\cache` |
| `@state` | `$XDG_STATE_HOME/invowk` or `~/.local/state/invowk` | `~/Library/Logs/invowk` | `%LOCALAPPDATA%\\invowk\\state` |
| `@tmp` | OS temp directory | OS temp directory | OS temp directory |
| `@home` | user home | user home | user profile |
| `@work` | effective work directory | effective work directory | effective work directory |

Paths are resolved using OS-native rules, normalized before validation, and rejected if traversal or symlink resolution escapes the allowed root.

### Decision 8: Lua bridge and stdlib surface

`virtual-lua` injects an `invowk` table:
- `invowk.path(name)` resolves anchors and logical mappings.
- `invowk.env` is a read-only environment view.
- `invowk.state` exposes execution metadata, including `bin_path` for the most recently resolved host binary.
- `invowk.cmd.<name>(...)` streams a u-root built-in or allowed host binary.
- `invowk.capture.<name>(...)` captures stdout, stderr, and exit code.

Lua stdlib handling:
- Safe libraries such as `string`, `table`, `math`, and `utf8` are available.
- File I/O is available only through path-validated implementations.
- `os.getenv` reads from the same environment as `invowk.env`.
- `os.execute`, `io.popen`, unrestricted `package.loadlib`, `debug`, and dynamic Go package import through `golib` are not available in `virtual-lua`.
- `require` is restricted to the inline script context and source module tree. Native shared libraries are not loadable through `require`.

Rationale: Lua should be useful for file-oriented automation without becoming a second native runtime.

### Decision 9: Fresh Lua runtime per command execution

Every command execution gets a fresh Lua VM. Commands invoked through command dependencies also get fresh VMs because they are separate command executions. Simple dependency checks are not command execution unless they run a custom check script.

Rationale: Fresh VMs prevent hidden global-state coupling across commands and keep execution deterministic.

### Decision 10: Lua resource controls are explicit runtime config

`virtual-lua` exposes optional Lua resource controls:
- `cpu_limit`: non-negative integer in golua CPU units; omitted or zero means no Invowk-imposed CPU unit cap.
- `memory_limit`: optional byte-size string; omitted means no Invowk-imposed Lua heap cap.

When configured, limits are enforced through golua runtime contexts and failures produce actionable runtime diagnostics.

Rationale: arnodel/golua exposes CPU and memory resource mechanisms, but exact limits are user- and workload-specific. This change wires the capability without inventing a hidden default that could surprise users.

### Decision 11: Audit treats Lua as first-class module content

The audit system must discover and analyze:
- Inline `virtual-lua` scripts.
- `script.file` Lua implementations.
- Lua files reachable by module-local `require`.
- Self-contained `.lua` files in invowkmod source trees that are not directly referenced but could be used by Lua scripts.

Deterministic audit flags risky patterns. LLM audit instructions include Lua-specific guidance and explicit direction to use subagents when possible for non-trivial Lua module trees.

## Risks / Trade-offs

- **[Risk] Users assume virtual means kernel isolation**: Documentation, dry-run, and audit text must describe `virtual-*` as a safety harness and point to `container` for process isolation.
- **[Risk] Host binary escape weakens path sandbox**: Host binaries are denied by default, named in CUE when allowed, and surfaced in dry-run/audit output.
- **[Risk] Config namespace `virtual` is confused with the removed runtime name**: Schema, diagnostics, and docs must say `virtual` is a config namespace only; runtime selectors must use `virtual-sh` or `virtual-lua`.
- **[Risk] Full host PATH default can resolve surprising binaries**: `binary_lookup_mode: "strict"` provides deterministic system-path lookup for higher-integrity modules.
- **[Risk] Path validation misses symlink or Windows path edge cases**: Implement validation after OS-native normalization and symlink resolution; include Windows drive, UNC, separator, and case tests.
- **[Risk] Lua stdlib becomes too restrictive**: Keep path-validated file I/O and module-local `require`, while disabling only escape-oriented features.
- **[Risk] Documentation drift after the rename**: Treat docs/snippets/i18n/sample/test fixture updates as required implementation tasks and run agent-doc/doc integrity checks.
