## ADDED Requirements

### Requirement: Command-scope construction model
Invowk SHALL maintain an Alloy model of how a command scope is built and queried, rather than of `CommandScope.CanCallTarget` in isolation. The model SHALL cover:
- discovered sources by kind: cwd invowkfile, local module, include, provisioned, user-dir global, and vendored child of a module;
- each module's self-declared `ModuleID`;
- `SourceID` derivation from the directory name, include alias, or lock namespace;
- the uniqueness facts enforced by `CheckModuleCollisions`;
- `IsGlobalModule` inheritance by vendored children;
- the caller's requires and lock entries through `IsDeclaredLockedCommandSource`;
- `buildCommandScope`;
- the full decision order of `CanCallTarget`: validation deny, local, same module (including both empty-source leniencies), global by `SourceID`, then direct dependency by `(ModuleID, SourceID)`.

#### Scenario: Allowed targets are bounded
- **WHEN** Alloy checks all discovery configurations within the declared scope
- **THEN** every allowed target SHALL belong to one of these sets:
  - the caller's own module;
  - a user-dir global module;
  - a vendored child of a user-dir global module;
  - a requirement that the caller declares and locks.

#### Scenario: Global children are an explicit exception
- **WHEN** a vendored child of a global module is callable from an unrelated module
- **THEN** the model SHALL admit it only through the declared global-children exception, and the correspondence record SHALL state that this is intended behaviour

#### Scenario: Collision facts are load-bearing
- **WHEN** the `CheckModuleCollisions` uniqueness fact is removed as a mutant
- **THEN** the "allowed targets are bounded" check SHALL produce a counterexample in which a spoofed `SourceID` is admitted as global

#### Scenario: Empty source identity is a load-bearing discovery fact
- **WHEN** the model drops discovery's guarantee that module-backed sources carry a `SourceID`
- **THEN** a mutant check SHALL produce a counterexample in which a foreign source sharing the caller's `ModuleID` is admitted, recording that the empty-source leniency in `targetIsSameModule` is safe only because discovery never produces it

#### Scenario: Root invowkfile commands
- **WHEN** a module targets a command from the cwd invowkfile, whose `SourceID` is `invowkfile` and whose `ModuleID` is empty
- **THEN** the model SHALL check that the call is denied, consistent with the scope policy that `commandScopeDecision` reports, and SHALL check that the local branch of `CanCallTarget` is unreachable from discovery inputs

### Requirement: Explicit-only dependency closure model
Invowk SHALL maintain an Alloy model of the explicit-only policy. Closure SHALL be keyed by `ModuleRefKey` (URL and path, without version). The one-step code check (`CheckMissingTransitiveDeps` over resolved modules) SHALL be modelled literally, and the specification SHALL be written with the transitive closure `^requires`, so the equivalence between them is not a tautology.

#### Scenario: Sync accepts only closed requirement sets
- **WHEN** Alloy checks root requirement sets and module graphs within bounds
- **THEN** the one-step diagnostic set SHALL be empty if and only if the roots contain `roots.^requires`

#### Scenario: Tidy result
- **WHEN** tidy's result is modelled statically
- **THEN** it SHALL equal `roots.^requires − roots` as a key set

#### Scenario: Tidy call bound
- **WHEN** rapid drives `tidyToFixedPoint` with a generated `resolveAllFunc`
- **THEN** the test SHALL check that the number of `resolveAll` calls equals the number of breadth-first layers plus one confirming round, and that the result equals the closure minus the roots

#### Scenario: Unreachable branch is declared
- **WHEN** the model's coverage shows that the no-new-key return inside the tidy loop is unreachable
- **THEN** the manifest SHALL declare it dead with that reason

#### Scenario: Version is abstracted
- **WHEN** two requirements differ only by version
- **THEN** the correspondence record SHALL state that the closure treats them as the same key

### Requirement: Lock-file identity model
Invowk SHALL maintain an Alloy model of lock-file identity. An entry's identity SHALL be its `module_id`, or else its namespace prefix before `@`. The model SHALL check the following:
- every identity with two or more entries is reported by `FindAmbiguousLockedModuleEntries`, even when the hashes are equal;
- `EvaluateVendoredModuleHash` returns `ambiguous` for such an identity;
- entries that differ by key but share a `ModuleID` are always caught by ambiguity detection or by the canonical-collision check.

#### Scenario: Ambiguity is always reported
- **WHEN** two lock entries share an identity
- **THEN** that identity SHALL appear in the ambiguity result, and vendored evaluation SHALL return `ambiguous`

#### Scenario: v2 entries never skip verification
- **WHEN** the model restricts to lock version 2.0 under the parser's requirement that entries carry `module_id` and `content_hash`
- **THEN** no identity SHALL be claimed by exactly one entry without a hash, and a mutant without the parser fact SHALL produce a counterexample
- **THEN** a witness SHALL show that a version 1.0 lock can reach that state, which is the mechanism of finding F4

#### Scenario: Namespace fallback collision
- **WHEN** a version 1.0 entry's namespace prefix equals another entry's `module_id`
- **THEN** the model SHALL record whether the two are reported as ambiguous, and the correspondence record SHALL classify the result

#### Scenario: Missing or empty hash
- **WHEN** exactly one entry claims an identity and it has no content hash
- **THEN** the golden replay SHALL require `EvaluateVendoredModuleHash` to return `unavailable` for that identity

### Requirement: Relational bindings to code
Each relational model SHALL be bound to the code in two ways:
- **Golden vectors:** exhaustive small-scope instances replayed against the real functions. For the scope model, the functions are reached through `CheckCommandDependenciesExistWithLockProvider`, using fake `CommandSetProvider` and `CommandScopeLockProvider`.
- **rapid property tests:** these use generators restricted to inputs that discovery and the parser actually produce.

Tests SHALL call `t.Parallel()` first. rapid tests SHALL compare key sets rather than order-dependent lists.

#### Scenario: Golden vectors pass
- **WHEN** `go test` runs the golden-vector tests
- **THEN** the real result SHALL equal the model's predicate value for every exported instance

#### Scenario: rapid disagreement
- **WHEN** a rapid property fails
- **THEN** rapid SHALL report a shrunk counterexample, and the failure message SHALL name the model assertion
