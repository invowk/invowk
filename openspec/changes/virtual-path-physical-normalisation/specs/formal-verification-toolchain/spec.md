# Spec Delta

## MODIFIED Requirements

### Requirement: Defect fixes are separate from finding records
A product fix for a defect SHALL NOT be part of the change that records it as a finding. One exception applies: a change that the maintainer directed to fix a defect MAY record it and fix it together when an escape was observed on unmodified code and that observation is cited (a CI run and job id, or a probe with its commit). The finding SHALL then be recorded directly as fixed, with a pre-fix regression mutant, a passing fixed configuration, a replay that asserts the fixed behaviour, and a README Findings row that cites the evidence.

#### Scenario: Finding recorded without its fix
- **WHEN** a change records a defect as a finding, and the exception does not apply
- **THEN** that change SHALL NOT contain the product fix for the defect

#### Scenario: Finding recorded as fixed after an observed escape
- **WHEN** a maintainer-directed fix change records a defect whose escape was observed on unmodified code
- **THEN** the change SHALL cite that observation in the README Findings row, SHALL add a `mutantPreFix…` command with `expect = "counterexample"` and no `finding` key next to a passing fixed configuration, and SHALL add a replay that asserts the fixed behaviour

### Requirement: A rebinding that fails on unmodified code is a finding
When a rebound binding fails on unmodified code and no open finding explains the failure, the work SHALL stop and the failure SHALL be reported. The discrepancy SHALL be recorded under the next free finding id in the Findings table of `formal/README.md` through the finding procedure, and the failing binding SHALL NOT be committed. The assertion SHALL NOT be weakened, and product code SHALL NOT change.

#### Scenario: A rebinding exposes a defect
- **WHEN** a new real-function replay fails on unmodified code, and the failure is not explained by an open finding
- **THEN** the work SHALL stop and report the failure, with no weaker assertion and no product change
- **THEN** the discrepancy SHALL be recorded under the next free finding id, with a manifest `finding` command next to a passing fix configuration, a characterisation test, and a `formal/README.md` Findings row
