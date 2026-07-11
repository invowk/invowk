# Live Test Inventory

Generate this inventory at the start of every audit. Do not record snapshot
counts or file tables here: the test surface changes frequently, and stale
inventories undermine deterministic review.

## Complete Sorted Scope

```bash
rg --files cmd internal pkg tests tools -g '*_test.go' | sort
rg --files tests/cli/testdata -g '*.txtar' | sort
```

Save the command output in the review context block and pass the relevant slice
to each surface reviewer. Review every current file in scope; do not sample.

## Counts and Line Sizes

```bash
go_files=$(rg --files cmd internal pkg tests tools -g '*_test.go' | sort)
txtar_files=$(rg --files tests/cli/testdata -g '*.txtar' | sort)
printf 'go_test_files=%s\n' "$(printf '%s\n' "$go_files" | sed '/^$/d' | wc -l)"
printf 'txtar_files=%s\n' "$(printf '%s\n' "$txtar_files" | sed '/^$/d' | wc -l)"
printf '%s\n' "$go_files" | sed '/^$/d' | xargs -r wc -l
```

For a directory summary:

```bash
rg --files cmd internal pkg tests tools -g '*_test.go' \
  | awk -F/ '{ if ($1 == "internal" || $1 == "pkg" || $1 == "tools") print $1 "/" $2; else print $1 }' \
  | sort | uniq -c | sort -k2,2
```

## Runtime Mirror Pairs

The machine-enforced exemption source is
`tests/cli/runtime_mirror_exemptions.json`. Generate candidate pairs live:

```bash
for virtual in $(rg --files tests/cli/testdata -g 'virtual_*.txtar' | sort); do
  base=${virtual##*/virtual_}
  native=${virtual%/virtual_*}/native_${base}
  if test -f "$native"; then
    printf 'PAIR %s %s\n' "$virtual" "$native"
  else
    printf 'UNPAIRED %s\n' "$virtual"
  fi
done
```

Then rely on behavioral guardrails rather than inferring exemptions from names:

```bash
go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...
```

## Surface Allocation

- SS1-SS3: all live `*_test.go` files in the checklist scope.
- SS4: integration tests plus current CI workflow and helper files named by the
  checklist.
- SS5: all live `.txtar` files and testscript harness files.
- SS6: live virtual/native pair output, the exemption JSON, and platform tests.
- SS7: coverage/guardrail files named by the checklist plus their current tests.
- SS8: live TUI, server, container, watcher, benchmark, and goplint test files
  selected by the checklist.
