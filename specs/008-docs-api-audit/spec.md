# Feature Specification: Documentation/API Audit

**Feature Branch**: `008-docs-api-audit`  
**Created**: 2026-02-02  
**Status**: Draft  
**Input**: User description: "Perform an extensive and extremely careful analysis between invowk's documentation (README, website docs etc.) and the user facing APIs/feature-set to identify possible gaps, invalid examples, and other kinds of mismatches."

## Clarifications

### Session 2026-02-02

- Q: Which severity scale should the audit use for findings? -> A: Critical / High / Medium / Low
- Q: Should the in-repo examples/ and modules/ sample directories be treated as documentation sources? -> A: Include both, and define a single centralized examples/samples folder as a documented rule.
- Q: Should the audit treat public Go packages under pkg/ as user-facing APIs to include in the surface inventory? -> A: Exclude pkg/ APIs; limit scope to CLI/config/modules/docs.
- Q: What should the severity mapping be based on? -> A: User impact of the mismatch.
- Q: What should be the required output format of the audit report? -> A: A single Markdown report file.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Coverage & Mismatch Inventory (Priority: P1)

As a maintainer, I want a complete inventory of user-facing features mapped to their documentation so gaps and mismatches are visible and can be fixed quickly.

**Why this priority**: This establishes a baseline of documentation accuracy and directly reduces user confusion.

**Independent Test**: A reviewer can verify that every in-scope user-facing surface is either linked to a specific doc location or flagged as undocumented.

**Acceptance Scenarios**:

1. **Given** a repository version with a defined set of user-facing commands, flags, and configuration fields, **When** the audit is completed, **Then** the report lists each surface with a documentation reference or an "undocumented" status.
2. **Given** documentation that mentions a command or flag not present in the current feature set, **When** the audit is completed, **Then** the report flags it as docs-only and includes the exact source location.

---

### User Story 2 - Example Validation Summary (Priority: P2)

As a contributor, I want documentation examples validated against the current feature set so invalid or misleading examples are identified and can be corrected.

**Why this priority**: Examples are the fastest path for users to adopt the tool; invalid examples cause immediate failures.

**Independent Test**: A reviewer can inspect the report and confirm that every example is marked valid or invalid with a concrete reason.

**Acceptance Scenarios**:

1. **Given** documentation examples for configuration, module usage, and CLI commands, **When** the audit is completed, **Then** each example is marked valid or invalid with a specific reason and source location.
2. **Given** an example that conflicts with published constraints, **When** the audit is completed, **Then** the report identifies the conflict and recommends a correction.

---

### User Story 3 - Prioritized Fix Plan (Priority: P3)

As a release manager, I want findings categorized by severity and recommended action so documentation fixes can be triaged effectively.

**Why this priority**: Prioritization ensures limited maintenance time is spent on the most impactful issues first.

**Independent Test**: A reviewer can filter the report by severity and see a clear, actionable fix recommendation for each finding.

**Acceptance Scenarios**:

1. **Given** findings with varying impact, **When** the audit is completed, **Then** each finding includes a severity level and a recommended action.

---

### Edge Cases

- Multiple documentation sources describe the same feature with conflicting behavior.
- Examples omit required inputs or rely on implicit defaults not documented.
- Documentation references deprecated or renamed commands.
- Documentation includes constraints that no longer apply or omits constraints that do apply.
- Examples rely on environment-specific assumptions that are not stated.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The audit MUST define the in-scope documentation sources and list them in the report.
- **FR-002**: The audit MUST inventory all in-scope user-facing surfaces and map each to a documentation reference or mark it as undocumented.
- **FR-003**: The audit MUST identify documentation statements that refer to features not present in the current user-facing surface.
- **FR-004**: Each finding MUST include a mismatch type, a source location, and the expected behavior as described by the current feature set.
- **FR-005**: Each documentation example MUST be evaluated and marked valid or invalid with a specific reason.
- **FR-006**: Findings MUST include a severity level (Critical / High / Medium / Low) and a recommended action.
- **FR-007**: The report MUST include summary metrics: total surfaces, coverage percentage, and counts by mismatch type and severity.
- **FR-008**: The report MUST document scope boundaries, assumptions, and exclusions.
- **FR-009**: The audit MUST be based on the current repository content and user-visible behavior only.
- **FR-010**: The report MUST define a single canonical location for examples/samples and flag any example content located outside that canonical location.
- **FR-011**: The audit report MUST be delivered as a single Markdown report file.
- **FR-012**: `invowk docs audit` MUST support human-readable output (default) and JSON output for its summary via `--output human|json` (or `--json`), without changing the Markdown report file output.

### Functional Acceptance Criteria

- **FAC-001 (FR-001)**: The report includes an explicit list of all in-scope documentation sources.
- **FAC-002 (FR-002)**: Every in-scope user-facing surface appears in the report with either a documentation reference or an "undocumented" label.
- **FAC-003 (FR-003)**: Each docs-only feature is listed with a source location and a brief description of the missing feature.
- **FAC-004 (FR-004)**: Each finding includes a mismatch type, a source location, and the expected behavior as stated by the current feature set.
- **FAC-005 (FR-005)**: Every documentation example is marked valid or invalid with a specific reason.
- **FAC-006 (FR-006)**: Each finding includes a severity level (Critical / High / Medium / Low) and a recommended action.
- **FAC-007 (FR-007)**: The report includes total surface count, coverage percentage, and counts by mismatch type and severity.
- **FAC-008 (FR-008)**: The report states scope boundaries, assumptions, and exclusions in a dedicated section.
- **FAC-009 (FR-009)**: Findings reference only the current repository content and user-visible behavior.
- **FAC-010 (FR-010)**: The report names the canonical examples/samples location and lists any example content found outside it.
- **FAC-011 (FR-011)**: The report is provided as one Markdown file.
- **FAC-012 (FR-012)**: JSON output includes the report path, total surfaces, coverage percentage, counts by mismatch type, and counts by severity.

### Key Entities *(include if feature involves data)*

- **Documentation Source**: A specific document or section reviewed, with its location reference.
- **User-Facing Surface**: A command, flag, configuration field, or documented behavior exposed to users.
- **Example**: A usage snippet intended for users to copy or follow.
- **Finding**: A detected mismatch, gap, or invalid example, with classification and evidence.
- **Mismatch Type**: The category of discrepancy (missing, outdated, incorrect, inconsistent).
- **Severity**: The impact level used for prioritization.
- **Recommendation**: The suggested remediation action for a finding.

## Assumptions

- The audit targets the current state of the repository at the time it is performed.
- In-scope documentation includes README, website documentation, in-repo guides, and sample directories (examples/ and modules/).
- User-facing surfaces include CLI commands, flags, configuration file fields, module definitions, and documented behaviors referenced in the docs.
- External blog posts, issue trackers, and roadmap discussions are out of scope unless explicitly referenced in in-repo documentation.
- Public Go packages under pkg/ are out of scope unless explicitly documented for end users.

## Dependencies

- Access to the current repository content and in-scope documentation sources at the time of the audit.
- A stable definition of the user-facing surface for the audited revision.
- Agreement on severity definitions used to prioritize findings (Critical / High / Medium / Low).
- Severity mapping uses user impact (task failure, confusion, or minor issue) as the basis.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of in-scope user-facing surfaces are inventoried and mapped to documentation or flagged as undocumented.
- **SC-002**: 100% of findings include a mismatch type, a source location, and a recommended action.
- **SC-003**: 100% of documented examples are either validated or explicitly marked invalid with a reason.
- **SC-004**: A maintainer can resolve the top 10 high-severity findings without additional clarification.
- **SC-005**: The report includes coverage percentage and counts by mismatch type and severity.
