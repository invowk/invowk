---
paths:
  - "**/*.go"
---

# Licensing

The project is licensed under MPL-2.0 (see [LICENSE](../../LICENSE)).

- Every `.go` file starts with exactly `// SPDX-License-Identifier: MPL-2.0` as its first line, followed by a blank line. Any `//go:build` constraint, package documentation and the `package` clause come after that blank line.
- Do NOT add a copyright year, copyright holder or license text; the SPDX line is the whole header.
- Verify with `make license-check` (also a CI job).

```go
// SPDX-License-Identifier: MPL-2.0

// Package config handles application configuration.
package config
```
