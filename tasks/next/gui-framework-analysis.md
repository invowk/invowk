# Cross-Platform GUI for Invowk: Framework Analysis

## Context

Invowk needs a GUI that lets users **browse discovered commands and run them interactively** — with flags, arguments, runtime selectors, and dependency status automatically rendered as form fields. The GUI must work on Linux, macOS, and Windows. A strong preference exists for **no CGO** to preserve the current clean single-binary distribution (`go install`, GoReleaser, install scripts).

---

## The CGO Reality

Every framework that opens a real OS window on **Linux** requires CGO for at least one of: GTK/X11/Wayland windowing, OpenGL/Vulkan graphics, or WebKit. The only exceptions today are:

| Approach | How it avoids CGO on Linux |
|---|---|
| **modernc.org/tk9.0** | Transpiles Tcl/Tk C source to Go via the modernc C-to-Go transpiler |
| **go-app** | Compiles to WASM, runs in a browser — no OS window |
| **Bubble Tea / Huh** | Terminal-only — no OS window |
| **Lorca** | Launches installed Chrome — hard runtime dependency (non-starter) |

On **Windows**, several frameworks are CGO-free (Wails v3, Gio, tk9.0) because they use `syscall`/`golang.org/x/sys/windows` directly. On **macOS**, almost everything needs CGO for Cocoa/AppKit.

---

## Framework Comparison

### Tier 1: Fully CGO-Free (all 3 platforms)

#### 1. modernc.org/tk9.0 — CGO-Free Native Tk

- **How:** Tcl/Tk 9.0 C source transpiled to pure Go via `modernc` (same approach as modernc's CGO-free SQLite)
- **CGO:** None on any platform. `go install` and `GOOS=... go build` just work
- **Widgets:** Real Tk widgets — text inputs, dropdowns, checkboxes, radio buttons, spinboxes, file dialogs, tabs, trees, notebooks. Near-native appearance (uses OS theme engine: Aqua on macOS, Windows controls on Windows, GTK theme on Linux)
- **Maturity:** Announced Sep 2024, v0.65.0 by Mar 2025. **New and not yet battle-tested.** The underlying Tcl/Tk is 30+ years stable; risk is in the Go wrapper layer
- **Distribution:** Single binary, no external dependencies, tiny overhead
- **Dark mode:** Supported via Azure Fluent-like theme
- **Look & feel:** Functional but visually dated compared to modern UIs. Tk 9.0 improves on classic Tk but is still recognizably "Tk"
- **Forms:** Excellent built-in form widgets (Entry, Spinbox, Combobox, Checkbutton, Radiobutton, Scale, Labelframe for grouping)

**Verdict:** The most promising CGO-free option for a real desktop GUI. The maturity risk is real but the foundation (Tcl/Tk) is rock-solid. Best fit if "looks native-ish" matters more than "looks modern."

#### 2. go-app (WASM + Browser PWA)

- **How:** Compiles Go to WebAssembly, serves via Go HTTP server, runs in browser or as installed PWA
- **CGO:** None. Standard `GOARCH=wasm GOOS=js` tooling
- **Widgets:** HTML/CSS — full control over appearance
- **Maturity:** Active, used in production by Dagger (Cloud v3 UI)
- **Distribution:** Requires a running Go HTTP server + browser. Not a standalone desktop app — fundamentally a "local web app" experience
- **WASM binary size:** Large (Go WASM output is not small; 10-30MB+ for complex apps)
- **Look & feel:** Web aesthetic. Can look very modern with the right CSS

**Verdict:** Viable if the GUI is framed as `invowk gui` launching a local web server and opening a browser tab. Awkward for users expecting a native window. The server+browser model adds friction.

#### 3. Bubble Tea + Huh (Terminal TUI) — Already Integrated

- **How:** Fullscreen terminal UI using Charm libraries
- **CGO:** None. Pure Go
- **Widgets:** Text inputs, selects, multi-selects, confirms, file pickers (via `huh`). Tables, lists, viewports, spinners (via `bubbles`)
- **Maturity:** Extremely mature. Already used in Invowk's TUI components
- **Distribution:** Zero additional dependencies — part of the existing binary
- **Look & feel:** Terminal aesthetic. Cannot look like a native GUI. Works over SSH
- **Gap:** No pre-execution form for collecting flags/args exists today — the current TUI components are used for in-execution prompts (input, confirm, choose, etc.)

**Verdict:** The lowest-risk, lowest-effort option. A fullscreen `invowk tui` command that renders a command browser + form builder using `huh` forms would cover 80% of the use case with zero new dependencies. Already proven in the codebase.

### Tier 2: CGO Required on Linux/macOS (CGO-free on Windows only)

#### 4. Wails v3 (Go + Web Frontend via System WebView)

- **CGO:** None on Windows (v3 improvement). **Required** on Linux (GTK + WebKit2GTK) and macOS (WKWebView/Cocoa)
- **Frontend:** React/Vue/Svelte/vanilla JS — full web stack flexibility
- **Maturity:** v2 stable/production; v3 in alpha but "running in production" by some teams
- **Distribution:** Single binary with embedded assets. No browser bundled (uses OS WebView)
- **Cross-compilation:** Hard. Linux→macOS unsupported. Linux arm64 cross-compilation broken
- **TypeScript bindings:** Auto-generated from Go structs

**Verdict:** Best option if CGO is acceptable and the team wants a modern web-based UI. The web frontend adds significant build complexity (Node.js, npm, bundler). Cross-compilation constraints conflict with GoReleaser's multi-platform builds.

#### 5. Gio (Immediate-Mode GPU-Rendered)

- **CGO:** None on Windows. **Required** on Linux (X11/Wayland + OpenGL) and macOS (Cocoa)
- **Paradigm:** Immediate-mode (no retained widget tree). Steeper learning curve
- **Widgets:** Core is minimal; `gio-v` third-party library provides Material 3 widgets
- **Maturity:** Active, funded via OpenCollective, v0.9.0 in 2025
- **Performance:** Excellent (GPU vector path rendering)
- **Distribution:** Small binary, no external runtime

**Verdict:** Powerful but unfamiliar paradigm for form-heavy UI. The thin built-in widget set means relying on third-party libraries for basic form controls.

### Tier 3: CGO Required Everywhere

#### 6. Fyne (Material Design GUI Toolkit)

- **CGO:** **Required on ALL platforms** (OpenGL via CGO bindings)
- **Widgets:** Most comprehensive built-in set: text input, password, numeric, select, checkbox, radio, slider, progress, dialogs, tabs, lists, tables, trees
- **Maturity:** Most popular Go GUI toolkit. Used by Tailscale. Has a published book
- **Distribution:** Self-contained executables. Docker-based cross-compilation via `fyne-cross`
- **Look & feel:** Material Design — consistent but non-native on all platforms

**Verdict:** If CGO is acceptable everywhere, Fyne has the best widget coverage and ecosystem maturity. The `fyne-cross` Docker workflow adds build complexity.

### Not Recommended

| Framework | Why Not |
|---|---|
| **Lorca** | Requires Chrome installed. Unmaintained |
| **webview/go-webview** | CGO everywhere. Too low-level (no framework) |
| **Tauri** | No Go support yet (Rust-only backend) |
| **Cogent Core** | Still maturing, less third-party production use |
| **guigui (Ebitengine)** | Alpha stage, CGO on Linux/macOS |

---

## Summary Comparison Table

| Framework | CGO-Free? | Linux CGO? | macOS CGO? | Windows CGO? | Maturity | Forms/Input | Cross-Compile | Native Look | Bundle Size |
|---|---|---|---|---|---|---|---|---|---|
| **tk9.0** | Yes | None | None | None | New (2024) | Tk widgets | Easy (`go build`) | Near-native | Small |
| **go-app** | Yes | None | None | None | Active | HTML/WASM | Easy | No (web) | Large (WASM) |
| **Bubble Tea** | Yes | None | None | None | Very Mature | Huh library | Easy | Terminal | Tiny |
| **Wails v3** | Partial | Required | Required | None (v3) | Production | Web/HTML | Hard | No (web) | Small |
| **Gio** | Partial | Required | Required | None | Production | Via third-party | Moderate | No (custom) | Small |
| **Fyne** | No | Required | Required | Required | Very Mature | Excellent | Hard (fyne-cross) | No (Material) | Medium |

---

## What Invowk's GUI Must Render

Based on the command model in `pkg/invowkfile/`, a GUI needs to render:

### Per-Command Form Fields

| Source | Widget Type | Details |
|---|---|---|
| `Flag` (type=string) | Text input | With optional regex validation overlay |
| `Flag` (type=bool) | Checkbox/toggle | Values are `"true"` / `"false"` strings |
| `Flag` (type=int) | Numeric input (integer) | Optional leading `-` |
| `Flag` (type=float) | Numeric input (decimal) | `strconv.ParseFloat` compatible |
| `Argument` (positional) | Ordered text inputs | Same type system minus bool |
| `Argument` (variadic) | Multi-value input | Only the last arg; tag input or repeatable field |
| `Required` marker | Mandatory field indicator | `Required` and `DefaultValue` are mutually exclusive |
| `Validation` regex | Real-time field validation | Go `regexp.Compile`-compatible patterns |
| `DefaultValue` | Pre-filled value | Applied during `resolveDefinitions()` |
| `Short` flag | Display label | Show as `-x / --name` |

### Command Browser / Navigation

- Commands with spaces in names (`"test unit"`) form a subcommand hierarchy
- Module-sourced commands show module ID prefix (`io.invowk.sample deploy`)
- Platform filtering: gray out or hide commands not available on current OS
- Ambiguous commands (same `SimpleName` across modules) need disambiguation UI

### Runtime & Platform Selectors

- Runtime dropdown: `native` / `virtual` / `container` (filtered by `GetAllowedRuntimesForPlatform`)
- Default pre-selected via `GetDefaultRuntimeForPlatform`
- Container-specific fields (image, volumes, ports) appear only when container is selected

### Dependency Status Panel

- `DependsOn` merged from root + command + implementation levels
- Show pre-flight check status for: tools, commands, filepaths, capabilities, env vars, custom checks
- OR semantics within `Alternatives` per entry

### Key Data Structures (source files)

- **Invowkfile / Command / Implementation:** `pkg/invowkfile/invowkfile.go`, `command.go`, `implementation.go`
- **Flag type system:** `pkg/invowkfile/flag.go` — `FlagType` enum: `string`, `bool`, `int`, `float`
- **Argument type system:** `pkg/invowkfile/argument.go` — `ArgumentType` enum: `string`, `int`, `float` (no bool)
- **Runtime / Platform:** `pkg/invowkfile/runtime.go` — `RuntimeMode` (`native`/`virtual`/`container`), `PlatformType` (`linux`/`macos`/`windows`)
- **Dependencies:** `pkg/invowkfile/dependency.go` — `DependsOn` struct with tools, commands, filepaths, capabilities, custom checks, env vars
- **Env var projection:** `pkg/invowkfile/env.go` — `FlagNameToEnvVar()`, `ArgNameToEnvVar()`
- **Input validation:** `pkg/invowkfile/validation_input.go` — `validateValueType()`, `validateValueWithRegex()`
- **CommandInfo (discovery):** `internal/discovery/` — carries `Name`, `SimpleName`, `SourceID`, `ModuleID`, `IsAmbiguous`

---

## Recommended Strategy (Tiered)

### Phase 1: Enhanced TUI (Zero new dependencies, immediate value)

Build `invowk tui run <cmd>` (or similar) that renders a fullscreen Bubble Tea form using `huh`:
- Auto-generates form fields from `Flag` and `Argument` definitions
- Pre-fills defaults, validates types + regex in real-time
- Shows command description, dependency status, runtime selector
- Executes the command and streams output in a viewport
- This is the **incremental path** — extends existing `internal/tui/` infrastructure

**Trade-off:** Terminal-only. No mouse-friendly GUI. Limited visual richness.

### Phase 2: Local Web UI (CGO-free, modern look)

Build `invowk gui` that starts a local HTTP server and opens the default browser:
- Go backend serves a REST/WebSocket API exposing the command model
- Lightweight web frontend (could be embedded via `embed.FS`)
- No framework dependency — just `net/http` + embedded HTML/JS/CSS
- Auto-generates HTML forms from the same command model
- Offers the rich visual experience (syntax highlighting, responsive layout, dark mode)
- Works on all platforms without CGO

**Trade-off:** Requires a browser. "Open browser tab" UX isn't as polished as a native window. Port conflicts possible. But this is how many developer tools work (Jupyter, Storybook, Grafana, k9s alternatives).

### Phase 3 (Future): Native Desktop Window

If/when `modernc.org/tk9.0` matures (or a better CGO-free option emerges):
- Wrap the same command model API with a native GUI
- `tk9.0` would give real OS windows with native-ish widgets, no CGO, single binary
- Evaluate maturity annually — the transpiled Tcl/Tk approach is sound but the Go wrapper is young

**Trade-off:** Tk aesthetic is dated. But functional and truly cross-platform without CGO.

---

## Key Architectural Principle

Regardless of which rendering layer is chosen, the **command model API** should be a shared, framework-agnostic Go package that:
1. Discovers commands (via existing `internal/discovery/`)
2. Returns typed command metadata (flags, args, types, defaults, validation, deps)
3. Validates inputs (reusing `pkg/invowkfile/validation_input.go` logic)
4. Executes commands (via existing `commandService` pipeline)

This decouples the rendering layer from the domain logic, allowing all three phases to share the same backend. The TUI, web UI, and potential native GUI would all be thin rendering adapters over this shared API.

---

## Sources

- [Wails v3 Alpha Documentation](https://v3alpha.wails.io/)
- [The Road to Wails v3](https://wails.io/blog/the-road-to-wails-v3/)
- [Fyne Official](https://fyne.io/) / [Cross-Compiling Wiki](https://github.com/fyne-io/fyne/wiki/Cross-Compiling)
- [Gio UI](https://gioui.org/) / [Install Docs](https://gioui.org/doc/install)
- [go-app](https://go-app.dev/) / [Dagger: Replaced React with Go+WASM](https://dagger.io/blog/replaced-react-with-go)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [Huh](https://github.com/charmbracelet/huh)
- [modernc.org/tk9.0](https://pkg.go.dev/modernc.org/tk9.0) / [GitLab](https://gitlab.com/cznic/tk9.0)
- [Cogent Core](https://www.cogentcore.org/core/)
- [Ebitengine](https://ebitengine.org/) / [guigui](https://pkg.go.dev/github.com/hajimehoshi/guigui)
- [webview/webview](https://github.com/webview/webview)
