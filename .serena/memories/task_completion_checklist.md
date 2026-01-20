# Task Completion Checklist

Before considering work complete, verify:

## 1. Tests Pass
```bash
make test
```

## 2. License Headers Present
```bash
make license-check
```
All new `.go` files must have `// SPDX-License-Identifier: EPL-2.0` as first line.

## 3. Dependencies Tidy
```bash
make tidy
```

## 4. Documentation Updated
Check if affected docs need updates:
- README.md (user-facing changes)
- website docs (CLI/API changes)
- CUE schema comments (schema changes)

## 5. Website Builds (if changed)
```bash
cd website && npm run build
```

## 6. Sample Modules Valid (if module-related)
```bash
go run . module validate modules/*.invkmod --deep
```

## 7. Stale References Cleaned
After refactors, check for:
- Stale tests
- Outdated CUE types
- Old docs/examples

## 8. Lint Passes
```bash
make lint
```
Ensure golangci-lint reports no issues.
