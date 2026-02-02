# Implementation Plan: Documentation/API Audit

**Branch**: `008-docs-api-audit` | **Date**: 2026-02-02 | **Spec**: /var/home/danilo/Workspace/github/invowk/invowk/specs/008-docs-api-audit/spec.md
**Input**: Feature specification from `/var/home/danilo/Workspace/github/invowk/invowk/specs/008-docs-api-audit/spec.md`

## Summary

Deliver a documentation/API audit that inventories user-facing surfaces (CLI, config, modules, docs), validates examples, and outputs a single Markdown report with coverage metrics, mismatch types, and severity-based recommendations. Implementation will scan in-scope sources (README, website docs, in-repo guides, examples/, modules/), exclude pkg/ APIs unless explicitly documented, apply a Critical/High/Medium/Low severity scale based on user impact, and define a canonical examples location in the report.

## Technical Context

**Language/Version**: Go 1.25+  
**Primary Dependencies**: Go stdlib; existing repo libraries (cobra, viper, cuelang.org/go, mvdan.cc/sh, testscript)  
**Storage**: File system only (read-only scan of repo contents)  
**Testing**: go test + testscript CLI integration tests  
**Target Platform**: Cross-platform CLI (Linux/macOS/Windows); container runtime Linux-only (Debian-based)  
**Project Type**: single  
**Performance Goals**: Produce the audit report in <=2 minutes for the current repo size on a typical developer machine  
**Constraints**: Single Markdown report output; no persistent storage; avoid new heavy dependencies; follow Debian-only container guidance if container runtime is used  
**Scale/Scope**: Single repository audit; dozens of documentation sources; hundreds of user-facing surfaces

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Phase 0

- I. Idiomatic Go & Schema-Driven Design: Pass (use existing Go idioms and CUE constraints if schemas change)
- II. Comprehensive Testing Discipline: Pass (plan includes unit and CLI test coverage)
- III. Consistent User Experience: Pass (CLI pattern and output conventions maintained)
- IV. Single-Binary Performance: Pass (lightweight scan, no heavy deps)
- V. Simplicity & Minimalism: Pass (scope limited to audit/report)
- VI. Documentation Synchronization: Pass (feature enforces doc sync)
- VII. Pre-Existing Issue Resolution: Pass (halt and report if discovered)

### Post-Phase 1 Re-check

- All principles remain Pass based on Phase 1 artifacts (research, data model, contracts, quickstart).

## Project Structure

### Documentation (this feature)

```text
specs/008-docs-api-audit/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
internal/
pkg/
modules/
examples/
tests/
website/
README.md
invkfile.cue
```

**Structure Decision**: Single Go CLI repository using the existing root layout (cmd/, internal/, pkg/, tests/, website/).
