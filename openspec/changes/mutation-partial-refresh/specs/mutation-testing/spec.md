# Spec Delta

## ADDED Requirements

### Requirement: Partial refresh of formal-bindings survivors
A committed partial-refresh command SHALL rewrite the formal-bindings baseline and triage ledger entries of chosen symbols from a report-only formal-bindings run. A symbol is chosen by name (optionally qualified by its file), or because one of its ledger entries fails the staleness or anchor check. The command SHALL refuse a chosen symbol that still exists but that the formal-bindings plan does not target.

#### Scenario: Symbols chosen by name
- **WHEN** the refresh runs with one or more symbol names
- **THEN** it SHALL refresh exactly those symbols, and SHALL fail with a message naming the symbol when a name matches no planned function or matches planned functions in several files without a file qualifier

#### Scenario: Symbols chosen by staleness
- **WHEN** the refresh runs in stale mode
- **THEN** it SHALL refresh every symbol that has a ledger entry failing the staleness or anchor check of the triage check, including entries listed in `[triage] stale_survivors`, and SHALL NOT choose orphaned survivors' symbols

#### Scenario: Unplanned symbol
- **WHEN** a chosen symbol still names a function in its file but is not a target of the formal-bindings plan, such as an orphaned survivor's symbol
- **THEN** the refresh SHALL fail without writing any file

#### Scenario: Deleted or renamed function
- **WHEN** a chosen symbol no longer names a function in its file
- **THEN** the refresh SHALL remove all of its entries, add none, and say so in the summary

### Requirement: The refresh removes survivors the run no longer reports as escaped
For each chosen symbol, the refresh SHALL remove from the baseline and the ledger every entry whose id the run did not report as escaped: an id the run no longer generates, or one it reports killed, skipped or errored. It SHALL also remove each removed id from `[triage] stale_survivors`.

#### Scenario: Removed id is dropped
- **WHEN** a chosen symbol's ledger entry names an id the run does not generate
- **THEN** the refresh SHALL delete that id from the baseline, the ledger and `[triage] stale_survivors`, and SHALL count it as removed

#### Scenario: Baselined id now killed
- **WHEN** the run reports a chosen symbol's baselined id as killed
- **THEN** the refresh SHALL remove it as no longer a survivor and SHALL say it was killed

### Requirement: The refresh adds new escaped mutants as untriaged survivors
For each chosen symbol, the refresh SHALL add every id the run reports as escaped in that symbol and the baseline does not list. Each new baseline entry SHALL carry go-mutesting's fields from the run and its own evidence `digest`; each new ledger entry SHALL carry its anchor derived from the run's diff and an untriaged placeholder for its class and reason. Ids the run reports with any other status SHALL NOT be added.

#### Scenario: New escapee is added
- **WHEN** the run reports an escaped mutant in a chosen symbol whose id is not baselined
- **THEN** the refresh SHALL add one baseline entry for the id, with its file, mutator, line and the run's plan-inputs digest as its own `digest`, and one ledger entry whose anchor hashes to the id

#### Scenario: Killed id is not added
- **WHEN** the run reports a mutant in a chosen symbol as killed, skipped or errored and its id is not baselined
- **THEN** the refresh SHALL NOT add it

#### Scenario: Escapees of other symbols are ignored
- **WHEN** the run reports an escaped, unbaselined mutant in a symbol that was not chosen
- **THEN** the refresh SHALL NOT add it

### Requirement: Untriaged survivors fail the triage check
The triage check SHALL fail on every ledger entry that carries the refresh's untriaged placeholder, in its class or in its reason, with a message that names the id and symbol and says to write the class and reason. The placeholder SHALL NOT be a valid class.

#### Scenario: Placeholder after a refresh
- **WHEN** the triage check runs on a ledger to which the refresh added an entry
- **THEN** it SHALL fail for that entry with the untriaged message

#### Scenario: Class written but reason left
- **WHEN** a person sets a valid class on a refreshed entry but leaves the placeholder reason
- **THEN** the triage check SHALL still fail for that entry

#### Scenario: Triaged entry passes
- **WHEN** a refreshed entry has a valid class, a written reason, the follow-up its class needs, and two recorded escaped reruns at its own digest
- **THEN** the triage check SHALL raise no failure for that entry

### Requirement: The refresh leaves other entries byte-identical
The refresh SHALL leave every baseline entry, ledger entry, ledger comment and `[triage]` entry that it does not remove or add byte-identical, including the entries it keeps in the chosen symbols and the baseline's top-level digest.

#### Scenario: Unrelated entries unchanged
- **WHEN** the refresh adds and removes entries of one symbol
- **THEN** the diff of the baseline, the ledger and `formal/manifest.toml` SHALL contain only the added and removed entries

#### Scenario: Kept survivor unchanged
- **WHEN** a chosen symbol's baselined id is reported escaped again
- **THEN** the refresh SHALL keep its baseline and ledger entries unchanged and count it as kept

#### Scenario: Refresh is idempotent
- **WHEN** the refresh runs twice with the same run and the same symbols
- **THEN** the second run SHALL change no file and report every entry of those symbols as kept

### Requirement: The refresh uses a run of the current inputs
A full formal-bindings run SHALL record its plan-inputs digest with its reports. The refresh SHALL fail without writing any file when the run has no recorded digest, or when the digest of the current tree's plan inputs differs from it, and SHALL name the run command to repeat.

#### Scenario: Run of other inputs
- **WHEN** a target file or killer-test file changed after the run
- **THEN** the refresh SHALL fail, naming both digests and `make mutation-formal`

#### Scenario: Run in another worktree
- **WHEN** the run was made in a throwaway worktree at a commit whose plan inputs equal the current tree's
- **THEN** the refresh SHALL accept it and resolve the reported file paths against that run's root

### Requirement: The refresh checks rerun evidence and reports a summary
After writing, the refresh SHALL check, for every remaining entry of the chosen symbols, the rerun evidence the triage check requires, and SHALL print the ids that lack it with the `make mutation-formal-rerun` command to run. It SHALL NOT run mutation itself. It SHALL print counts of removed, added, untriaged, kept and awaiting-evidence entries.

#### Scenario: Evidence missing after adding
- **WHEN** the refresh adds entries that have no recorded reruns at their digest
- **THEN** it SHALL list them as awaiting evidence and print `make mutation-formal-rerun MUTATION_MUTANT_ID=<ids>` with the note that each id needs two escaped reruns

#### Scenario: Evidence complete
- **WHEN** every remaining entry of the chosen symbols has two escaped reruns at its evidence digest
- **THEN** the summary SHALL report zero awaiting evidence

#### Scenario: Make target
- **WHEN** a maintainer runs `make mutation-formal-refresh` with symbols or the stale mode
- **THEN** it SHALL run the refresh on the default run report directory, and print usage and fail when neither is given
