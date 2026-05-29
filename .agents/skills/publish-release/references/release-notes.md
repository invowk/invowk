# Release Notes

Use this reference whenever drafting, reviewing, or approving release notes.

## Required Structure

```markdown
## Poetic Opening

> ...

— Author, "Poem"

A short, warm bridge from the stanza into the release.

## What's Changed

- ...

## Manual Actions Needed

- ...

## Warnings and Deprecations

- ...

## Bug Fixes

- ...
```

Keep all five headers even when a section is empty. Use plain entries such as "Nothing required" or "No known deprecations" instead of dropping a section.

## Tone

- Sound warm, collaborative, and concrete.
- Explain what changed in user terms before naming internal machinery.
- Prefer "You can now..." and "This release makes..." over commit-subject fragments.
- Be honest about rough edges, alpha-state breaks, migration steps, and known issues.
- Do not bury manual work in "What's Changed"; place it under `Manual Actions Needed`.

## Semantic Analysis Required

Do not rely only on conventional commits. Always combine at least:

```bash
previous=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1)
git log --no-merges --format='%H%x09%s%x09%b%x1e' "$previous..HEAD"
git diff --stat "$previous..HEAD"
git diff --name-status "$previous..HEAD"
git diff "$previous..HEAD" -- cmd internal pkg tests docs website README.md .github .goreleaser.yaml scripts Makefile openspec
```

If preparing notes for a tag that is not `HEAD`, compare `previous..target-ref`.

Look specifically for:

- CLI commands, flags, environment variables, exit behavior, output text, and config defaults.
- CUE schema fields, validation, defaults, examples, and migration needs.
- Runtime behavior for native, virtual-sh, virtual-lua, and container.
- Module/audit/discovery semantics that affect existing invowkfiles or invowkmods.
- Installation, release-channel, docs-versioning, signing, and package-manager changes.
- Security, supply-chain, or trust-boundary changes.
- Bug fixes visible to users, even when commits say `refactor`, `test`, or `chore`.
- Breaking changes, deprecations, warnings, and manual actions.

Use subagents for independent semantic passes when the diff is large or release risk is high. Give them raw ranges and specific read-only questions, not a desired answer.

## Poetic Opening Rules

Allowed authors:

- Arthur Rimbaud
- Oscar Wilde
- Charles Baudelaire
- Edgar Allan Poe
- William Shakespeare
- Florbela Espanca

Rules:

- Use one full stanza/verse only, not an entire poem.
- Never use a stray line, partial stanza, or excerpt that cuts through a stanza. Verify the source poem's stanza boundary before publishing.
- For English originals, include the English stanza and attribution.
- For French or Portuguese originals, include the original-language stanza and an English rendering.
- Prefer public-domain text. For translations, use a public-domain translation or create a brief agent-authored working translation and label it as such.
- Do not repeat a stanza across releases.
- Include enough attribution to search later: author, poem title, and optionally translator/source.
- Add a hidden metadata comment below the stanza when useful:

```markdown
<!-- poetic-opening: Author | Poem Title | first line -->
```

Double-check uniqueness:

1. Run `python3 .agents/skills/publish-release/scripts/check-poetic-opening.py <release-notes-file>`.
2. Search prior local tag bodies:
   `for t in $(git tag --list 'v[0-9]*'); do git cat-file tag "$t" 2>/dev/null | rg -n "Poem Title|first line|Author" && echo "$t"; done`
3. Search GitHub Releases:
   `gh release list --limit 200` and `gh release view <tag> --json body`.
4. If any previous release used the same stanza or substantially the same stanza, choose another.

## Section Guidance

`What's Changed`:

- Group by user-facing theme, not by commit type.
- Mention important internal work only when it affects reliability, speed, security, or maintainability users benefit from.

`Manual Actions Needed`:

- Include config edits, re-running installers, PATH refreshes, lockfile regeneration, module tidy/sync, token updates, package-manager lag, or migration steps.

`Warnings and Deprecations`:

- Include breaking changes, alpha caveats, removed behavior, deprecated fields, unsupported platforms, known package-manager delay, and compatibility limits.

`Bug Fixes`:

- Describe symptoms users might recognize and the resolved behavior.
- Include fixed CI/release-channel problems only if they affect users or maintainers.
