# Agent Checklist

Before considering work complete:

1. **Linting passes**: `make lint`.
2. **Tests pass**: `make test`.
3. **License headers**: `make license-check` (for new Go files).
4. **Dependencies tidy**: `make tidy`.
5. **Documentation updated**: Check sync map for affected docs.
6. **Website builds**: `cd website && npm run build` (if website changed).
7. **Sample modules valid**: `go run . module validate modules/*.invkmod --deep` (if module-related).
8. **CLI tests pass**: `make test-cli` (if CLI commands/output changed).
