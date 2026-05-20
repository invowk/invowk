## Context

The active `rename-virtual-and-add-lua` change introduced a shared virtual runtime safety harness for `virtual-sh` and `virtual-lua`. An earlier draft used implementation-level `allowed_paths`, with each entry either a common path string or a platform-keyed object.

That shape works mechanically, but it blends three concepts:

- Platform-specific host path facts.
- Virtual-runtime filesystem policy.
- Logical bridge names exposed to scripts.

It also leaves `allowed_paths` semantically wrong once a full-filesystem mode exists: in full mode, named paths are no longer the only allowed paths. They are handles exposed to shell and Lua scripts.

This change is a clean break before the virtual filesystem contract ships. The final public shape must be the only active shape in CUE, Go, docs, samples, generated output, tests, and OpenSpec artifacts.

## Goals / Non-Goals

**Goals:**
- Replace the legacy implementation-level path field with platform-scoped `virtual.filesystem`.
- Make filesystem permission mode explicit with `virtual.filesystem.access`.
- Support exactly two access modes: `"restricted"` and `"full"`.
- Default omitted access mode to `"restricted"`.
- Rename logical path mappings to `virtual.filesystem.paths`.
- Keep logical path mappings as a map/object, not a list.
- Keep `runtimes[].allowed_binaries` and `runtimes[].binary_lookup_mode` in runtime config.
- Preserve shell/Lua bridge exposure through `INVOWK_PATH_<NAME>` and `invowk.path("<NAME>/...")`.
- Add dry-run and audit visibility for full filesystem access.
- Remove every old artifact, helper, example, and active-contract spec reference instead of preserving compatibility.

**Non-Goals:**
- No backward compatibility for the legacy implementation-level path field.
- No alias, migration warning, dual-read parser, legacy tombstone, ignored field, or compatibility decode path.
- No list-shaped path mapping form.
- No platform-specific `allowed_binaries` syntax in this change.
- No change to runtime selector names: `virtual` remains invalid as a runtime name.
- No kernel-level sandbox guarantee for virtual runtimes.

## Decisions

### Decision 1: Use `platforms[].virtual.filesystem`

The final shape is:

```cue
implementations: [{
	script: {content: "..."}

	platforms: [{
		name: "linux"
		virtual: {
			filesystem: {
				access: "restricted"
				paths: {
					DATA:  "@data/my-tool"
					CACHE: "@cache/my-tool"
				}
			}
		}
	}]

	runtimes: [{
		name: "virtual-lua"
		allowed_binaries: ["git"]
		binary_lookup_mode: "strict"
	}]
}]
```

Rationale: `virtual` is the family namespace already used in config, and here it names platform-specific settings for the virtual runtime family. `filesystem` creates room for future platform-scoped virtual settings without overloading a safety-only name.

Alternatives considered:
- `platforms[].allowed_paths`: rejected because the field is virtual-specific, not platform-general.
- `platforms[].virtual_safety.allowed_paths`: rejected because the namespace is too narrow for future virtual-family platform settings.
- `platforms[].virtual_runtime.allowed_paths`: rejected because it is redundant beside `runtimes` and can sound like a runtime selector.

### Decision 2: Replace the legacy path field with `filesystem.paths`

`filesystem.paths` is a map from logical uppercase names to path strings:

```cue
paths: {
	DATA: "@data/my-tool"
}
```

The keys remain safe environment suffixes. Values are strings, not platform-keyed objects, because each `paths` map already lives under a single platform entry.

In `"restricted"` mode, `paths` entries are both named bridge handles and allowed filesystem roots. In `"full"` mode, `paths` entries are named bridge handles only.

Alternatives considered:
- Keep `allowed_paths`: rejected because `"full"` access makes the name inaccurate.
- Make `paths` a list: rejected because entries are named capabilities consumed by scripts; a map is simpler, deterministic, and avoids duplicate-name ambiguity.
- Use `"*"` inside the path map: rejected because `"*"` is not a valid logical bridge name and should not become an environment variable suffix.

### Decision 3: Add `filesystem.access`

`filesystem.access` supports:

- `"restricted"`: default. VM-controlled filesystem operations are allowed only under implicit safe roots and `filesystem.paths` roots.
- `"full"`: VM-controlled filesystem operations may access the host filesystem after normal path normalization and resolver checks. This is an explicit opt-out from root containment, not a security sandbox.

Omitting `virtual`, `virtual.filesystem`, or `virtual.filesystem.access` is equivalent to `"restricted"` with no additional named paths.

Dry-run and audit output must surface `"full"` prominently because it is a broad filesystem permission.

Alternatives considered:
- `allow_all_paths: true`: rejected because it does not compose as cleanly with future access modes and is harder to render as a single permission state.
- `filesystem_access: "full"` beside `paths`: rejected because nesting under `filesystem` gives clearer ownership and room for future filesystem settings.

### Decision 4: Keep host binary policy in runtime config

`allowed_binaries` and `binary_lookup_mode` remain under each virtual runtime config:

```cue
runtimes: [{
	name: "virtual-sh"
	allowed_binaries: ["git"]
	binary_lookup_mode: "strict"
}]
```

Rationale: These fields answer "may this selected virtual runtime launch host binaries, and how are they resolved?" They are runtime policy, not platform path mapping.

Alternatives considered:
- Move binary policy under `platforms[].virtual`: rejected because it becomes ambiguous when one implementation supports both `virtual-sh` and `virtual-lua`.
- Add platform-keyed binary policy under runtimes: rejected for this change because bare binary names cover the common cross-platform case and platform-specific binary policy can be handled by separate implementations if needed.

### Decision 5: Clean-break removal is part of the contract

Implementation must remove old CUE fields, Go struct fields, value types, docs, snippets, generated examples, OpenSpec active-contract text, and helper names tied to the legacy implementation-level path field.

Closed CUE structs should reject the stale field as unknown. Go direct construction should not have an implementation-level path mapping field to set. Generated CUE must be unable to emit the old shape.

## Risks / Trade-offs

- [Risk] `virtual` namespace could be confused with removed `virtual` runtime selector. -> Mitigation: schema diagnostics, docs, and examples must state that `virtual` is a namespace only; runtime selectors remain `virtual-sh` and `virtual-lua`.
- [Risk] `"full"` filesystem access may be mistaken for process isolation. -> Mitigation: dry-run, audit, and docs must explain that virtual runtimes are not a kernel sandbox and that `container` is required for process-level isolation.
- [Risk] Moving paths under platforms increases duplication when all platforms share the same logical path. -> Mitigation: the duplication is acceptable because host paths are platform facts; separate implementations can reduce duplication when a command is truly single-platform.
- [Risk] Runtime code may continue reading stale implementation-level fields. -> Mitigation: remove the Go field, update schema sync/generation tests, and run stale-reference searches for legacy field and symbol names.
- [Risk] Active OpenSpec artifacts can drift from the final contract. -> Mitigation: update the existing virtual runtime change artifacts so their proposal, design, specs, and tasks use only the final `virtual.filesystem` shape.

## Migration Plan

This is an unreleased clean break. No user migration path, compatibility mode, or deprecation period is required.

Implementation sequence:

1. Update OpenSpec artifacts for the active virtual runtime work to describe only the final shape.
2. Replace CUE schema and Go model fields with `PlatformConfig.Virtual.Filesystem`.
3. Rewire validation, path resolution, runtime execution, bridge injection, dry-run, and audit to consume the selected platform's virtual filesystem config.
4. Update generation, docs, snippets, samples, fixtures, and tests.
5. Run stale-shape searches and verification gates before the change is considered complete.

Rollback is ordinary git revert before release; no persisted user data format is migrated by this change.

## Open Questions

None. The selected contract is:

- `platforms[].virtual.filesystem.access: "restricted" | "full"`
- `platforms[].virtual.filesystem.paths: {NAME: path}`
- `runtimes[].allowed_binaries`
- `runtimes[].binary_lookup_mode`
