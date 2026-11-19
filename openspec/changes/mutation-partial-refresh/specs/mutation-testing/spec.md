# Spec Delta

## ADDED Requirements

### Requirement: Partial refresh of formal-bindings survivors
A committed partial-refresh command SHALL rewrite the formal-bindings baseline and triage ledger entries of chosen symbols from a report-only formal-bindings run. A symbol is chosen by name (optionally qualified by its file), or because one of its ledger entries fails the staleness or anchor check. The command SHALL refuse a chosen symbol that still exists but that the formal-bindings plan does not target.

#### Scenario: Symbols chosen by name
- **WHEN** the refresh runs with one or more symbol names
- **THEN** it SHALL refresh exactly those symbols, and SHALL fail without writing, with a message naming the symbol, when a name matches planned functions in several files without a file qualifier, or matches no planned function and has no ledger entries in a file that no longer declares it

#### Scenario: Symbols chosen by staleness
- **WHEN** the refresh runs in stale mode
- **THEN** it SHALL refresh every symbol that has a ledger entry failing the staleness or anchor check of the triage check, including entries listed in `[triage] stale_survivors`, and SHALL NOT choose orphaned survivors' symbols

#### Scenario: Unplanned symbol
- **WHEN** a chosen symbol still names a function in its file but is not a target of the formal-bindings plan, such as an orphaned survivor's symbol
- **THEN** the refresh SHALL fail without writing any file

#### Scenario: Deleted or renamed function
- **WHEN** a symbol chosen by name or by staleness has ledger entries, matches no planned function, and no longer names a function in its file
- **THEN** the refresh SHALL remove all of its entries, add none, and say so in the summary

#### Scenario: Row still names the deleted function
- **WHEN** a correspondence row still names a function that no longer exists, so the formal-bindings plan cannot be built
- **THEN** the refresh SHALL fail without writing and say to update the row first

### Requirement: The refresh attributes mutants to plan functions
Each reported mutant SHALL belong to the plan function whose line range contains its original start line. A symbol's escaped ids SHALL be the ids with at least one escaped mutant attributed to it. Baseline entries SHALL belong to a symbol only through the ledger entry of the same id.

#### Scenario: Mutant outside every plan function
- **WHEN** the run reports a mutant whose line lies in no plan function of its file
- **THEN** the refresh SHALL count and list it as unattributed and SHALL NOT add it

#### Scenario: Id both killed and escaped
- **WHEN** the run reports one mutant of a chosen symbol's baselined id as killed and another mutant with the same id in that symbol as escaped
- **THEN** the refresh SHALL count the id as escaped in that symbol and keep the entry

#### Scenario: Id shared with a symbol that was not chosen
- **WHEN** an escaped id of a chosen symbol is ledgered under a symbol that was not chosen, as when the same line is mutated the same way in two functions of one file
- **THEN** the refresh SHALL leave both its entries unchanged, report it as shared with its owner, and SHALL NOT fail

### Requirement: The refresh removes survivors the run no longer reports as escaped
For each chosen symbol, the refresh SHALL remove every ledger entry of that symbol, and the baseline entry of the same id, whose id the run reports escaped in no mutant: an id the run no longer generates, or one it reports only killed, skipped or errored. It SHALL remove every id of a chosen symbol from `[triage] stale_survivors`.

#### Scenario: Removed id is dropped
- **WHEN** a chosen symbol's ledger entry names an id the run does not generate
- **THEN** the refresh SHALL delete that id from the baseline, the ledger and `[triage] stale_survivors`, and SHALL count it as removed

#### Scenario: Baselined id now killed
- **WHEN** the run reports a chosen symbol's baselined id as killed
- **THEN** the refresh SHALL remove it as no longer a survivor and SHALL say it was killed

### Requirement: The refresh moves ids between chosen symbols only
An entry of a chosen symbol whose id escapes only in other symbols SHALL move to the chosen one of them as a new untriaged entry. When the id escapes only in symbols that were not chosen, the refresh SHALL fail without writing and name them.

#### Scenario: Id moves to another chosen symbol
- **WHEN** a chosen symbol's baselined id escapes only in another chosen symbol
- **THEN** the refresh SHALL remove it from the first symbol and add it to the second as an untriaged entry

#### Scenario: Id moves to a symbol that was not chosen
- **WHEN** a chosen symbol's baselined id escapes only in symbols that were not chosen
- **THEN** the refresh SHALL fail without writing any file and name those symbols

### Requirement: The refresh adds new escaped mutants as untriaged survivors
For each chosen symbol, the refresh SHALL add every id the run reports as escaped in that symbol and the baseline does not list. Each new baseline entry SHALL carry go-mutesting's fields from the run and its own evidence `digest`; each new ledger entry SHALL carry its anchor derived from the run's diff and an untriaged placeholder for its class and reason. Ids the run reports with any other status SHALL NOT be added, and an id escaped in several chosen symbols SHALL be added once.

#### Scenario: New escapee is added
- **WHEN** the run reports an escaped mutant in a chosen symbol whose id is not baselined
- **THEN** the refresh SHALL add one baseline entry for the id, with its file, mutator, line and the run's plan-inputs digest as its own `digest`, and one ledger entry whose anchor hashes to the id

#### Scenario: Killed id is not added
- **WHEN** the run reports a mutant in a chosen symbol as killed, skipped or errored and its id is not baselined
- **THEN** the refresh SHALL NOT add it

#### Scenario: Escapees of other symbols are ignored
- **WHEN** the run reports an escaped, unbaselined mutant in a symbol that was not chosen
- **THEN** the refresh SHALL NOT add it

#### Scenario: Placement of a new entry
- **WHEN** the refresh adds a ledger entry
- **THEN** it SHALL insert it after the last ledger entry of the same symbol, else of the same file, else before the first closed entry, and SHALL insert the baseline entry at its sort position

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
The refresh SHALL leave every baseline entry, ledger entry, ledger comment and `[triage]` entry that it does not remove, add or re-anchor byte-identical, including the baseline entries it keeps in the chosen symbols and the baseline's top-level digest.

#### Scenario: Unrelated entries unchanged
- **WHEN** the refresh adds and removes entries of one symbol
- **THEN** the diff of the baseline, the ledger and `formal/manifest.toml` SHALL contain only the added, removed and re-anchored entries of that symbol and its ids' `stale_survivors` lines

#### Scenario: Kept survivor unchanged
- **WHEN** a chosen symbol's baselined id is reported escaped again and its ledger entry passes the anchor check
- **THEN** the refresh SHALL keep its baseline and ledger entries unchanged and count it as kept

#### Scenario: Refresh is idempotent
- **WHEN** the refresh runs twice with the same run and the same symbols
- **THEN** the second run SHALL change no file, re-anchor nothing, and report every entry of those symbols as kept

### Requirement: Kept entries of chosen symbols pass the anchor check after a refresh
A kept ledger entry that fails the anchor check SHALL have only its anchor keys (`removed`, `added`, `context`) rewritten from the run's diff. After the refresh, every remaining entry of a chosen symbol SHALL pass the staleness and anchor checks and SHALL NOT be listed in `[triage] stale_survivors`.

#### Scenario: Kept survivor with a failing anchor
- **WHEN** a chosen symbol's baselined id is reported escaped again but its ledger entry fails the anchor check, for example because the context line of an insertion-only mutant changed, or it is listed in `[triage] stale_survivors`
- **THEN** the refresh SHALL rewrite only that entry's anchor keys from the run's diff, keep its class and reason, remove the id from `[triage] stale_survivors`, count it as kept and re-anchored, and the triage check SHALL raise no staleness or anchor failure for it

### Requirement: The refresh fails closed before writing
Before writing, the refresh SHALL check that the baseline round-trips through its serializer, that the new ledger parses back to the old entries plus the added and minus the removed ones with only re-anchored entries' anchor keys changed, and that every remaining entry of a chosen symbol passes the anchor check. When any check fails, it SHALL write no file.

#### Scenario: Parse-back failure
- **WHEN** the new ledger text would not parse back to the expected entries
- **THEN** the refresh SHALL fail without writing any file

#### Scenario: Anchor still failing
- **WHEN** a remaining entry of a chosen symbol would still fail the anchor check after the refresh
- **THEN** the refresh SHALL fail without writing any file and name the entry

### Requirement: The refresh uses a complete run of the current inputs
A full formal-bindings run SHALL record its plan-inputs digest and each match group's exit status with its reports. The refresh SHALL fail without writing any file, naming the run command to repeat, when the run has no recorded digest, when the digest of the current tree's plan inputs differs from it, or when the run is incomplete: a plan group has no report, no recorded status, or a status other than success or the escaped-mutants quality-gate status.

#### Scenario: Run of other inputs
- **WHEN** a target file or killer-test file changed after the run
- **THEN** the refresh SHALL fail, naming both digests and `make mutation-formal`

#### Scenario: Incomplete run
- **WHEN** a match group of the run crashed, timed out or was not run, so it has no report or a failing status
- **THEN** the refresh SHALL fail without writing any file, naming the groups and `make mutation-formal`, and SHALL NOT remove any entry as not generated

#### Scenario: Run in another worktree
- **WHEN** the run was made in a throwaway worktree at a commit whose plan inputs equal the current tree's
- **THEN** the refresh SHALL accept it and resolve the reported file paths against that run's root

### Requirement: The refresh checks rerun evidence and reports a summary
After writing, the refresh SHALL check, for every remaining entry of the chosen symbols, the rerun evidence the triage check requires, and SHALL print the ids that lack it with the `make mutation-formal-rerun` command to run. It SHALL NOT run mutation itself. It SHALL print counts of removed, moved, added, untriaged, kept, re-anchored, shared, unattributed and awaiting-evidence entries.

#### Scenario: Evidence missing after adding
- **WHEN** the refresh adds entries that have no recorded reruns at their digest
- **THEN** it SHALL list them as awaiting evidence and print `make mutation-formal-rerun MUTATION_MUTANT_ID=<ids>` with the note that each id needs two escaped reruns

#### Scenario: Evidence complete
- **WHEN** every remaining entry of the chosen symbols has two escaped reruns at its evidence digest
- **THEN** the summary SHALL report zero awaiting evidence

#### Scenario: Make target
- **WHEN** a maintainer runs `make mutation-formal-refresh` with symbols or the stale mode
- **THEN** it SHALL run the refresh on the default run report directory, and print usage and fail when neither is given
