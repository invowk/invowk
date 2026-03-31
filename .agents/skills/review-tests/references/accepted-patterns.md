# Accepted Test Patterns Registry

Patterns initially flagged during audit that were determined to be justified in context.
Unlike permanent exceptions in `known-exceptions.md`, accepted patterns carry
**reconsideration triggers** -- conditions under which the acceptance should be re-evaluated.

The goal is to eliminate false-positive noise from audits while preserving the ability to
revisit decisions as the codebase, tooling, or Go version evolves.

---

## Confidence Levels

Each accepted pattern has a confidence level that determines how frequently it is reconsidered.

| Level | Meaning | Review Cadence | Typical Age |
|---|---|---|---|
| **SETTLED** | Well-understood justification; multiple rounds confirm it is the right choice | Every 6 audit rounds | 6+ rounds |
| **PROVISIONAL** | Justified given current constraints, but constraints are known to be evolving | Every 3 audit rounds | 3-5 rounds |
| **EXPERIMENTAL** | Recently accepted; justification is plausible but not yet validated across rounds | Next audit round | 0-2 rounds |

### Graduation and Demotion

- **Promotion**: An EXPERIMENTAL pattern that remains justified after 3 consecutive rounds
  promotes to PROVISIONAL. A PROVISIONAL pattern that remains justified after 6 total rounds
  promotes to SETTLED.
- **Graduation to permanent exception**: A SETTLED pattern that has been stable for 12+ rounds
  AND whose reconsideration triggers are all structurally unlikely (e.g., would require a major
  Go version change or CUE library rewrite) may be moved to `known-exceptions.md` as a
  permanent exception. This is a deliberate decision, not automatic.
- **Demotion**: If a reconsideration trigger fires, the pattern drops to EXPERIMENTAL regardless
  of its current level. It must then be re-evaluated in the next audit round: either re-accepted
  with updated justification or removed from the registry (becoming a real finding again).
- **Removal**: If during re-evaluation the pattern is no longer justified, remove it from the
  registry. Subsequent audit rounds will flag it normally.

---

## Registry

### How to Read the Table

| Column | Purpose |
|---|---|
| **Pattern** | Brief description of the code pattern that looks like a violation but is justified |
| **Affected Checks** | Checklist item IDs (e.g., T3-C05) that would flag this pattern |
| **Files** | Specific files or file-scope where this acceptance applies (not blanket) |
| **Justification** | Why the pattern is correct despite looking like a violation |
| **Confidence** | SETTLED / PROVISIONAL / EXPERIMENTAL |
| **Reconsideration Triggers** | Conditions that would invalidate the acceptance |
| **Accepted** | Date the pattern was first accepted (YYYY-MM-DD) |
| **Last Reviewed** | Date of most recent review (YYYY-MM-DD) |

### Currently Accepted Patterns

*No patterns currently in the registry. Entries are added when an audit finding is determined
to be a justified deviation that does not fit the permanent exception categories in
`known-exceptions.md`.*

<!-- TEMPLATE for adding entries:

| Pattern | Affected Checks | Files | Justification | Confidence | Reconsideration Triggers | Accepted | Last Reviewed |
|---|---|---|---|---|---|---|---|
| Description of the pattern | T{N}-C{NN} | `path/to/file.go` | Why it is correct | EXPERIMENTAL | 1) trigger one 2) trigger two | YYYY-MM-DD | YYYY-MM-DD |

-->

---

## When to Use This Registry vs known-exceptions.md

| Use This Registry When... | Use known-exceptions.md When... |
|---|---|
| The justification depends on current Go version, library versions, or codebase structure | The justification is structural and unlikely to change (e.g., "CUE is not thread-safe") |
| The pattern might become a real issue if constraints change | The pattern is a permanent design decision |
| The acceptance is recent and not yet battle-tested | The exception has been stable across many audit rounds and codebase changes |
| You want future audits to periodically re-evaluate | You want future audits to skip without question |

**Gray area**: If unsure, default to this registry with EXPERIMENTAL confidence. It is always
safer to accept provisionally and graduate to permanent than to prematurely add a permanent
exception that masks a future issue.

---

## How to Add Entries

When an audit finding is determined to be a justified deviation:

1. **Verify it does not fit an existing `known-exceptions.md` category.** If it does, add it
   there instead (permanent exceptions have well-defined structural categories).
2. **Write a specific justification.** "It works" is not a justification. Explain the technical
   reason the pattern is correct despite violating the checklist item.
3. **Define at least one reconsideration trigger.** Every acceptance must have a concrete
   condition under which it should be re-evaluated. Good triggers:
   - "Go version >= X adds API Y that replaces this workaround"
   - "If CUE library adds thread-safe context, serial subtests can be parallelized"
   - "If the error type is refactored to use sentinel wrapping"
   - "If the file is split and the import cycle that forces this pattern is resolved"
   Bad triggers (too vague): "If things change", "If we decide to fix it later"
4. **Scope to specific files.** Do not add blanket acceptances. If the pattern is justified in
   `pkg/invowkfile/validation_test.go`, list that file -- not "all of pkg/invowkfile/".
5. **Set confidence to EXPERIMENTAL** for all new entries unless promoting from a known
   in-round re-evaluation.
6. **Set Accepted and Last Reviewed to today's date.**
7. **Mark the original finding as severity SKIP** with a reference: "Accepted pattern -- see
   `accepted-patterns.md`".

---

## Reconsideration Protocol

The coordinator runs this protocol during Step 3 (Merge and Report), after cross-checking
findings against `known-exceptions.md`.

### Automatic Reconsideration

For each entry in the registry:

1. **Check review cadence.** If the entry's confidence level cadence has elapsed since
   Last Reviewed (EXPERIMENTAL: 1 round, PROVISIONAL: 3 rounds, SETTLED: 6 rounds),
   the entry is due for review.
2. **Evaluate reconsideration triggers.** Check whether any trigger condition has been met
   (e.g., Go version bumped, library updated, file restructured).
3. **Update or flag.**
   - If triggers are NOT met and the cadence review is routine: update Last Reviewed date,
     promote confidence if eligible, and continue.
   - If any trigger IS met: flag the entry in the "Patterns for Reconsideration" section
     of the report. The pattern remains SKIP for this round but is demoted to EXPERIMENTAL
     for the next round so it gets full re-evaluation.
   - If the pattern no longer exists in the codebase (file deleted, code refactored):
     remove the entry from the registry.

### Reconsideration Report Section

The coordinator appends a "Patterns for Reconsideration" section to the final report listing
any entries that are due for review or whose triggers have fired. This section is informational
-- it does not create findings, but surfaces patterns that the team should consciously re-evaluate.

Format:

```
PATTERNS FOR RECONSIDERATION
==============================
AP-001: [Pattern description]
  Confidence : PROVISIONAL -> EXPERIMENTAL (trigger fired)
  Trigger    : "Go 1.26 adds testing.B.Context()" -- NOW MET
  Action     : Re-evaluate in next audit round; if still justified, re-accept

AP-002: [Pattern description]
  Confidence : EXPERIMENTAL (routine review due)
  Status     : Last reviewed 2 rounds ago; cadence is every 1 round
  Action     : Verify justification still holds; promote to PROVISIONAL if confirmed
==============================
```
