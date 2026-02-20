# Declarative TUI Prompts (Future Feature)

> **Status:** Design document only. Not planned for implementation at this time.
> **Date:** 2026-02-20

## Summary

Extend the `#Argument` and `#Flag` CUE schema types with an optional `prompt` block that declares TUI component behavior. When a command is invoked and a required arg/flag is missing, Invowk automatically presents the appropriate TUI prompt instead of erroring.

## Chosen Design: Prompt-Enhanced Args/Flags

Prompts are **not** a separate top-level concept. Instead, they enhance the existing typed argument and flag system. This reuses the `INVOWK_ARG_*` / `INVOWK_FLAG_*` injection mechanism and avoids a second parallel variable namespace.

### Schema Shape

```cue
cmds: [{
    name: "deploy"
    args: [{
        name: "environment"
        type: "string"
        prompt: {
            type:    "choose"
            message: "Deploy where?"
            options: ["staging", "production"]
        }
    }]
    flags: [{
        name: "force"
        type: "bool"
        prompt: {
            type:    "confirm"
            message: "Force deploy?"
        }
    }]
    implementations: [{
        script: "deploy.sh $INVOWK_ARG_ENVIRONMENT"
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}]
```

### CUE Schema Addition

```cue
// PromptConfig defines interactive TUI prompt behavior for an arg or flag (optional).
// When the arg/flag value is not provided via CLI, the user is prompted interactively.
// The result is injected via the standard INVOWK_ARG_*/INVOWK_FLAG_* mechanism.
#PromptConfig: close({
    // type selects the TUI component to use
    type: "input" | "choose" | "confirm" | "write" | "filter" | "file"

    // message is the prompt text shown to the user (required)
    message: string & !="" & strings.MaxRunes(1024)

    // options provides choices for "choose" and "filter" types (required for those types)
    options?: [...string & !="" & strings.MaxRunes(256)] & [_, ...]

    // placeholder shows hint text in "input" and "write" types (optional)
    placeholder?: string & strings.MaxRunes(256)

    // char_limit limits input length for "input" type (optional)
    char_limit?: int & >0

    // extensions filters file types for "file" type (optional, e.g., [".go", ".ts"])
    extensions?: [...string & =~"^\\.[a-zA-Z0-9]+$"]
})
```

Add `prompt?` field to both `#Argument` and `#Flag`:

```cue
#Argument: close({
    // ... existing fields ...
    prompt?: #PromptConfig
})

#Flag: close({
    // ... existing fields ...
    prompt?: #PromptConfig
})
```

### Go Struct Addition

```go
// PromptConfig defines interactive TUI prompt behavior for an arg or flag.
type PromptConfig struct {
    Type        string   `json:"type"`
    Message     string   `json:"message"`
    Options     []string `json:"options,omitempty"`
    Placeholder string   `json:"placeholder,omitempty"`
    CharLimit   int      `json:"char_limit,omitempty"`
    Extensions  []string `json:"extensions,omitempty"`
}
```

Add to `Argument` and `Flag` structs:

```go
Prompt *PromptConfig `json:"prompt,omitempty"`
```

### Behavior

1. CLI parsing proceeds as normal (Cobra collects args and flags).
2. After parsing, any required arg/flag that is **missing** and has a `prompt` definition triggers the TUI component.
3. The prompt runs on the host terminal (through the existing TUI infrastructure).
4. Results are injected via the standard `INVOWK_ARG_*` / `INVOWK_FLAG_*` mechanism.
5. If `--ivk-interactive` is set, prompts render as modal overlays in the alternate screen.
6. If stdin is not a TTY (e.g., piped input), prompts are skipped and the missing arg/flag errors as usual.

### Key Design Decisions

- **Reuses existing injection**: No new `INVOWK_PROMPT_*` namespace. Args use `INVOWK_ARG_<NAME>`, flags use `INVOWK_FLAG_<NAME>`.
- **CLI always wins**: If a value is provided via CLI, the prompt is never shown.
- **Non-TTY safety**: Prompts only activate when stdin is a terminal. CI/scripts are unaffected.
- **Type checking**: The prompt result is validated against the arg/flag `type` and `validation` regex, just like CLI-provided values.
- **Container transparency**: Prompts work through the TUI server bridge when running in container runtime.

### Implementation Files (When Ready)

- `pkg/invowkfile/invowkfile_schema.cue` — add `#PromptConfig`, add `prompt?` to `#Argument` and `#Flag`
- `pkg/invowkfile/types.go` or new `pkg/invowkfile/prompt.go` — `PromptConfig` Go struct
- `pkg/invowkfile/argument.go` / `pkg/invowkfile/flag.go` — add `Prompt` field
- `cmd/invowk/cmd_execute.go` — prompt resolution after input validation, before exec context build
- `cmd/invowk/cmd_prompt.go` (new) — prompt orchestration using existing `internal/tui/` components
- `pkg/invowkfile/sync_test.go` — schema sync tests
- `tests/cli/testdata/` — txtar tests (TTY-dependent, may need tmux testing pattern)

### Open Questions for Future Implementation

1. **Prompt ordering**: When multiple args/flags need prompts, should they execute in declaration order or all at once (form-style)?
2. **Conditional prompts**: Should a prompt be able to reference the result of a prior prompt? (e.g., "Select region" → "Select server in $REGION")
3. **Default values in prompts**: If an arg has both `default_value` and `prompt`, which takes precedence when the arg is missing? (Proposed: prompt wins, but pre-fills with default.)
4. **Multi-select for variadic args**: Should `choose` with `multi_select: true` map naturally to variadic arguments?
