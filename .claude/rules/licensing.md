---
paths:
  - "**/*.go"
---

# Licensing

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0). See the [LICENSE](../../LICENSE) file for the full license text.

## SPDX License Headers

**All Go source files MUST include an SPDX license header** as the very first line(s) of the file, before any package documentation or code. This ensures clear and machine-readable licensing information.

### Required Header Format

Every `.go` file must start with this exact header:

```go
// SPDX-License-Identifier: MPL-2.0
```

### Placement Rules

1. The SPDX header MUST be the **first line** of every Go source file.
2. A blank line MUST follow the SPDX header.
3. Package documentation comments (if any) come after the blank line.
4. The `package` declaration follows the documentation.

### Complete Example

```go
// SPDX-License-Identifier: MPL-2.0

// Package config handles application configuration using Viper with CUE.
package config

import (
    "fmt"
)
```

### Example Without Package Documentation

```go
// SPDX-License-Identifier: MPL-2.0

package main

func main() {
    // ...
}
```

### Verification

Run the license header check to verify all source files have proper headers:

```bash
make license-check
```

### Adding Headers to New Files

When creating new Go source files, always include the SPDX header. The header format is intentionally minimal to reduce boilerplate while maintaining legal clarity.

**DO NOT** include:
- Copyright year (changes over time, creates maintenance burden).
- Copyright holder name (tracked in LICENSE file and git history).
- Full license text (referenced via SPDX identifier).

**DO** include:
- Only the SPDX-License-Identifier line with `MPL-2.0`.

## Common Pitfall

- **Forgetting SPDX headers** - Every new `.go` file needs `// SPDX-License-Identifier: MPL-2.0` as the first line.
