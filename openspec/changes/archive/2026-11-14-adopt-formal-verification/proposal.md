## Why

Invowk's most security- and correctness-critical logic is either **relational** (how a command scope is built from discovery, `invowkmod.cue` requires, and the lock file; the explicit-only dependency closure; lock-file identity) or a **protocol** (lock-to-content integrity across sync, vendor, and discover; the `serverbase` lifecycle; the SSH host-callback token lifecycle; the watch loop; atomic lock writes). Today these are covered by hand-enumerated example tests and mutation testing. Mutation testing measures the strength of existing tests, but nothing generates the module graphs, attacker actions, or interleavings that nobody thought to write. The project has zero property-based or fuzz tests.

Go has no verifier comparable to Kani for Rust, so the models cannot check the shipped code directly. The sibling Lushtext project measured Quint, TLA+/TLC, Apalache, and `stateright` on this machine (Lushtext `docs/next/formal-verification-quint-vs-tlaplus.md`, 2026-09-24). Lushtext kept TLA+ only for disposable sketches because hand re-expressed models drifted from its Rust code, and Kani removed that drift. Invowk has no Kani option, so this change adopts external models **and** binds them to the real Go code: generated test vectors, counterexample replay, rapid state machines, and trace validation. The binding is what the Lushtext evaluation found most valuable.

A first reading of the modelled code already produced concrete candidate findings (design D9), including a fresh-cache sync path that never compares fetched content against the locked hash, and a lifecycle race that can overwrite a terminal state.

## What Changes

- Add **Alloy 6** relational models for command-scope construction, the explicit-only dependency closure, and lock-file identity, using Alloy's native `expect` annotations.
- Add **TLA+ models checked by TLC** for lock-to-content integrity (with attacker actions), the `serverbase` lifecycle, the SSH host-callback token lifecycle, the watch loop, and atomic-write durability.
- Bind models to code in four ways:
  - **`pgregory.net/rapid`** property and state-machine tests that run in `make test`;
  - **golden vectors** exported from exhaustive small-scope Alloy instances and replayed against the real functions;
  - **counterexample replay** that turns each model counterexample into a Go regression test;
  - **trace validation**, where harnesses emit generated TLA+ trace modules from the real code and TLC checks them.
- Add a **fail-closed runner** with pinned, checksum-verified jars, declared verdicts, named-violation mutants, witness invariants, action-coverage checks, recorded state counts, and a calibration rule.
- Deliver in **three gated phases**: toolchain plus relational track; protocol models; then trace validation and CI promotion. Each phase is verifiable on its own.
- Record the rejected tools (Quint, Gobra, Dafny, Lean 4, Goose/Perennial, Gomela) and the reason for each.
- Make rapid runs deterministic under the mutation profiles.
- No user-facing CLI, schema, or runtime behaviour changes. Findings are recorded; fixing them is a separate maintainer decision.

## Capabilities

### New Capabilities

- `formal-verification-toolchain`: Pinned tool acquisition, layout, fail-closed verdicts, vacuity and coverage guards, calibration, model-to-code bindings, correspondence records, Make targets, the CI lane and its promotion rule, and the rejected-tool record.
- `relational-model-verification`: Alloy models and rapid bindings for command-scope construction, explicit-only dependency closure, and lock-file identity.
- `protocol-model-verification`: TLA+/TLC models, trace validation, and rapid bindings for lock-to-content integrity, the serverbase lifecycle, the SSH host-callback token lifecycle, the watch loop, and atomic-write durability, plus a rapid-only binding for container retry.

### Modified Capabilities

- `mutation-testing`: adds a requirement that property-based tests run with a fixed seed, no failure files, and a bounded shrink time under mutation profiles, so kill and escape results stay reproducible against the stable-ID baseline.

## Impact

- **New directories:** `formal/alloy/`, `formal/tla/`, `formal/README.md`, and `formal/manifest.toml`. Tools are cached under the already-ignored `bin/formal/`. Reports go to `artifacts/formal/`, and `**/testdata/rapid/` is ignored.
- **New scripts and targets:** `scripts/formal.py` (Python standard library) with `scripts/test_formal.py`, registered in `test-scripts`. New Make targets: `formal`, `formal-alloy`, `formal-tla`, `formal-traces`, `formal-golden`, and `formal-rapid-deep`.
- **Go code:** new `_test.go` files only, in these packages:
  - `pkg/invowkmod`
  - `pkg/fspath`
  - `internal/app/deps`
  - `internal/app/modulesync`
  - `internal/core/serverbase`
  - `internal/sshserver`
  - `internal/watch`
  - `internal/container`

  No production code changes. Trace harnesses are gated at runtime by an environment variable rather than a build tag, so lint, vet, and goplint keep compiling them.
- **Dependencies:**
  - `pgregory.net/rapid` v1.3.0, test-only, with `go.mod` as its source of truth.
  - Alloy 6.2.0 and `tla2tools.jar` 1.7.4, both SHA-256 verified.
  - Temurin JDK at an exact version through `actions/setup-java@v6`.
  - No TLA+ CommunityModules.
- **CI:** a new `.github/workflows/formal-verification.yml` that runs weekly, on dispatch, and on a path-filtered pull-request subset. It is not a required check until the promotion rule is met.
- **Governance:**
  - a new `.agents/skills/formal-verification/` with `agents/openai.yaml`;
  - index and code-area mapping rows in `.claude/CLAUDE.md`, which `AGENTS.md` symlinks to;
  - `version-pinning.md`, `commands.md`, and the `ci-update` sync-pair entries;
  - `.goplint/ownership.v1.json` documentation patterns for model files only;
  - `scripts/mutation.sh` rapid environment settings.
