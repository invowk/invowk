# Quickstart: Documentation/API Audit

## Goal
Generate a single Markdown report that maps user-facing surfaces to documentation, validates examples, and ranks issues by severity.

## Run the audit
```
invowk docs audit --out docs-audit.md
```

## What the report includes
- In-scope documentation sources and scope boundaries
- Coverage summary and counts by mismatch type and severity
- Findings with source locations, expected behavior, and recommendations
- Example validation results and canonical examples location

## Act on findings
- Fix mismatches in docs or behavior
- Move or consolidate samples into the canonical examples location
- Re-run the audit to verify improvements
