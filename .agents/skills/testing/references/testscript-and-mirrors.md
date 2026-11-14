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
- `commonSetup` (`tests/cli/cmd_test.go`) sets `HOME=$WORK` for every test
  (plus `APPDATA`/`USERPROFILE` on Windows); see rule "Integration and Container
  Tests" before changing home handling.
- Use `--` between Invowk flags and user-command flags.
- Assert both output channels on error paths unless the harness documents
  unavoidable incidental output.
- Give each file a purpose comment, explicit feature guards, and all embedded
  support files required to reproduce the behavior.

## Cross-Platform Implementations

Virtual runtime implementations may cover Linux, macOS, and Windows together.
Native mirrors must split Unix shell and Windows PowerShell implementations:
the CUE split and translation table live in `native-mirror`, and the PowerShell
and assertion rules in rule "Native tests".

Mirror pair alignment:

- Command paths must match unless `command_path_exempt` in the exemption file
  records the difference; `TestVirtualNativeCommandPathAlignment` checks this.
- stdout/stderr assertions should match by default. Runtime-identity output
  (runtime name, dry-run rendering, runtime-specific error wording, as in
  `virtual_dryrun`/`native_dryrun` and `virtual_timeout`/`native_timeout`) may
  differ; that is not recorded in the JSON and not machine-checked, so justify
  each difference with a comment in the native file.
- Within one native test, assertions are identical across platforms (rule
  "Native tests").

The only exemption source of truth is
`tests/cli/runtime_mirror_exemptions.json`. Do not copy its entries into skill
documentation.

## Live Inventory and Verification

Enumerate the suite live:

```bash
rg --files tests/cli/testdata -g '*.txtar' | sort
```

Guard tests and `make test-cli`: `.agents/skills/testing/SKILL.md` "Verification
Router".

Use testscript for non-interactive CLI behavior. Route interactive TUI flows to
`tmux-testing`, and visual/demo recording to `tui-testing`.
