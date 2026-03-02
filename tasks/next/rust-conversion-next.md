# Rust Conversion — Next Items

> **STATUS: IN PROGRESS** — identified during the 2026-03-01 conversion session.
> Current state: ~53K lines, 1024 tests, all files under 1000 lines.
> Items 1, 3, 4, and 5 completed. Item 1 completed 2026-03-01 (standalone components only).

## 1. TUI Component Implementations (Ratatui) ✅ COMPLETE (standalone)

Completed 2026-03-01 (standalone components). Key changes:
- Replaced `iocraft = "0.5"` with `ratatui = "0.29"`, `tui-textarea`, `tui-input`, `throbber-widgets-tui`, `fuzzy-matcher`
- Created `adapters/tui/app.rs` — `TuiApp` trait, `AppAction`, `run_app()` (Elm architecture)
- Created `adapters/tui/theme.rs` — color constants matching Go's lipgloss theme
- Implemented all 10 standalone components in `adapters/tui/`:
  - `confirm.rs`, `input.rs`, `choose.rs`, `filter.rs`, `file_picker.rs`
  - `styled_write.rs`, `textarea.rs`, `spin.rs`, `pager.rs`, `table.rs`, `format.rs`
- Extended `ports/tui_component.rs` with `ComponentResult` enum, `StandaloneComponent` trait, expanded `TuiComponentFactory`
- Created `adapters/tui/factory.rs` — `RatatuiComponentFactory` creating all 10 component types
- Added all 9 missing tuiserver routes via unified `handle_generic_component` handler
- Updated all 10 CLI commands to use Ratatui components with stdin/stdout fallback for pipes

**Still deferred (separate batch):**
- `EmbeddableComponent` trait for modal overlays during interactive execution
- `RenderOverlay` — modal ANSI compositing (Clear + centered Rect in Ratatui)
- Interactive PTY execution mode (state machine with viewport + PTY bridge)

## 2. Integration / CLI Tests

No testscript (txtar) equivalents exist yet. Go has 27+ `.txtar` files in `tests/cli/testdata/`.

**Scope:**
- Choose a Rust CLI integration test framework (options: `assert_cmd` + `predicates`, or custom)
- Port the test infrastructure: binary build, `$WORK` isolation, custom conditions
- Port core test scenarios: command execution, discovery, config loading, error rendering
- Container-specific tests (require Docker/Podman availability condition)

**Go reference:** `tests/cli/cmd_test.go` (TestMain, TestCLI, commonSetup, commonCondition).

## 3. Phase 2 Container Dep Runner — Full Wiring ✅ COMPLETE

Completed 2026-03-01. Key changes:
- Created `ports/container_probe.rs` with `ContainerRunner`, `ContainerProbeRunnerFactory` traits
- `RegistryBuildResult` exposes `container_engine: Option<Arc<dyn ContainerEngine>>`
- `DefaultContainerProbeRunnerFactory` in `adapters/runtime/container_probe.rs`
- `ExecuteCommandUseCase` receives factory, wires Phase 2 automatically
- Fixed DDD layer violation (moved `ContainerRunner` from adapters to ports)

## 4. RuntimeConfig.depends_on Field ✅ COMPLETE

Completed 2026-03-01. Key changes:
- Added `depends_on: Option<DependsOn>` to `RuntimeConfig` with `#[serde(default)]`
- Validation in `structure_command.rs` warns on non-container runtime, validates content on container
- `execute_command.rs` extracts `container_deps` from `RuntimeConfig.depends_on`
- Serde handles deserialization automatically (no CUE parser changes needed)

## 5. CUE Parser Module Command Wiring ✅ COMPLETE

Completed 2026-03-01. Key changes:
- Created `CueInvowkmodLoader` adapter in `adapters/config/cue_invowkmod_loader.rs`
- Wired `DefaultCueParser` with `SchemaId::Invowkmod` into module `sync`, `vendor`, `list`, `validate` commands
- Removed `parse_requirements_simple` and `extract_field` hand-rolled parsers from `module/mod.rs` and `list.rs`
- Module commands converted to `async fn execute()` for `spawn_blocking(DefaultCueParser::new)`
- Fixed `cmd.rs` `enable_builtin_coreutils` hardcode — now reads from `AppConfig` via `CueAppConfigLoader`
- Implemented `config set` persistence — writes CUE dotted-path syntax to `~/.config/invowk/config.cue`
