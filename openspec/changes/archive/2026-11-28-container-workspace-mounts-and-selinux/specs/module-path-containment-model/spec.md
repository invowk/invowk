# Spec Delta

## Purpose

Updates `module-path-containment-model` to align `ModulePathContainment.als` and `VirtualPathHarness.als` with the 3-tier container workspace/volume mount ceiling and runtime rejection of absolute host bind-mounts in `*.invowkmod`.

## MODIFIED Requirements

### Requirement: Allowed-location predicate set
The set of allowed-location predicates SHALL contain:
- `allowedWorkspace`: the container workspace mount root (`invowkDir` by default, or `effectiveWorkDir` / resolved `volumes` bounded by the 3-tier containment ceiling: module root for Layer 1, `<cwd>` for Layer 2 local sources, or operator-configured `IncludeEntry.workspace_root` for Layer 3 included modules, and CLI `--ivk-workdir`);
- `allowedVendoredSelf`: a vendored child's own root, for the child's own operations;
- `allowedVirtualRoot`: the virtual runtime's standard anchors, temporary roots, and declared `filesystem.paths`;
- `allowedTrustedWorkdir` (`VirtualPathHarness` only): a trusted workdir, that is the `--ivk-workdir` override, a workdir of an invowkfile outside a module, or a module's default workdir, as a virtual allowed root under `restricted` access. Justification: the user chose the directory, or it is the module directory, so no module widens its own roots through it;
- `allowedWorkdirCwd`: a module-declared `workdir` that lies outside the module, as the execution working directory only, never as a read, copy, hash, or virtual allowed-root target (and never as a container mount root unless admitted by Layer 2 `<cwd>` or Layer 3 `IncludeEntry.workspace_root`).

#### Scenario: Workdir admitted only as working directory
- **WHEN** a virtual runtime operation or Layer 1 module uses a module-declared `workdir` outside the module root
- **THEN** `allowedWorkdirCwd` SHALL admit it only as the execution working directory, and never as a read, copy, hash, virtual allowed-root, or Layer 1 container mount target

#### Scenario: Trusted workdir admitted as a virtual root
- **WHEN** a request under `restricted` access resolves physically inside a trusted workdir
- **THEN** `allowedTrustedWorkdir` SHALL admit it, and the predicate SHALL appear in the `VirtualPathHarness` correspondence table with its justification

#### Scenario: Layer 2 local module workdir inside cwd admitted for container workspace
- **WHEN** a local `cwd` module mounts `workdir: ".."` inside `<cwd>` via `workspace_mount: { root: "workdir" }`
- **THEN** `allowedWorkspace` SHALL admit `<cwd>` for container workspace mounting while rejecting escapes above `<cwd>` and rejecting absolute host bind-mounts in `*.invowkmod`
