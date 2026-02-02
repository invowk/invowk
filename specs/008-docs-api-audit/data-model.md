# Data Model: Documentation/API Audit

## Documentation Source
- id: string (path or URL)
- kind: README | website-doc | guide | sample
- location: string (path)
- scopeNotes: string (optional)

## User-Facing Surface
- id: string
- type: command | flag | config-field | module | behavior
- name: string
- sourceLocation: string
- documentationRefs: [Documentation Source id]

## Example
- id: string
- sourceLocation: string
- relatedSurfaceId: string (optional)
- status: valid | invalid
- invalidReason: string (required when status is invalid)

## Finding
- id: string
- mismatchType: missing | outdated | incorrect | inconsistent
- severity: Critical | High | Medium | Low
- sourceLocation: string
- expectedBehavior: string
- recommendation: string
- relatedSurfaceId: string (optional)

## Audit Report
- generatedAt: timestamp
- scope: {docSources, exclusions, canonicalExamplesPath}
- metrics: {totalSurfaces, coveragePercentage, countsByMismatchType, countsBySeverity}
- findings: [Finding]
- examples: [Example]

## Relationships
- Documentation Source -> Example (one-to-many)
- User-Facing Surface -> Example (optional one-to-many)
- User-Facing Surface -> Finding (one-to-many)
