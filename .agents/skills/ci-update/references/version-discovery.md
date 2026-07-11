# Version Discovery

Load this reference after building the live version inventory. Use Context7
first for current CLI/tool semantics; use the commands below for version facts.

## Go Tools

Query the module named by the import path rather than assuming the installed
binary's module:

```bash
go list -m -versions gotest.tools/gotestsum
go list -m -versions golang.org/x/vuln
go list -m -versions golang.org/x/perf
```

For pseudo-versions, check whether the module now has a suitable tagged release
and inspect the module's official release notes before recommending a change.

## Released Binaries and Actions

Use the official GitHub release endpoint for repositories that publish
releases:

```bash
gh api repos/OWNER/REPO/releases/latest --jq '.tag_name'
```

Apply it to every repository discovered from the inventory, including inline
binaries and actions. Compare action majors; Dependabot handles covered updates
within the repository's configured policy. Classify a branch pin separately as
an explicit exception or drift risk.

If an action has no GitHub Release, inspect its official tags and maintenance
policy rather than treating a missing release as "up to date."

## npm and MCP Packages

Use the official npm registry metadata:

```bash
npm view PACKAGE version
```

If npm is unavailable, query `https://registry.npmjs.org/PACKAGE/latest` and
record that fallback. Use Context7 or the package's official documentation for
configuration and migration semantics, not for latest-version discovery.

## GoReleaser and Node.js

- Query GoReleaser's official GitHub releases, then read the official migration
  notes for relevant changes within the configured major track.
- Query Node.js release lifecycle data from the official Node.js release
  schedule. If using a lifecycle aggregator as a fallback, label it as such and
  confirm the target major against Node.js documentation.

## Evidence Rules

- Record the source and query date for every "latest" claim.
- Report authentication, rate-limit, network, or parsing failures.
- Never substitute remembered versions for failed queries.
- Keep current version, latest version, coverage owner, risk, and affected
  locations as separate report columns.
