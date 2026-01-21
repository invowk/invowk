# Agent Checklist

Before considering work complete:

1. **Tests pass**: `make test`.
2. **License headers**: `make license-check` (for new Go files).
3. **Dependencies tidy**: `make tidy`.
4. **Documentation updated**: Check sync map for affected docs.
5. **Website builds**: `cd website && npm run build` (if website changed).
6. **Sample modules valid**: `go run . module validate modules/*.invkmod --deep` (if module-related).
7. **CLI tests pass**: `make test-cli` (if CLI commands/output changed).
