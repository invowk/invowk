## ADDED Requirements

### Requirement: Successful constructor returns follow exact control-flow evidence
Goplint SHALL retain a constructor return obligation unless the exact control-flow edge and returned error identity prove that return unsuccessful. Branch ancestry, condition text, type equality, a different error value, or an empty extracted target set MUST NOT substitute for edge-sensitive result evidence.

#### Scenario: Else branch is a successful return
- **WHEN** an unvalidated object is returned from the `else` branch of `if err != nil` together with that branch's nil `err`
- **THEN** goplint MUST retain the returned-object obligation and report a violation or blocking inconclusive
- **AND** the ancestor non-nil condition MUST NOT classify the `else` return as unsuccessful

#### Scenario: Inverted and nested result checks preserve polarity
- **WHEN** successful and unsuccessful constructor returns are separated by inverted, nested, switch-derived, or phi-joined error conditions
- **THEN** each return MUST be classified from its realizable edge and exact returned error value
- **AND** uncertainty about the relation MUST be blocking inconclusive rather than exclusion of the return

#### Scenario: Empty return target set requires proof
- **WHEN** constructor identity extraction produces no returned-object target
- **THEN** goplint MAY classify the constructor safe only after proving that no realizable successful return contains a non-nil object
- **AND** discarded, ambiguous, or unresolved return identity MUST produce blocking inconclusive

### Requirement: Protocol summaries preserve conditional effect relations
Goplint SHALL preserve each summary effect's target identity, source identity, condition, condition-result slot, and execution order through export, import, recursion, composition, and caller transfer. A conditional validation effect MUST apply only on a caller continuation that proves the matching result condition.

#### Scenario: Discarded helper error does not restore validation
- **WHEN** a helper mutates a validated object, conditionally validates it, and the caller discards or overwrites the helper's returned error
- **THEN** the caller's prior validation MUST remain invalidated or blocking inconclusive
- **AND** summary application MUST NOT synthesize a nil result for the conditional validation effect

#### Scenario: Checked helper success restores validation
- **WHEN** the caller proves the exact helper error result nil after an ordered mutation-then-validation summary
- **THEN** the matching object MAY become validated on that nil continuation
- **AND** non-nil, unknown, mismatched, or unrelated error continuations MUST remain unvalidated or inconclusive

#### Scenario: Summary order remains observable
- **WHEN** two summaries contain validation-before-mutation and mutation-before-validation effects respectively
- **THEN** goplint MUST produce the distinct typestate required by each ordered sequence
- **AND** normalization MUST NOT sort, merge, or collapse away the condition or order

### Requirement: Protocol routing covers every package procedure root
Goplint SHALL discover and analyze every function declaration and function literal body in an analyzed package, including literals in package initializers, independent of whether an invocation is syntactically visible. Relevant missing procedure or SSA identity MUST be blocking inconclusive.

#### Scenario: Package-level stored closure is analyzed
- **WHEN** a package variable initializer stores a function literal containing a cast, validation, protected use, constructor obligation, or escape
- **THEN** goplint MUST analyze that literal body and emit its required violation or inconclusive outcome
- **AND** routing through a `GenDecl` MUST NOT silently omit the literal because it is not a `FuncDecl`

#### Scenario: Nested closure is reported once
- **WHEN** a function literal is reachable both from the package procedure inventory and from an enclosing procedure's closure discovery
- **THEN** stable procedure identity MUST cause one semantic analysis and one diagnostic result per obligation
- **AND** discovery order MUST NOT duplicate or suppress the finding

#### Scenario: Unresolved package literal fails closed
- **WHEN** a relevant package-level literal cannot be associated with a unique SSA procedure or initializer context
- **THEN** the affected obligation MUST emit a stable blocking inconclusive outcome
- **AND** the body MUST NOT be treated as non-executable or safe

### Requirement: Protocol uncertainty is classified before suppression
Every protocol entry point SHALL classify the semantic outcome before consulting exception, inline-ignore, or baseline policy. Inconclusive outcomes MUST use the always-visible reporting path regardless of whether the obligation occurs in a function declaration, nested closure, escaping closure, package-level literal, constructor, or boundary request.

#### Scenario: Excepted closure still reports inconclusive
- **WHEN** a closure matches an exception or inline-ignore directive and its protocol analysis is inconclusive
- **THEN** goplint MUST emit the inconclusive diagnostic
- **AND** the policy match MAY affect only an otherwise suppressible definite finding

#### Scenario: Every protocol route has the same ordering
- **WHEN** architecture validation enumerates protocol analysis and reporting entry points
- **THEN** each entry point MUST classify uncertainty before any policy suppression branch
- **AND** a route that can return before inconclusive classification MUST fail the architecture gate

### Requirement: Post-validation escape and fact uncertainty remain conservative
Goplint SHALL invalidate or make blocking inconclusive every relevant post-validation mutation, replacement, mutable escape, indirect store, or incompatible imported fact that can affect the tracked identity. Value copies MAY remain safe only when the copied form cannot mutate the validated identity.

#### Scenario: Mutable channel or aggregate escape after validation is blocking
- **WHEN** a validated object's address or mutable alias is sent on a channel or stored into an aggregate before a protected use or successful return
- **THEN** goplint MUST invalidate the validation or report escaped-heap or concurrent-mutation inconclusive
- **AND** post-validation state MUST NOT absorb the escape as an identity effect

#### Scenario: Immutable value copy remains precise
- **WHEN** a validated non-pointer value is copied into a channel, aggregate, or callee slot and no alias to the original identity escapes
- **THEN** goplint MUST preserve the original object's validated state
- **AND** the mutable-escape rule MUST NOT create unrelated uncertainty

#### Scenario: Imported fact slots match the attached signature
- **WHEN** an imported protocol fact names a function, target slot, source slot, or condition-result slot absent from or incompatible with the attached function signature
- **THEN** fact validation MUST reject it as incompatible
- **AND** every relevant dependent obligation MUST be blocking inconclusive rather than silently skipping the impossible effect

#### Scenario: Imported fact identity is exact
- **WHEN** a fact's package path, function identity, format, condition vocabulary, or slot role differs from the object to which it is attached
- **THEN** goplint MUST reject the fact before summary application
- **AND** no partial compatible subset MAY be treated as a complete resolved summary
