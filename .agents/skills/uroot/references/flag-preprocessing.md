# POSIX Combined Short Flags: Preprocessing Mechanism

Read this before changing `Registry.Run()` preprocessing, the
`NativePreprocessor` marker or `baseWrapper`, or how the virtual runtimes
dispatch to the registry. The directives that every u-root change must follow
stay in `../SKILL.md` ("POSIX Combined Short Flags").

## How It Works

Custom implementations support POSIX-style combined short flags (e.g., `ln -sf`
instead of `ln -s -f`). This is handled centrally in `Registry.Run()`;
individual commands don't need any changes.

1. `Registry.Run()` checks if the command implements `NativePreprocessor` (a
   private marker interface).
2. If it does NOT (custom implementation), args are preprocessed via
   `unixflag.ArgsToGoArgs()`, which splits combined flags:
   `["-sf", "target"]` → `["-s", "-f", "target"]`.
3. If it DOES (upstream wrapper via `baseWrapper`), args pass through unchanged;
   upstream wrappers handle this internally in their `RunContext` method.

After preprocessing, `Registry.Run()` pre-validates path arguments for the
upstream wrappers (`validateUrootCommandPathArgs`) and then calls the command.

### Adding New Custom Commands

New custom commands automatically get combined flag support as long as they:
- are registered in `BuildDefaultRegistry()` (add `r.Register(newFooCommand())`
  in the factory function in `registry.go`);
- do NOT embed `baseWrapper` (which would mark them as upstream wrappers).

### Adding New Upstream Wrappers

New upstream wrappers that embed `baseWrapper` automatically inherit
`NativePreprocessor` and skip centralized preprocessing. No extra code needed.

## Key Files

- `internal/uroot/command.go` — `NativePreprocessor` marker interface
- `internal/uroot/wrapper.go` — `baseWrapper.nativePreprocessor()` implementation and `wrapError()`
- `internal/uroot/registry.go` — `Run()` preprocessing, path pre-validation and `BuildDefaultRegistry()`
- `internal/runtime/sh.go` (`tryUrootBuiltin()`), `internal/runtime/lua.go`
  (`runVirtualUtility()`) and `internal/runtime/virtual_policy.go` — virtual
  utility resolution routes through `Registry.Run()`
