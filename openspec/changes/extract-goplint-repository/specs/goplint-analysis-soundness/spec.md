## REMOVED Requirements

The entire `goplint-analysis-soundness` capability migrates to the `invowk/goplint` repository, where it is maintained as docs-guard-governed documentation validated against that repository's Makefile, gates, and tree. Invowk consumes these guarantees through the exact pinned goplint version; no requirement is weakened — each is enforced by the goplint repository's own CI (semantic and complete profiles) before any version invowk can pin is released.

### Requirement: Goplint has one explicit soundness contract
**Reason**: Analyzer contract; authoritative copy migrates to `invowk/goplint`.

### Requirement: Validation requires a proven successful result
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Object identity and alias effects are SSA-sensitive
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Canonical interprocedural analysis follows realizable paths
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Generic and method-set forms preserve protocol obligations
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Feasibility and refinement are SSA-versioned and fail closed
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Result vocabulary reflects actual evidence
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Only the canonical analysis pipeline is available
**Reason**: Analyzer architecture; migrates to `invowk/goplint`.

### Requirement: Independent oracles cover the analyzer contract
**Reason**: Goplint self-verification; migrates to `invowk/goplint`.

### Requirement: Soundness gates are deterministic and performance bounded
**Reason**: Goplint self-verification; migrates to `invowk/goplint`.

### Requirement: Every executable call and closure is conservatively modeled
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Deferred constructor validation is proven on every successful return
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Protocol inconclusive outcomes are always visible
**Reason**: Analyzer semantics; migrates to `invowk/goplint`. Invowk's consumer gates continue to surface inconclusive outcomes as blocking through the pinned analyzer's behavior.

### Requirement: Successful constructor returns follow exact control-flow evidence
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Protocol summaries preserve conditional effect relations
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Protocol routing covers every package procedure root
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Protocol uncertainty is classified before suppression
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Post-validation escape and fact uncertainty remain conservative
**Reason**: Analyzer semantics; migrates to `invowk/goplint`.

### Requirement: Blocking mutation kernel spans every semantic category
**Reason**: Goplint self-verification; migrates to `invowk/goplint`.

### Requirement: Completion-proof evidence is regeneratable and documented
**Reason**: Completion-evidence machinery migrates to `invowk/goplint` with the v5 record defined over the goplint tree.
