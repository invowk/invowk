# Test File Inventory

Snapshot of the repository's test surface. Subagents should use the commands
below to refresh counts before review; listed files are orientation aids, not a
substitute for live enumeration.

**Last refreshed**: 2026-05-21

**Current snapshot**: 416 `*_test.go` files, 129 `.txtar` files, 120,876 lines of test code.

```bash
rg --files -g '*_test.go' cmd internal pkg tests tools
rg --files tests/cli/testdata -g '*.txtar'
find cmd internal pkg tests tools -name '*_test.go' -exec wc -l {} +
```

---

## Go Test Files by Directory

### `cmd/invowk/` (33 files)

CLI adapter tests. Key files:

| File | Lines | Focus |
|---|---|---|
| `cmd_args_test.go` | 633 | CLI argument handling |
| `cmd_watch_test.go` | 526 | Watch-mode CLI behavior |
| `cmd_context_test.go` | 524 | CLI context construction and config flow |
| `cmd_coverage_test.go` | 424 | Built-in command txtar coverage guardrail |
| `cmd_app_additional_test.go` | 404 | App wiring and command behavior |
| `cmd_app_discovery_cache_test.go` | 367 | Discovery cache behavior |
| `cmd_validate_test.go` | 337 | Validate command behavior |
| `cmd_dryrun_test.go` | 327 | Dry-run rendering and execution guards |

### `internal/` (216 files across 23 subdirectories)

| Subdirectory | Files | Largest File (lines) | Focus |
|---|---|---|---|
| `audit/` | 16 | `scan_context_test.go` (680) | Module security audit scanning and LLM review helpers |
| `runtime/` | 29 | `runtime_sh_test.go` (991) | Native, virtual-sh, virtual-lua, and container runtimes |
| `tui/` | 25 | `tui_filter_test.go` (651) | Bubble Tea model state transitions |
| `container/` | 22 | `engine_docker_mock_test.go` (803) | Docker/Podman engine mocks and types |
| `discovery/` | 11 | `discovery_collisions_test.go` (971) | Module/command discovery and collision handling |
| `app/commandsvc/` | 7 | `dispatch_test.go` (428) | Command execution service |
| `app/deps/` | 10 | `checks_test.go` (960) | Dependency validation |
| `provision/` | 7 | `provisioner_test.go` (763) | Container provisioning |
| `config/` | 6 | `config_test.go` (962) | Configuration management |
| `watch/` | 4 | `watcher_test.go` (671) | File watching |
| `tuiserver/` | 4 | `server_test.go` (543) | TUI server client/server |
| `sshserver/` | 3 | `server_test.go` (631) | SSH server |
| `uroot/` | 30 | `registry_test.go` (916) | u-root utility implementations |
| `issue/` | 2 | `issue_test.go` (470) | Error handling and issue templates |
| `benchmark/` | 4 | `benchmark_test.go` (694) | PGO profile benchmarks |
| `app/execute/` | 2 | `orchestrator_test.go` (678) | Execution orchestration |
| `core/serverbase/` | 1 | `base_test.go` (701) | Server state machine |
| `testutil/` | 4 | `clock_test.go` (284) | Test utility helpers |
| `testutil/invowkfiletest/` | 1 | `command_test.go` (464) | Invowkfile test factory |
| `agentcmd/` | 2 | `create_test.go` (302) | Agent command helpers |
| `auditllm/` | 2 | `provider_test.go` (740) | Audit LLM provider plumbing |
| `containerplan/` | 1 | `persistent_test.go` (166) | Persistent container planning |
| `tuiwire/` | 1 | `tui_context_test.go` (100) | TUI context wiring |

### `pkg/` (100 files across 7 subdirectories)

| Subdirectory | Files | Largest File (lines) | Focus |
|---|---|---|---|
| `invowkfile/` | 62 | `validation_test.go` (873) | Invowkfile parsing, validation, sync |
| `invowkmod/` | 22 | `lockfile_test.go` (910) | Module metadata, operations, locking |
| `types/` | 8 | `listen_port_test.go` (90) | DDD value type tests |
| `cueutil/` | 3 | `parse_test.go` (373) | CUE utility tests |
| `platform/` | 2 | `sandbox_test.go` (223) | Platform detection tests |
| `fspath/` | 2 | `fspath_test.go` (104) | Filesystem path tests |
| `containerargs/` | 1 | `container_name_test.go` (67) | Container argument value types |

### `tests/cli/` (6 test files plus support files)

| File | Focus |
|---|---|
| `cmd_test.go` | Main testscript runner, shared setup, and testscript conditions |
| `cmd_container_test.go` | Container testscript runner with suite lock + cleanup |
| `container_harness.go` | Container engine selection, smoke probes, and cleanup helpers |
| `container_harness_test.go` | Container harness decision and smoke-probe unit tests |
| `testscript_env_test.go` | testscript environment setup and isolation helpers |
| `tui_tmux_test.go` | tmux-based TUI e2e tests |
| `runtime_mirror_test.go` | Virtual/native mirror coverage enforcement |

### `tools/goplint/` (60 files)

| Subdirectory | Files | Largest File (lines) | Focus |
|---|---|---|---|
| `goplint/` | 57 | `integration_supplementary_test.go` (807) | Analyzer tests, CFA, baseline, integration |
| Root | 3 | — | Entry point tests |

---

## Testscript Files (`tests/cli/testdata/`)

### By Category

#### Virtual-Sh Runtime Tests (52 files)

| File | Lines | Mirror Status |
|---|---|---|
| `virtual_flags.txtar` | 184 | Has native mirror |
| `virtual_disambiguation.txtar` | 176 | Has native mirror |
| `virtual_args.txtar` | 162 | Has native mirror |
| `virtual_deps_files.txtar` | 128 | Has native mirror |
| `virtual_env.txtar` | 100 | Has native mirror |
| `virtual_edge_cases.txtar` | 97 | Exempt: CUE validation only |
| `virtual_deps_tools.txtar` | 78 | Has native mirror |
| `virtual_deps_env.txtar` | 78 | Has native mirror |
| `virtual_multi_source_full.txtar` | 73 | Has native mirror |
| `virtual_multi_source.txtar` | 71 | Has native mirror |
| `virtual_vendored_execution.txtar` | 67 | Has native mirror |
| `virtual_runtime_override.txtar` | 67 | Exempt: cross-runtime override |
| `virtual_simple.txtar` | 62 | Has native mirror |
| `virtual_deps_cmds.txtar` | 62 | Has native mirror |
| `virtual_ambiguity.txtar` | 56 | Has native mirror |
| `virtual_deps_custom_error.txtar` | 56 | Has native mirror |
| `virtual_deps_custom.txtar` | 55 | Has native mirror |
| `virtual_deps_caps.txtar` | 54 | Has native mirror |
| `virtual_dryrun.txtar` | 51 | Has native mirror |
| `virtual_deps_tools_error.txtar` | 49 | Has native mirror |
| `virtual_deps_root.txtar` | 48 | Has native mirror |
| `virtual_deps_env_error.txtar` | 48 | Has native mirror |
| `virtual_isolation.txtar` | 46 | Exempt: command names include runtime name |
| `virtual_category.txtar` | 45 | Has native mirror |
| `virtual_env_cli_override.txtar` | 43 | Has native mirror |
| `virtual_deps_impl.txtar` | 41 | Has native mirror |
| `virtual_verbose.txtar` | 40 | Has native mirror |
| `virtual_timeout.txtar` | 38 | Has native mirror |
| `virtual_diagnostics_footer.txtar` | 37 | Exempt: diagnostics formatting |
| `virtual_shell.txtar` | 36 | Exempt: virtual-shell-specific |
| `virtual_watch.txtar` | 27 | Has native mirror |
| `virtual_args_subcommand_conflict.txtar` | 27 | Exempt: CUE validation only |
| `virtual_deps_runtime.txtar` | 29 | Has native mirror |
| `virtual_deps_cmds_error.txtar` | 28 | Has native mirror |
| `virtual_deps_caps_error.txtar` | 28 | Has native mirror |
| `virtual_deps_tools_platform.txtar` | 26 | Has native mirror |
| `virtual_uroot_combined_flags.txtar` | 161 | Exempt: u-root |
| `virtual_uroot_text_ops.txtar` | 84 | Exempt: u-root |
| `virtual_uroot_error_handling.txtar` | 54 | Exempt: u-root |
| `virtual_uroot_tee.txtar` | 52 | Exempt: u-root |
| `virtual_uroot_seq.txtar` | 48 | Exempt: u-root |
| `virtual_uroot_basename_dirname.txtar` | 48 | Exempt: u-root |
| `virtual_uroot_file_ops.txtar` | 46 | Exempt: u-root |
| `virtual_uroot_ln.txtar` | 45 | Exempt: u-root |
| `virtual_uroot_mktemp.txtar` | 44 | Exempt: u-root |
| `virtual_uroot_gzip.txtar` | 44 | Exempt: u-root |
| `virtual_uroot_find.txtar` | 44 | Exempt: u-root |
| `virtual_uroot_shasum.txtar` | 41 | Exempt: u-root |
| `virtual_uroot_realpath.txtar` | 41 | Exempt: u-root |
| `virtual_uroot_base64.txtar` | 41 | Exempt: u-root |
| `virtual_uroot_basic.txtar` | 35 | Exempt: u-root |
| `virtual_uroot_sleep.txtar` | 31 | Exempt: u-root |

#### Native Runtime Tests (32 files)

| File | Lines | Virtual Counterpart |
|---|---|---|
| `native_flags.txtar` | 286 | `virtual_flags.txtar` |
| `native_disambiguation.txtar` | 265 | `virtual_disambiguation.txtar` |
| `native_args.txtar` | 233 | `virtual_args.txtar` |
| `native_deps_files.txtar` | 167 | `virtual_deps_files.txtar` |
| `native_env.txtar` | 146 | `virtual_env.txtar` |
| `native_deps_env.txtar` | 110 | `virtual_deps_env.txtar` |
| `native_deps_tools.txtar` | 106 | `virtual_deps_tools.txtar` |
| `native_multi_source.txtar` | 105 | `virtual_multi_source.txtar` |
| `native_ambiguity.txtar` | 101 | `virtual_ambiguity.txtar` |
| `native_multi_source_full.txtar` | 94 | `virtual_multi_source_full.txtar` |
| `native_deps_cmds.txtar` | 90 | `virtual_deps_cmds.txtar` |
| `native_vendored_execution.txtar` | 89 | `virtual_vendored_execution.txtar` |
| `native_simple.txtar` | 86 | `virtual_simple.txtar` |
| `native_runtime_override.txtar` | 77 | — |
| `native_deps_custom.txtar` | 71 | `virtual_deps_custom.txtar` |
| `native_deps_caps.txtar` | 71 | `virtual_deps_caps.txtar` |
| `native_deps_custom_error.txtar` | 70 | `virtual_deps_custom_error.txtar` |
| `native_isolation.txtar` | 66 | — |
| `native_category.txtar` | 66 | `virtual_category.txtar` |
| `native_dryrun.txtar` | 64 | `virtual_dryrun.txtar` |
| `native_deps_tools_error.txtar` | 63 | `virtual_deps_tools_error.txtar` |
| `native_deps_root.txtar` | 62 | `virtual_deps_root.txtar` |
| `native_deps_env_error.txtar` | 62 | `virtual_deps_env_error.txtar` |
| `native_deps_impl.txtar` | 61 | `virtual_deps_impl.txtar` |
| `native_env_cli_override.txtar` | 55 | `virtual_env_cli_override.txtar` |
| `native_timeout.txtar` | 53 | `virtual_timeout.txtar` |
| `native_verbose.txtar` | 45 | `virtual_verbose.txtar` |
| `native_deps_cmds_error.txtar` | 35 | `virtual_deps_cmds_error.txtar` |
| `native_deps_tools_platform.txtar` | 33 | `virtual_deps_tools_platform.txtar` |
| `native_deps_caps_error.txtar` | 35 | `virtual_deps_caps_error.txtar` |
| `native_watch.txtar` | 32 | `virtual_watch.txtar` |
| `native_deps_runtime.txtar` | 29 | `virtual_deps_runtime.txtar` |

#### Container Runtime Tests (8 files)

| File | Lines |
|---|---|
| `container_provision.txtar` | 97 |
| `container_callback.txtar` | 96 |
| `container_exitcode.txtar` | 88 |
| `container_env.txtar` | 83 |
| `container_args.txtar` | 70 |
| `container_basic.txtar` | 61 |
| `container_dockerfile.txtar` | 55 |
| `container_unsupported_images.txtar` | 41 |

#### Built-in Command Tests (29 files)

| File | Lines | Category |
|---|---|---|
| `validate.txtar` | 225 | Validation |
| `module_vendor.txtar` | 137 | Module operations |
| `config_set.txtar` | 66 | Configuration |
| `init_templates.txtar` | 56 | Init |
| `config_override_flag.txtar` | 53 | Configuration |
| `module_remove_happy.txtar` | 47 | Module operations |
| `module_import.txtar` | 44 | Module operations |
| `module_archive.txtar` | 38 | Module operations |
| `config_show.txtar` | 38 | Configuration |
| `init_default.txtar` | 36 | Init |
| `module_create.txtar` | 34 | Module operations |
| `config_dump.txtar` | 32 | Configuration |
| `version_help.txtar` | 31 | Version/help |
| `completion.txtar` | 28 | Shell completion |
| `module_sync_happy.txtar` | 27 | Module operations |
| `module_tidy.txtar` | 26 | Module operations |
| `config_init.txtar` | 26 | Configuration |
| `module_list.txtar` | 25 | Module operations |
| `module_add_remove.txtar` | 24 | Module operations |
| `audit_clean.txtar` | 25 | Audit |
| `audit_severity.txtar` | 23 | Audit |
| `audit_findings.txtar` | 22 | Audit |
| `audit_json.txtar` | 21 | Audit |
| `config_path.txtar` | 24 | Configuration |
| `tui_format.txtar` | 22 | TUI formatting |
| `module_sync_update.txtar` | 21 | Module operations |
| `tui_style.txtar` | 20 | TUI styling |
| `dogfooding_invowkfile.txtar` | 18 | Dogfooding |
| `module_deps.txtar` | 14 | Module dependencies |

---

## Virtual/Native Pairing Summary

| Category | Count | Status |
|---|---|---|
| Virtual with native mirror | 31 | Complete pairs |
| Virtual exempt (u-root) | 16 | No mirror needed |
| Virtual exempt (other) | 5 | CUE validation, diagnostics, shell-specific, cross-runtime override |
| Native without virtual | 0 | All native runtime txtar files map to a virtual counterpart |
| Container tests | 8 | Exempt (Linux-only) |
| Built-in command tests | 29 | Exempt (CLI handlers, not runtimes) |

Machine-readable exemptions: `tests/cli/runtime_mirror_exemptions.json`

---

## Largest Test Files

Current `*_test.go` files over the 900-line soft monitor threshold:

| File | Lines | Status |
|---|---|---|
| `internal/runtime/runtime_sh_test.go` | 991 | Split before adding more cases |
| `internal/discovery/discovery_collisions_test.go` | 971 | Split before adding more cases |
| `internal/config/config_test.go` | 962 | Split before adding more cases |
| `internal/app/deps/checks_test.go` | 960 | Split before adding more cases |
| `internal/uroot/registry_test.go` | 916 | Monitor |
| `pkg/invowkmod/lockfile_test.go` | 910 | Monitor |
