## REMOVED Requirements

The entire `goplint-soundness-assurance` capability migrates to the `invowk/goplint` repository, where it is maintained as docs-guard-governed documentation validated against that repository's Makefile, gates, and tree. No requirement is weakened — each is enforced by the goplint repository's own CI (semantic and complete profiles, ownership manifest v3, clean-tree evidence v5) before any version invowk can pin is released.

### Requirement: Production propagation implements the declared protocol domain
**Reason**: Analyzer-internal assurance; migrates to `invowk/goplint`.

### Requirement: Object identity is flow-sensitive and fail closed
**Reason**: Analyzer-internal assurance; migrates to `invowk/goplint`.

### Requirement: Unknown effects can invalidate an established property
**Reason**: Analyzer-internal assurance; migrates to `invowk/goplint`.

### Requirement: Refinement evidence remains paired with its exact state and path
**Reason**: Analyzer-internal assurance; migrates to `invowk/goplint`.

### Requirement: Interprocedural analysis converges through finite summaries
**Reason**: Analyzer-internal assurance; migrates to `invowk/goplint`.

### Requirement: Generic and unsupported forms preserve obligations conservatively
**Reason**: Analyzer-internal assurance; migrates to `invowk/goplint`.

### Requirement: One production semantic authority remains
**Reason**: Analyzer architecture; migrates to `invowk/goplint`.

### Requirement: Soundness evidence is complete, independent, and causal
**Reason**: Goplint self-verification; migrates to `invowk/goplint`.

### Requirement: Completion evidence is reproducible and claim accurate
**Reason**: Completion-evidence machinery; migrates to `invowk/goplint` with the v5 record.

### Requirement: Soundness execution plans are immutable and exhaustive
**Reason**: Harness contract; migrates with the distributed executor to `invowk/goplint`.

### Requirement: Concurrent and distributed evidence preserves causal bindings
**Reason**: Harness contract; migrates with the distributed executor to `invowk/goplint`.

### Requirement: Optimized profiles preserve their declared semantic populations
**Reason**: Harness contract; migrates to `invowk/goplint`.

### Requirement: Completion record identity separates semantic content from prose
**Reason**: Completion-record format; the v5 successor in `invowk/goplint` retains the dual-digest design over the goplint tree.

### Requirement: Harness-tier assurance proves orchestration integrity
**Reason**: Harness tier; migrates to `invowk/goplint` (invowk's routing reduces to documentation/consumer classes).

### Requirement: Task-ledger completion expectations have one reviewed source
**Reason**: Completion-plan governance; migrates to `invowk/goplint` with the v5 plan.
