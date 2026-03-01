# Rust Conversion — Remaining Items (v2)

> **STATUS: PENDING** — identified during the 2026-03-01 TUI implementation session.
> Current state: ~53K lines, 1022 tests, all files under 1000 lines.
> Previous items (v1) all complete: TUI standalone components, container dep runner,
> RuntimeConfig.depends_on, CUE parser module wiring.

## 1. Interactive PTY Execution Mode (TUI)

The 10 standalone TUI components are complete, but the **interactive execution mode** —
where a running command can request TUI overlays via the tuiserver HTTP protocol — is not
yet implemented. This is the most architecturally complex TUI piece.

**Go reference:** `internal/tui/interactive.go`, `interactive_model.go`, `interactive_exec.go`,
`interactive_helpers.go`, `embeddable.go` (~46KB across 5 files).

**What needs to be built:**

### 1a. EmbeddableComponent Trait

Rust equivalent of Go's `EmbeddableComponent` interface. Each of the 10 existing components
needs a second constructor mode (`new_for_modal()`) that renders into a provided `Rect`
rather than owning the full terminal.

```
trait EmbeddableComponent {
    fn handle_event(&mut self, event: Event) -> bool;  // true = consumed
    fn render(&self, frame: &mut Frame, area: Rect);
    fn is_done(&self) -> bool;
    fn result(&self) -> ComponentResult;
    fn cancelled(&self) -> bool;
    fn set_size(&mut self, width: u16, height: u16);
}
```

- File: `adapters/tui/embeddable.rs`
- `create_embeddable_component()` factory dispatching by `Component` enum
- `calculate_modal_size()` — type-aware sizing (simple prompts get 4 lines, pager gets full height)

### 1b. Modal Overlay Rendering

Ratatui approach is simpler than Go's ANSI string splicing:
1. Render the base content (PTY output viewport) to the full frame
2. Calculate centered `Rect` for the modal
3. Render `Clear` widget to erase the base in that area
4. Render a `Block` with rounded border + modal background (`#1a1a2e`)
5. Render the embeddable component inside the block's inner area

- File: `adapters/tui/overlay.rs`
- No `sanitizeModalBackground()` needed — Ratatui's cell buffer avoids ANSI color bleed

### 1c. Interactive Execution Model

State machine with PTY bridge:
- States: `Executing` → `Tui` → `Executing` (cycles) → `Completed`
- PTY output read in a background thread, forwarded via `mpsc::channel`
- Viewport: `Paragraph` with `.scroll()` for scrollable output buffer
- `TUIComponentMsg` equivalent: when the child process sends an HTTP request to the
  tuiserver, the server forwards it to the interactive model, which creates an
  `EmbeddableComponent` and switches to `Tui` state

**Key files:**
- `adapters/tui/interactive.rs` — `InteractiveModel` implementing `TuiApp`
- `adapters/tui/interactive_exec.rs` — `run_interactive_cmd()` (PTY setup, goroutine equiv)

**Dependencies:**
- `nix 0.29` (already present) for PTY allocation
- `ratatui::crossterm` for terminal management
- Existing `TuiServer` for HTTP delegation

**Estimated scope:** ~800 lines across 3-4 new files. Complex but well-bounded.

---

## 2. CLI Integration Tests

The Rust codebase has 1022 unit tests but **zero integration/CLI tests**. The Go codebase
has 107 `.txtar` test files in `tests/cli/testdata/`.

**Recommended framework:** `assert_cmd` (v2) + `predicates` (v3) + `tempfile` (already a dev-dep).

**Rationale:** `assert_cmd` wraps `std::process::Command` with `cargo_bin()` for automatic
binary discovery, composable predicate assertions (`stdout(contains("hello"))`), and timeout
support. Rejected alternatives: `trycmd` (lacks programmatic flexibility for complex fixtures),
custom `Command` wrapper (duplicates assert_cmd).

### 2a. Test Infrastructure

**New dev-dependencies:**
```toml
[dev-dependencies]
assert_cmd = "2"
predicates = "3"
```

**Directory structure** (single integration test binary for faster linking):
```
tests/
  cli_tests.rs           # Entry point: mod cli;
  cli/
    mod.rs               # Re-exports test submodules
    common.rs            # TestWorkspace, container_available(), skip macros, CUE constants
    version_help.rs      # --version, --help, cmd --help
    config.rs            # config show/dump/init/set/path
    init.rs              # invowk init
    validate.rs          # invowkfile/module validation
    completion.rs        # shell completion generation
    virtual_simple.rs    # basic virtual runtime execution
    virtual_args.rs      # positional argument handling
    virtual_flags.rs     # flag handling
    virtual_env.rs       # environment variable configuration
    container.rs         # container tests (skip_unless_container!)
```

### 2b. TestWorkspace Pattern

Equivalent of Go's testscript `$WORK`:
```rust
pub struct TestWorkspace {
    pub dir: TempDir,
}
impl TestWorkspace {
    pub fn new() -> Self;
    pub fn write_file(&self, path: &str, content: &str);
    pub fn write_invowkfile(&self, content: &str);
    pub fn invowk_cmd(&self) -> assert_cmd::Command;  // pre-configured with HOME, XDG_CONFIG_HOME
}
```

### 2c. Custom Conditions

- `container_available()` — smoke-tests `debian:stable-slim echo ok` with Docker/Podman
- `skip_unless_container!()` macro — early return if no container runtime
- `#[cfg(target_os)]` for platform conditions (replaces txtar `[linux]`, `[windows]`)

### 2d. Porting Priority

| Batch | Files | Tests | Coverage |
|-------|-------|-------|----------|
| 1 (highest) | version_help, config, init, validate, completion, virtual_simple, virtual_args | ~47 | Core user journey |
| 2 | virtual_flags, virtual_env, virtual_dryrun, error_handling | ~30 | Feature coverage |
| 3 | container, virtual_discovery, module_commands | ~20 | Advanced features |
| 4 | Remaining virtual tests, native mirrors, TUI tests | ~40+ | Full parity |

### 2e. CUE Fixture Handling

CUE fixtures work as-is — the Rust binary uses the same CUE parser (FFI or WASM). Inline
CUE content in test functions using `r#"..."#` raw strings, matching the Go txtar pattern
where files are embedded directly in the test.

**Estimated scope:** ~1200 lines across 12 files for Batch 1. Framework setup is ~200 lines.

---

## 3. Minor Cleanup (Optional)

These are lower-priority items identified during the TUI simplify review:

- **Shared `ScrollState` struct** — deduplicate `move_up()`/`move_down()` across choose, filter,
  file_picker, table (~40 lines saved)
- **Shared `render_header()`** — deduplicate title+description rendering across 6 components
- **Shared `has_tty()` helper** in CLI tui module — deduplicate TTY check across 8 commands
- **`ChooseResult.selected`** uses `serde_json::Value` — could use a typed `Selection` enum
  instead, but this matches the Go wire protocol format
