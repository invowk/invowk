# Invowkmod Samples

This directory contains sample `.invowkmod` trees for manual validation, demos,
and security-audit smoke checks.

## Samples

- `io.invowk.sample.invowkmod` is the safe reference module. It should pass
  normal module validation.
- `com.example.audit.deterministic.invowkmod` is intentionally malicious. It is
  designed to trigger the deterministic `invowk audit` checkers reachable from
  a loadable module: lock file, script, network, environment, module metadata,
  and correlator. Symlink risks are rejected by module loading before the
  `SymlinkChecker` can run against a real `.invowkmod` tree.
- `com.example.audit.llm.subtle.invowkmod` avoids the deterministic pattern
  matchers and demonstrates semantic risks that require `invowk audit
  --llm-provider codex` or another LLM provider.

## Useful Commands

Validate the safe reference module:

```bash
go run . validate samples/invowkmods/io.invowk.sample.invowkmod
```

Run the deterministic audit fixture:

```bash
go run . audit --format json --severity info samples/invowkmods/com.example.audit.deterministic.invowkmod
```

Run the LLM-only semantic fixture:

```bash
go run . audit --llm-provider codex --format json --severity info samples/invowkmods/com.example.audit.llm.subtle.invowkmod
```
