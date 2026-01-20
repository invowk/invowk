# Bash Scripting

## Strict Mode (`set -euo pipefail`)

All bash scripts in this project use strict mode for safety:

```bash
set -euo pipefail
```

This enables:
- `-e` (errexit): Exit on any command failure
- `-u` (nounset): Error on undefined variables
- `-o pipefail`: Propagate errors through pipes

### Arithmetic Increment Gotcha

**CRITICAL: Never use `((var++))` with `set -e` when `var` might be 0.**

In bash, arithmetic expressions return exit status based on the expression's value:
- `((0))` returns exit status 1 (false)
- `((1))` returns exit status 0 (true)

The post-increment `((x++))` evaluates to the *original* value of `x`:

```bash
# DANGEROUS with set -e:
COUNTER=0
((COUNTER++))  # Evaluates to 0 (the original value), exits with status 1!
               # Script terminates here due to set -e
```

### Safe Arithmetic Patterns

**Use assignment syntax instead of increment operators:**

```bash
# CORRECT: Assignment always succeeds
COUNTER=$((COUNTER + 1))

# CORRECT: Alternative with let and || true guard
let COUNTER++ || true

# CORRECT: Using arithmetic expansion in assignment
: $((COUNTER++))  # The : (colon) command always succeeds
```

### Anti-Patterns to Avoid

```bash
# WRONG: Will fail if COUNTER is 0
((COUNTER++))

# WRONG: Will fail if FAILED is 0
((FAILED++))

# WRONG: Will fail if SKIPPED is 0
((SKIPPED++))
```

### Real-World Example

The VHS test scripts use counters for PASSED, FAILED, and SKIPPED tests:

```bash
# WRONG - Script exits on first skip when SKIPPED is 0:
SKIPPED=0
if [[ ! -f "$golden_file" ]]; then
    ((SKIPPED++))  # Exit status 1, script terminates!
    continue
fi

# CORRECT - Assignment syntax always works:
SKIPPED=0
if [[ ! -f "$golden_file" ]]; then
    SKIPPED=$((SKIPPED + 1))  # Always succeeds
    continue
fi
```

## Common Pitfall

- **Arithmetic with `set -e`** - Always use `VAR=$((VAR + 1))` instead of `((VAR++))` when `VAR` might be 0.
