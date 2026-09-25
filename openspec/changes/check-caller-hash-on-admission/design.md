## Decisions

- **Check at admission, not discovery.** Discovery verifies each vendored copy against the lock of the module that vendored it, which is correct for that module. The caller's expectation only exists when its scope is built, so the comparison belongs in `IsDeclaredLockedCommandSource`.
- **Compare with the caller's lock.** Comparing with the sibling's lock always agrees (the model's `compare_with_sibling_lock` mutant), so it cannot detect the mismatch.
- **Hash once per module directory.** `buildCommandScope` memoizes the decision per (module ID, source, directory), since all commands of a module share its directory.
- **Fail closed.** A recorded hash with a directory that cannot be hashed is not admitted.

## Risks / Trade-offs

- Projects with deliberately divergent dependency versions across modules now fail validation. That divergence was the unverified case.
