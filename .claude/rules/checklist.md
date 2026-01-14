# Agent Checklist

Before considering work complete:

1. **Tests pass**: `make test`.
2. **License headers**: `make license-check` (for new Go files).
3. **Dependencies tidy**: `make tidy`.
4. **Documentation updated**: Check sync map for affected docs.
5. **Website builds**: `cd website && npm run build` (if website changed).
6. **Sample packs valid**: `go run . pack validate packs/*.invkpack --deep` (if pack-related).
