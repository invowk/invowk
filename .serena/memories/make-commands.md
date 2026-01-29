# Invowk Make Commands

## Build & Run
- `make build` - Build the binary
- `make run` - Build and run

## Testing
- `make test` - Run all tests
- `make test-cli` - Run CLI integration tests (testscript)
- `make test-race` - Run tests with race detector

## Code Quality
- `make lint` - Run golangci-lint
- `make license-check` - Verify SPDX headers
- `make tidy` - Run go mod tidy

## Demos
- `make vhs-demos` - Generate demo GIFs
- `make vhs-validate` - Validate VHS tape syntax

## Module Validation
```bash
go run . module validate modules/*.invkmod --deep
```

## Pre-Commit Checklist
1. `make lint`
2. `make test`
3. `make license-check`
4. `make tidy`
5. Update docs if needed
6. `make test-cli` (if CLI changed)
