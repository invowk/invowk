# Test File Inventory

Deterministic enumeration of all test files in the repository. Subagents traverse exact listed
files — no sampling. This file should be regenerated when test files are added or removed.

**Last generated**: 2026-03-26

**Totals**: ~335 `*_test.go` files, ~116 `.txtar` files, ~99,579 lines of test code.

---

## Go Test Files by Directory

### `cmd/invowk/` (27 files)

CLI adapter tests. Key files:

| File | Lines | Focus |
|---|---|---|
| `coverage_test.go` | 425 | Built-in command txtar coverage guardrail |
| `cmd_deps_filepath_test.go` | 798 | Filepath dependency validation |
| `cmd_args_test.go` | 633 | CLI argument handling |
| `cmd_deps_caps_env_test.go` | 561 | Capability and env dependency tests |
| `cmd_validate_runtime_deps_test.go` | 559 | Runtime dependency validation |
| `cmd_deps_test.go` | 555 | Dependency CLI handler tests |

### `internal/` (173 files across 19 subdirectories)

| Subdirectory | Files | Largest File (lines) | Focus |
|---|---|---|---|
| `runtime/` | 27 | `runtime_virtual_test.go` (784) | All three runtimes (native, virtual, container) |
| `tui/` | 24 | `filter_test.go` (672) | Bubble Tea model state transitions |
| `container/` | 19 | `engine_docker_mock_test.go` (793) | Docker/Podman engine mocks and types |
| `discovery/` | 11 | `discovery_core_test.go` (815) | Module/command discovery |
| `app/commandsvc/` | 7 | — | Command execution service |
| `app/deps/` | 5 | `checks_test.go` (591) | Dependency validation |
| `provision/` | 5 | `provisioner_test.go` (774) | Container provisioning |
| `config/` | 4 | `config_test.go` (999) | Configuration management |
| `watch/` | 4 | `watcher_test.go` (521) | File watching |
| `tuiserver/` | 4 | `client_test.go` (483) | TUI server client/server |
| `sshserver/` | 3 | `server_test.go` (554) | SSH server |
| `uroot/` | 30 | `registry_test.go` (806) | u-root utility implementations |
| `issue/` | 2 | `issue_test.go` (583) | Error handling and issue templates |
| `benchmark/` | 2 | `benchmark_test.go` (854) | PGO profile benchmarks |
| `app/execute/` | 2 | `orchestrator_test.go` (607) | Execution orchestration |
| `core/serverbase/` | 1 | `base_test.go` (578) | Server state machine |
| `testutil/` | 2 | — | Test utility helpers |
| `testutil/invowkfiletest/` | 1 | — | Invowkfile test factory |

### `pkg/` (90 files across 6 subdirectories)

| Subdirectory | Files | Largest File (lines) | Focus |
|---|---|---|---|
| `invowkfile/` | 54 | `validation_test.go` (831) | Invowkfile parsing, validation, sync |
| `invowkmod/` | 24 | `invowkmod_test.go` (779) | Module metadata, operations, locking |
| `types/` | 6 | — | DDD value type tests |
| `cueutil/` | 3 | — | CUE utility tests |
| `platform/` | 2 | — | Platform detection tests |
| `fspath/` | 1 | — | Filesystem path tests |

### `tests/cli/` (5 files)

| File | Focus |
|---|---|
| `cli_test.go` | Main testscript runner for non-container CLI tests |
| `container_test.go` | Container testscript runner with semaphore + cleanup |
| `tui_tmux_test.go` | tmux-based TUI e2e tests |
| `runtime_mirror_test.go` | Virtual/native mirror coverage enforcement |
| `helpers_test.go` | Shared testscript helpers and conditions |

### `tools/goplint/` (59 files)

| Subdirectory | Files | Largest File (lines) | Focus |
|---|---|---|---|
| `goplint/` | 56 | `main_test.go` (848) | Analyzer tests, CFA, baseline, integration |
| Root | 3 | — | Entry point tests |

---

## Testscript Files (`tests/cli/testdata/`)

### By Category

#### Virtual Runtime Tests (52 files)

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

#### Built-in Command Tests (24 files)

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
| `config_init.txtar` | 26 | Configuration |
| `module_list.txtar` | 25 | Module operations |
| `module_add_remove.txtar` | 24 | Module operations |
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
| Virtual with native mirror | ~26 | Complete pairs |
| Virtual exempt (u-root) | ~16 | No mirror needed |
| Virtual exempt (other) | ~6 | CUE validation, diagnostics, shell-specific, etc. |
| Native without virtual | ~4 | `native_runtime_override`, `native_isolation`, etc. |
| Container tests | 9 | Exempt (Linux-only) |
| Built-in command tests | 29 | Exempt (CLI handlers, not runtimes) |

Machine-readable exemptions: `tests/cli/runtime_mirror_exemptions.json`

---

## Files Over 800 Lines (Soft Limit)

These files are at or near the 800-line test file limit:

| File | Lines | Status |
|---|---|---|
| `internal/config/config_test.go` | 999 | Near hard limit |
| `internal/config/sync_test.go` | 867 | Over soft limit |
| `internal/benchmark/benchmark_test.go` | 854 | Over soft limit |
| `tools/goplint/main_test.go` | 848 | Over soft limit |
| `pkg/invowkfile/validation_test.go` | 831 | Over soft limit |
| `internal/discovery/discovery_core_test.go` | 815 | Over soft limit |
| `internal/uroot/registry_test.go` | 806 | Over soft limit |
