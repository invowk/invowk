# Research Notes: Documentation/API Audit

## Decision 1: Severity scale
Decision: Critical / High / Medium / Low based on user impact (task failure, confusion, or minor issue).
Rationale: Matches clarified requirement and supports consistent triage.
Alternatives considered: P1/P2/P3/P4; Blocker/Major/Minor/Trivial.

## Decision 2: Report format
Decision: Single Markdown report file.
Rationale: Aligns with the spec and provides human-readable output for maintainers.
Alternatives considered: Markdown + JSON; JSON only.

## Decision 3: In-scope documentation sources
Decision: README, website docs, in-repo guides, and sample directories (examples/ and modules/).
Rationale: These are user-facing references and common copy sources.
Alternatives considered: Exclude sample directories.

## Decision 4: User-facing surface scope
Decision: Include CLI commands, flags, config fields, module definitions, and documented behaviors; exclude pkg/ APIs unless explicitly documented for end users.
Rationale: Keeps the audit aligned with the user-facing surface definition.
Alternatives considered: Include all public packages.

## Decision 5: Canonical examples location
Decision: Treat examples/ as the canonical examples/samples location and flag example content outside it (including modules/).
Rationale: examples/ is the dedicated samples directory today; centralizing reduces ambiguity.
Alternatives considered: New samples/ root; modules/ as canonical.
