# Agent Workflow

This reference covers the validation and rendering pipeline optimized for AI agent generation, including error parsing, self-correction loops, and CI/CD integration.

## Validation Pipeline

The recommended three-stage pipeline ensures errors are caught early:

```bash
#!/bin/bash
set -euo pipefail

D2_FILE="$1"
OUTPUT="${D2_FILE%.d2}.svg"

# Stage 1: Format (canonicalize)
d2 fmt "$D2_FILE"

# Stage 2: Validate (no rendering, fast)
if ! d2 validate "$D2_FILE" 2>&1; then
    echo "Validation failed" >&2
    exit 1
fi

# Stage 3: Render (only after validation passes)
d2 --layout=tala --tala-seeds=100 "$D2_FILE" "$OUTPUT"
```

**Why three stages:**
1. **Format** - Normalizes whitespace, ordering, and syntax for deterministic diffs
2. **Validate** - Fast check without GPU/rendering overhead
3. **Render** - Only invoked when diagram is known-valid

## Error Output Format

D2 produces machine-parseable error messages:

```
filename.d2:LINE:COLUMN: error: MESSAGE
```

**Examples:**

```
diagram.d2:5:1: error: connection must reference valid shape: "undefined_node"
diagram.d2:12:15: error: invalid keyword: "styel" (did you mean "style"?)
diagram.d2:8:3: error: "direction" can only be one of the following values: "up", "down", "right", "left"
diagram.d2:15:1: error: shape "invalid_shape" not recognized
```

### Parsing Errors Programmatically

```python
import re
from dataclasses import dataclass

@dataclass
class D2Error:
    file: str
    line: int
    column: int
    message: str

def parse_d2_errors(stderr: str) -> list[D2Error]:
    """Parse D2 error output into structured objects."""
    pattern = r'^(.+):(\d+):(\d+): error: (.+)$'
    errors = []
    for line in stderr.strip().split('\n'):
        if match := re.match(pattern, line):
            errors.append(D2Error(
                file=match.group(1),
                line=int(match.group(2)),
                column=int(match.group(3)),
                message=match.group(4)
            ))
    return errors
```

## Self-Correction Loop Pattern

Agents should implement a retry loop for automatic error correction:

```python
import subprocess
from typing import Optional

MAX_RETRIES = 3

def generate_and_validate_d2(
    content: str,
    filepath: str,
    fix_callback: callable
) -> Optional[str]:
    """
    Generate D2 diagram with automatic error correction.

    Args:
        content: Initial D2 source code
        filepath: Path to write the .d2 file
        fix_callback: Function(content, errors) -> fixed_content

    Returns:
        Path to rendered SVG, or None if all retries failed
    """
    current_content = content

    for attempt in range(MAX_RETRIES):
        # Write current content
        with open(filepath, 'w') as f:
            f.write(current_content)

        # Format
        subprocess.run(['d2', 'fmt', filepath], check=True)

        # Validate
        result = subprocess.run(
            ['d2', 'validate', filepath],
            capture_output=True,
            text=True
        )

        if result.returncode == 0:
            # Validation passed, render
            output_path = filepath.replace('.d2', '.svg')
            subprocess.run(
                ['d2', '--layout=tala', '--tala-seeds=100', filepath, output_path],
                check=True
            )
            return output_path

        # Parse errors and attempt fix
        errors = parse_d2_errors(result.stderr)
        if not errors:
            break  # Unknown error format

        # Let agent fix the errors
        current_content = fix_callback(current_content, errors)

    return None  # All retries exhausted
```

## Common Errors and Fixes

| Error Pattern | Cause | Fix |
|---------------|-------|-----|
| `connection must reference valid shape` | Typo in node name | Check spelling, ensure node is defined |
| `invalid keyword` | Misspelled property | Check for typos (e.g., `styel` → `style`) |
| `shape not recognized` | Invalid shape name | Use valid shape: `rectangle`, `oval`, `diamond`, etc. |
| `direction can only be one of` | Invalid direction value | Use: `up`, `down`, `right`, `left` |
| `duplicate key` | Same key defined twice | Remove duplicate or rename |
| `expected newline` | Missing newline after block | Add newline before next statement |

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

For reproducible output in CI/CD:

```bash
# Keep d2-config layout-only; pass seeds through CLI
d2 --layout=tala --tala-seeds=100 diagram.d2 diagram.svg
```

**Why seeds matter:**
- Without seeds, TALA uses random initialization
- Same diagram may render with slight layout differences
- Version control diffs become noisy
- CI/CD may fail on visual regression tests

**Seed selection:**
- Any positive integer works
- Use consistent seed per project
- Document seed in team conventions

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Render D2 Diagrams

on:
  push:
    paths:
      - 'docs/diagrams/**/*.d2'
  pull_request:
    paths:
      - 'docs/diagrams/**/*.d2'

jobs:
  render:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install D2
        run: |
          curl -fsSL https://d2lang.com/install.sh | sh -s -- --dry-run
          curl -fsSL https://d2lang.com/install.sh | sh -s --

      - name: Install TALA (optional, requires license)
        if: ${{ secrets.TALA_LICENSE }}
        env:
          TALA_LICENSE: ${{ secrets.TALA_LICENSE }}
        run: |
          # TALA installation steps
          echo "TALA license configured"

      - name: Validate diagrams
        run: |
          for file in docs/diagrams/**/*.d2; do
            echo "Validating $file"
            d2 fmt "$file"
            d2 validate "$file"
          done

      - name: Render diagrams
        run: |
          for file in docs/diagrams/**/*.d2; do
            output="${file%.d2}.svg"
            echo "Rendering $file -> $output"
            d2 --layout=tala --tala-seeds=100 "$file" "$output"
          done

      - name: Commit rendered diagrams
        if: github.event_name == 'push'
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add docs/diagrams/**/*.svg
          git diff --staged --quiet || git commit -m "chore: render D2 diagrams"
          git push
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Find modified D2 files
d2_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.d2$')

if [ -n "$d2_files" ]; then
    echo "Validating D2 diagrams..."

    for file in $d2_files; do
        # Format
        d2 fmt "$file"
        git add "$file"

        # Validate
        if ! d2 validate "$file"; then
            echo "Error: $file failed validation"
            exit 1
        fi
    done

    echo "All D2 diagrams valid"
fi
```

## Batch Processing

For projects with many diagrams:

```bash
#!/bin/bash
# render-all.sh

set -euo pipefail

DIAGRAMS_DIR="${1:-docs/diagrams}"
LAYOUT="${2:-tala}"
PARALLEL="${3:-4}"

find "$DIAGRAMS_DIR" -name '*.d2' -print0 | \
    xargs -0 -P "$PARALLEL" -I {} sh -c '
        file="$1"
        output="${file%.d2}.svg"
        echo "Rendering: $file"
        d2 fmt "$file"
        d2 validate "$file" || exit 1
        d2 --layout='"$LAYOUT"' "$file" "$output"
    ' _ {}

echo "All diagrams rendered successfully"
```

## Watch Mode for Development

D2 includes a built-in watch mode:

```bash
# Watch single file
d2 --watch diagram.d2 diagram.svg

# Watch with browser preview
d2 --watch --browser diagram.d2
```

**Agent tip:** When iterating on diagram changes, use watch mode to see updates in real-time. Stop watch mode before committing to ensure final render is deterministic.

## Debugging Tips

### Verbose Output

```bash
d2 --debug diagram.d2 output.svg
```

### Check Layout Engine

```bash
d2 layout  # Lists available engines
```

### Inspect Intermediate Representation

```bash
d2 --dry-run diagram.d2  # Shows parsed AST
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

### Strategy 3: Fallback to Simpler Layout

If TALA fails, fall back to dagre:

```python
def render_with_fallback(filepath):
    """Try TALA first, fall back to dagre on failure."""
    try:
        subprocess.run(
            ['d2', '--layout=tala', '--tala-seeds=100', filepath, output],
            check=True,
            timeout=30
        )
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
        # Fall back to dagre
        subprocess.run(
            ['d2', '--layout=dagre', filepath, output],
            check=True
        )
```
