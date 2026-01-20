# Suggested Commands

## Build Commands
```bash
make build          # Build stripped binary (default)
make build-dev      # Build with debug symbols (faster, for dev)
make build-upx      # Build UPX-compressed binary (smallest)
make build-all      # Build both stripped and UPX variants
make build-cross    # Cross-compile for Linux, macOS, Windows
make clean          # Remove build artifacts
make install        # Install to GOPATH/bin
```

## Test Commands
```bash
make test              # Run all tests
make test-short        # Run tests in short mode (skip integration)
make test-integration  # Run integration tests only
go test -v ./...       # Run all tests with verbose output
go test -v ./pkg/invkfile/...  # Run specific package tests
```

## Dependency Management
```bash
make tidy           # Tidy go.mod dependencies
```

## Quality Checks
```bash
make license-check  # Verify SPDX headers in all Go files
make lint           # Run golangci-lint on all packages
make size           # Compare binary sizes (debug vs stripped vs UPX)
```

## Module Validation
```bash
go run . module validate modules/*.invkmod --deep  # Validate sample modules
go run . module validate modules/<module-name>.invkmod --deep  # Validate specific module
```

## Website (if modified)
```bash
cd website && npm run build  # Build website
```

## Git Utilities
```bash
git status          # Show working tree status
git log --oneline   # Show commit history
git diff            # Show unstaged changes
git add -p          # Interactive staging
```

## System Utilities (Linux)
```bash
ls -la              # List files with details
find . -name "*.go" # Find Go files
grep -r "pattern" . # Search in files
```
