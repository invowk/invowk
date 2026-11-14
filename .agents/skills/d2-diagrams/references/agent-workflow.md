# Agent Workflow

This reference covers the validation and repair loop for agent-written D2, how to read D2
errors, and how the repository's CI checks diagrams. Behaviour below was observed with D2
v0.7.1, the version CI pins.

## Validation Pipeline

`d2 validate` only parses. Compile errors (unknown shape, invalid `direction`, invalid style
keyword, `_` at the root, unresolved `${var}`) pass validation with "is valid D2" and fail only
when the file is compiled. So the loop needs a compile check, and that check must write outside
the repository:

```bash
#!/bin/bash
set -euo pipefail

D2_FILE="$1"
SCRATCH="$(mktemp -d)"   # outside the repository; never render next to the source

# Stage 1: Format (canonicalize)
d2 fmt "$D2_FILE"

# Stage 2: Validate (parse errors only, fast)
d2 validate "$D2_FILE"

# Stage 3: Compile check (catches compile errors that validate misses)
d2 "$D2_FILE" "$SCRATCH/out.svg"   # uses the file's d2-config engine (TALA)
```

Without TALA, compile-check with `d2 --layout=elk "$D2_FILE" "$SCRATCH/out.svg"`: an explicit
`--layout` on the CLI overrides the file's `d2-config` engine. TALA-only features (such as
`near` pointing at another object) can fail under ELK even though they are valid under TALA.

Stage 4 is the committed render, and only after the compile check passes:

```bash
make render-diagrams
```

The render script owns engine probing, seeds, the output location
(`docs/diagrams/rendered/<dir>/<base>.svg`) and the source-hash stamp that
`make check-diagram-renders` verifies. ELK renders (`./scripts/render-diagrams.sh --allow-elk`)
are written to the same location for local preview only; never commit them.

**Why the stages:**
1. **Format** — normalizes whitespace, ordering, and syntax for deterministic diffs
2. **Validate** — fast parse check
3. **Compile check** — catches the errors validate cannot see, without touching the repository
4. **Render** — produces the committed, stamped SVGs

## Error Output Format

D2 v0.7.1 prints errors in three shapes, depending on the command:

```
err: oss.terrastruct.com/d2/d2cli.validateCmd: 1:17: unexpected text after unquoted string
err: failed to fmt: /abs/path/diagram.d2:1:17: unexpected text after unquoted string
err: failed to compile diagram.d2: /abs/path/diagram.d2:1:12: unknown shape "invalid_shape"
```

`d2 validate` uses the first shape (line and column only). `d2 fmt` also parses, so it fails
on the same parse errors with the second shape. Compiling uses the third.

### Parsing Errors Programmatically

```python
import re
from dataclasses import dataclass

D2_ERROR = re.compile(
    r'^err: (?:failed to (?:compile \S*?|fmt): \S*?:|\S*validateCmd: )'
    r'(?P<line>\d+):(?P<col>\d+): (?P<msg>.+)$'
)

@dataclass
class D2Error:
    line: int
    column: int
    message: str

def parse_d2_errors(stderr: str) -> list[D2Error]:
    """Parse D2 fmt, validate or compile error output into structured objects."""
    errors = []
    for line in stderr.strip().split('\n'):
        if match := D2_ERROR.match(line):
            errors.append(D2Error(
                line=int(match.group('line')),
                column=int(match.group('col')),
                message=match.group('msg'),
            ))
    return errors
```

## Self-Correction Loop Pattern

Agents should retry with automatic error correction, treating a failed `fmt` or compile exactly
like a failed validation (tested against D2 v0.7.1 with parse and compile errors):

```python
import subprocess
import tempfile
from pathlib import Path
from typing import Callable, Optional

MAX_RETRIES = 3

def run(args: list[str]) -> subprocess.CompletedProcess:
    return subprocess.run(args, capture_output=True, text=True)

def generate_and_check_d2(
    content: str,
    filepath: str,
    fix_callback: Callable[[str, list], str],
) -> Optional[str]:
    """
    Write a D2 file and repair it until it validates and compiles.

    Args:
        content: Initial D2 source code
        filepath: Path of the .d2 source in the repository
        fix_callback: Function(content, errors) -> fixed_content

    Returns:
        The final source, or None if all retries failed. Run
        `make render-diagrams` afterwards for the committed render.
    """
    current_content = content
    scratch = Path(tempfile.mkdtemp())  # outside the repository

    for _ in range(MAX_RETRIES):
        Path(filepath).write_text(current_content)

        result = run(['d2', 'fmt', filepath])  # fails on parse errors too
        if result.returncode == 0:
            result = run(['d2', 'validate', filepath])
        if result.returncode == 0:
            # validate is parse-only: compile to scratch to catch the rest
            result = run(['d2', filepath, str(scratch / 'out.svg')])
            if result.returncode == 0:
                return Path(filepath).read_text()

        errors = parse_d2_errors(result.stderr)
        if not errors:
            break  # Unknown error format

        current_content = fix_callback(Path(filepath).read_text(), errors)

    return None  # All retries exhausted
```

## Common Errors and Fixes

Messages observed with D2 v0.7.1. Rows marked "validate" are parse errors, which `d2 fmt` also
reports. "Compile" errors are reported only when the file is compiled, never by `d2 validate`.

| Error message | Reported by | Cause | Fix |
|---|---|---|---|
| `unexpected text after unquoted string` | validate | Unquoted label containing `\n[...]`, brackets or similar | Quote the whole label: `a -> b: "Visits\n[HTTPS]"` |
| `edge map keys must be reserved keywords` | validate | Unquoted edge label containing `{...}` (e.g. `200 OK\n{orderId}`) | Quote the label |
| `double quoted strings must be terminated with "` | validate | Missing closing quote | Close the string |
| `maps must be terminated with }` | validate | Missing closing brace | Close the block |
| `unknown shape "X"` | compile | Invalid shape name | Use a shape from the reference below |
| `direction must be one of up, down, right, left, got "X"` | compile | Invalid direction value | Use `up`, `down`, `right` or `left` |
| `invalid style keyword: "X"` | compile | Misspelled `style.` property (e.g. `style.fil`) | Fix the keyword |
| `invalid underscore: no parent` | compile | `_` used at the root level | Reference the object directly |
| `could not resolve variable "X"` | compile | `${X}` with no matching `vars` entry | Define the variable or remove the substitution |

Some mistakes produce no error at all, so inspect the render for unexpected extra shapes:

- A misspelled node name in an edge silently creates a new node.
- A misspelled keyword without the `style.` prefix (e.g. `styel: red` or `shap: oval`) silently
  creates a child shape labelled with the value.
- A duplicate key is not an error; the later value wins.

### Shape Reference

Valid shapes for error correction:

```
rectangle (default)  oval           diamond        cylinder
circle              cloud          hexagon        parallelogram
person              queue          package        page
document            step           callout        stored_data
```

### Direction Reference

Valid direction values:

```
up      down      right      left
```

## Determinism Configuration

Keep `d2-config` layout-only and pass seeds on the CLI; `tala-seeds` is not a config key. In
this repository `scripts/render-diagrams.sh` adds `--tala-seeds=100` when the installed TALA
supports it, and records the layout and seed in each SVG's stamp. Do not hand-roll render
commands for committed SVGs.

**Why seeds matter:**
- TALA runs every seed in its list in parallel and returns the best layout; without the flag it
  uses its default list (`[1,2,3]` in v0.7.1), so a different list can produce a different layout
- Different seed lists or TALA versions make version-control diffs noisy
- One fixed seed per project keeps renders reproducible

## CI/CD Integration

`.github/workflows/validate-diagrams.yml` is the CI contract. It installs D2 pinned to v0.7.1,
then for every `docs/diagrams/**/*.d2` runs `d2 fmt --check` and `d2 validate`, runs
`scripts/check-diagram-readability.sh`, and runs `scripts/check-diagram-renders.sh`, which checks
the committed SVG stamps and manifests without TALA. CI does not compile or render, so compile
errors are caught only by the local loop and `make render-diagrams`.

For this repository, do not add CI auto-commit steps for rendered diagrams.
Render locally with `make render-diagrams`, review source and SVG diffs together,
and let CI validate that committed renders are current.

Local equivalents:

```bash
make render-diagrams
make check-diagram-renders
make check-diagram-readability
```

## Watch Mode for Development

D2 includes a built-in watch mode:

```bash
# Watch single file
d2 --watch diagram.d2 diagram.svg

# Watch with browser preview
d2 --watch --browser diagram.d2
```

**Agent tip:** When iterating on diagram changes, use watch mode to see updates in real-time.
Point the output outside the repository, and stop watch mode before committing so the final
render comes from `make render-diagrams`.

## Debugging Tips

### Verbose Output

```bash
d2 --debug diagram.d2 output.svg   # also prints which layout plugin was used
```

### Check Layout Engine

```bash
d2 layout  # Lists available engines
d2 layout tala  # Engine-specific flags, e.g. --tala-seeds
```

## Error Recovery Strategies

### Strategy 1: Incremental Generation

Generate diagram in stages, validating after each:

```python
def generate_incrementally(nodes, connections):
    """Build diagram piece by piece, validating each addition."""
    content = base_config()

    # Add nodes first
    for node in nodes:
        content += generate_node(node)
        if not validate(content):
            content = rollback_last_addition(content)
            content += generate_node_fallback(node)

    # Then connections
    for conn in connections:
        content += generate_connection(conn)
        if not validate(content):
            # Connection references invalid node
            content = rollback_last_addition(content)

    return content
```

Here `validate` should mean "validates and compiles" (see the pipeline above).

### Strategy 2: Template-Based Generation

Use validated templates as starting points:

```d2
# template-c4-container.d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

# System boundary - customize this
system: System Name {
  style.stroke: "#1168BD"
  style.stroke-dash: 3

  # Add containers here
}

# External actors - customize this
user: User {
  shape: person
}

# Connections - customize this
user -> system
```

### No Engine Fallback for Committed Renders

Production renders require TALA. If TALA is unavailable, fix the environment; do not switch
committed renders to dagre or ELK. `--allow-elk` exists only for local preview.
