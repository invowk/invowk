# Testscript and Runtime Mirrors

Use this reference for `tests/cli/` and `.txtar` changes. Use `native-mirror`
for the transformation workflow and `invowk-schema` for CUE authoring rules.

## Testscript Contract

- Let each test control its working directory. Do not assign `env.Cd` in
  setup. Use `cd $WORK` for embedded fixtures and `cd $PROJECT_ROOT` only for
  tests intentionally exercising the repository invowkfile.
- Set only environment variables used by the test or production code. Use
  `${:}` for path-list separators and `${/}` where testscript path separators
  are required.
- Testscript normally isolates `HOME`; container CLI setup must provide a
  writable, test-scoped home because Docker and Podman need configuration
  directories.
- Use `--` between Invowk flags and user-command flags.
- Assert both output channels on error paths unless the harness documents
  unavoidable incidental output.
- Give each file a purpose comment, explicit feature guards, and all embedded
  support files required to reproduce the behavior.

## Cross-Platform Implementations

Virtual runtime implementations may cover Linux, macOS, and Windows together.
Native mirrors must split Unix shell and Windows PowerShell implementations:

```cue
implementations: [
    {
        script: {content: "echo 'Hello'"}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: {content: "Write-Output 'Hello'"}
        runtimes: [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

Use `$env:NAME`, `Write-Output`, and `$ErrorActionPreference = 'Stop'` in
PowerShell. Keep command paths and stdout/stderr assertions aligned between a
virtual test and its native mirror unless the machine-readable exemption file
records a justified divergence.

The only exemption source of truth is
`tests/cli/runtime_mirror_exemptions.json`. Do not copy its entries into skill
documentation.

## Live Inventory and Verification

Enumerate the suite live:

```bash
rg --files tests/cli/testdata -g '*.txtar' | sort
go test -v -run 'TestBuiltinCommandTxtarCoverage|TestTUIExemptionTmuxCoverage' ./cmd/invowk/...
go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...
make test-cli
```

Use testscript for non-interactive CLI behavior. Route interactive TUI flows to
`tmux-testing`, and visual/demo recording to `tui-testing`.
