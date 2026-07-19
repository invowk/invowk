## ADDED Requirements

### Requirement: Every executable call and closure is conservatively modeled
Goplint SHALL model every relevant call expression in Go evaluation order and SHALL analyze every function literal body as an executable procedure independent of whether its invocation is syntactically visible in the enclosing function. A nested, sibling, returned, stored, passed, deferred, or concurrently launched closure or call MUST NOT be omitted from protocol transfer; incomplete ordering, identity, capture, or effect information MUST be blocking inconclusive.

#### Scenario: Nested mutation precedes the outer protected use
- **WHEN** a validated tracked value is passed by reference to a mutating inner call used as an argument of an outer protected call
- **THEN** the inner call's mutation or unresolved effect MUST transfer before the outer protected use
- **AND** the prior validation MUST be invalidated or the obligation MUST be blocking inconclusive

#### Scenario: Sibling calls each receive realizable call and return edges
- **WHEN** one source expression contains multiple relevant sibling calls
- **THEN** every call MUST receive its own ordered call-site identity, conservative transfer, and matching return continuation
- **AND** no sibling effect MAY be skipped because another call appears first in AST preorder

#### Scenario: Returned closure body is analyzed
- **WHEN** a function returns, stores, or passes a closure containing a protocol obligation
- **THEN** goplint MUST analyze the closure body as its own procedure and report its local violation or inconclusive outcome
- **AND** absence of an invocation visible in the enclosing function MUST NOT classify the closure body as non-executable or safe

#### Scenario: Unresolved closure capture fails closed
- **WHEN** a closure obligation depends on a captured identity or effect that cannot be resolved through the supported SSA semantics
- **THEN** the affected obligation MUST be blocking inconclusive with a stable reason
- **AND** the closure MUST NOT be silently skipped

### Requirement: Deferred constructor validation is proven on every successful return
Goplint SHALL establish constructor validation through a deferred closure only when canonical path analysis proves that every realizable successful return executes the matching validation, propagates that exact invocation's nil result to the constructor's returned error slot, and preserves the relation through all later deferred effects. Syntactic presence of validation and result assignments MUST NOT establish this proof.

#### Scenario: Conditional deferred validation does not discharge the constructor
- **WHEN** a deferred closure calls `Validate()` and assigns its result only under a condition that can be false on a successful constructor return
- **THEN** the constructor MUST remain unvalidated on that path
- **AND** goplint MUST report a violation or blocking inconclusive according to the supported path semantics

#### Scenario: Deferred result overwrite invalidates propagation
- **WHEN** a deferred closure assigns the validation result to the named error return and a later reachable effect can overwrite or disconnect that result
- **THEN** the validation MUST NOT discharge the successful-return obligation

#### Scenario: Deferred validation follows LIFO path semantics
- **WHEN** multiple deferred calls or closures affect the returned object or error slot
- **THEN** goplint MUST apply their summaries in Go's LIFO execution order on each constructor return
- **AND** unresolved relevant deferred effects MUST be blocking inconclusive

### Requirement: Protocol inconclusive outcomes are always visible
Every protocol inconclusive outcome SHALL be a hard-blocking diagnostic outside baseline and exception suppression. Baseline parsing, baseline updates, analyzer reporting, full scans, pre-commit, and CI MUST preserve the diagnostic even when stale suppression data names its category or stable finding ID.

#### Scenario: Baseline cannot suppress an inconclusive category
- **WHEN** a baseline contains an entry for a protocol inconclusive category or an inconclusive outcome
- **THEN** baseline validation MUST fail with an actionable error
- **AND** the analyzer MUST still emit the blocking inconclusive diagnostic

#### Scenario: Baseline update excludes inconclusives
- **WHEN** baseline update tooling observes protocol inconclusive outcomes
- **THEN** it MUST refuse to serialize them as accepted baseline entries
- **AND** the update MUST remain unsuccessful while those outcomes exist

#### Scenario: Blocking scans expose existing uncertainty
- **WHEN** the canonical repository scan encounters a previously baselined inconclusive finding
- **THEN** the scan MUST fail until stronger analysis classifies it or the underlying code removes the uncertainty
- **AND** migration to another suppression mechanism MUST NOT satisfy the gate
