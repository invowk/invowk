## Decisions

- **Reject, don't warn.** A hashless entry cannot detect tampering, so accepting it with a warning keeps the gap open; `invowk audit` already warned. The error names the one-command migration.
- **Refetch in sync.** The commit check still applies to v1.0 entries, so the fetched copy is trustworthy; the cached copy is not, so it is dropped before caching.
- **No automatic lock rewrite.** Upgrading the lock is an explicit `module sync`, keeping the lock file generated only by module commands.

## Risks / Trade-offs

- Existing projects with v1.0 locks and vendored modules fail until they sync once; the error message and docs give the command.
