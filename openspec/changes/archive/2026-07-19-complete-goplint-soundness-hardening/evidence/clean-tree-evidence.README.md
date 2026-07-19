# Clean-tree evidence staging

Task 12.6 completed with the reviewed final intended path selection. The
reproducible invocation was:

```bash
cd tools/goplint
go run ./cmd/clean-tree-evidence \
  -root ../.. \
  -paths openspec/changes/complete-goplint-soundness-hardening/evidence/intended-combined-diff.paths \
  -plan tools/goplint/testdata/gates/clean-tree-v1.json \
  -evidence-dir openspec/changes/complete-goplint-soundness-hardening/evidence
```

The path-selection file is deliberately not generated from the dirty worktree.
It must contain reviewed repository-relative paths, one per line. The tool uses
a temporary Git index, writes a synthetic tree and unreferenced synthetic
commit, materializes a clean detached worktree, runs the command plan, and
records identities, toolchain, command logs, and outcomes in
`clean-tree-run.v1.json`. Commands receive a proof-local golangci-lint cache so
deleted worktree paths cannot leak through host cache entries. The tool never
writes the caller's index.

The retained `clean-tree-run.v1.json` is the sole authority for the base
commit, synthetic tree and commit, intended-diff hash, toolchain, command
outcomes, and logs. Keeping those generated identities out of this input file
avoids a self-referential proof cycle. Cross-change reconciliation enumerates
all 12 predecessor requirements and all 66 completed predecessor tasks; every
`final_proof` field points to the authoritative passed run record.
