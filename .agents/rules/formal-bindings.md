---
paths:
  - "pkg/invowkmod/**"
  - "pkg/invowkfile/**"
  - "pkg/fspath/**"
  - "internal/app/deps/**"
  - "internal/app/modulecache/**"
  - "internal/app/moduleops/**"
  - "internal/app/modulesync/**"
  - "internal/core/serverbase/**"
  - "internal/discovery/**"
  - "internal/provision/**"
  - "internal/runtime/**"
  - "internal/sshserver/**"
  - "internal/watch/**"
---

# Formal Bindings

Code in these directories is named by the correspondence tables of the formal models under `formal/` (`rg -o '(internal|pkg|cmd)/[^ ]+\.go' formal`). `formal/manifest.toml` `[ci] paths` is a superset that also covers test helpers and other files the models replay. `make test` does not check those tables, so a rename or behavior change breaks the model-to-code correspondence and is caught only by the required `Models, golden vectors, and correspondence` check on the PR (or locally by the commands below).

- Load `.agents/skills/formal-verification/` before renaming, moving or changing the behavior of a function a table names (`rg -n '<Symbol>' formal/` tells you).
- After such a change, update the tables and golden vectors as the skill describes, then run `python3 scripts/formal.py correspondence` (no Java needed) and `make formal` when Java is available. Changes under `formal/` also need `make test-scripts` (formal-bindings triage check).
- When a new directory becomes bound, add it here and, if missing, to `[ci] paths` in the same change.
