# command-scope-content-admission Specification

## Purpose
Defines content-hash admission for direct dependency commands declared in `depends_on.cmds` (`IsDeclaredLockedCommandSource`). Complements `hashless-lock-entries` and F12 in `protocol-trace-validation`, which additionally reject v1.0 hashless caller lock entries.

## Requirements

### Requirement: Admission compares the caller's locked content
A command from a direct dependency SHALL be admitted into a module's command scope only if the caller's lock entry matches its module identity and command namespace and, when that entry records a content hash, the directory the command was discovered in hashes to it.

#### Scenario: Sibling's copy of another version
- **WHEN** the dependency's commands were discovered in a copy whose content hash differs from the caller's lock entry
- **THEN** `depends_on.cmds` validation SHALL report the command as forbidden

#### Scenario: Copy of the locked content
- **WHEN** the discovered copy hashes to the caller's locked content hash
- **THEN** the command SHALL be admitted

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `LockIntegrity.f6CheckCallerHash` SHALL pass, and `LockIntegrity.mutantPreFixF6AdmitByIdentity` and `mutantCompareWithSiblingLock` SHALL produce counterexamples
