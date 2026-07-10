# Intentional Simplifications Registry

Items listed here are DELIBERATELY simplified or incomplete in documentation.
Do not emit findings for the registered omission itself. Continue reviewing the rest of the
check; mark the whole item SKIP only when its check ID is explicitly listed in the final column.

## Registry

| ID | Location | What Is Simplified | Rationale | Whole-Check SKIP IDs |
|---|---|---|---|---|
| IS-001 | `website/src/pages/index.tsx` terminal demo | Simplified CLI output, minimal invowkfile | Mobile-friendly marketing page; full accuracy would clutter the hero section | — |
| IS-002 | `README.md` Quick Start heading | Minimal invowkfile without optional fields | First-time user experience; progressive disclosure | — |
| IS-003 | `website/docs/getting-started/quickstart.mdx` | Omits advanced features (modules, containers, deps) | First-time user onboarding; complexity introduced in later sections | — |
| IS-004 | `website/src/components/Snippet/data/getting-started.ts` | Minimal CUE examples | Matches quickstart progressive disclosure | — |
| IS-005 | `website/docs/core-concepts/*.mdx` | One feature per example | Clarity over completeness; each page focuses on one concept | — |
| IS-006 | `docs/architecture/*.md` | May lag behind minor internal refactors | Architecture docs cover major patterns and relationships, not every internal change | — |
| IS-007 | `website/docs/architecture/*.mdx` | Website architecture pages may summarize | Readable overview; D2 diagrams carry the detail | — |
| IS-008 | `README.md` Invowkfile Format heading | Omits `category` command field | Progressive disclosure; optional cosmetic field documented in website schema reference | — |

## Progressive Disclosure Rule

Docs follow progressive disclosure: each page teaches one concept at a time. Judge examples
against the page's educational purpose, not against the full schema. Getting-started examples
omit optional fields; core-concepts examples show one feature per example; the README Quick
Start is a minimal "hello world."

## When IS an Omission a Real Error?

An omission becomes a real finding when:

1. **The example would fail validation** — e.g., a full `cmds` entry with an `implementations`
   block but no `platforms` field would fail CUE validation. Even in a focused example, structural
   validity matters.
2. **The text claims completeness** — e.g., "here is a complete invowkfile" followed by a
   snippet missing required fields.
3. **The omitted feature contradicts the page topic** — e.g., a page about platform compatibility
   that doesn't mention the `platforms` field.
4. **A field name, type, or constraint is wrong** — simplification is about omitting optional
   content, not showing incorrect syntax.

## How to Add Entries

When a review finding is determined to be intentional:

1. Allocate the next stable `IS-NNN` ID; never reuse retired IDs.
2. Add a row to the Registry table above with the exact file path or narrow glob.
3. Describe what is simplified and why it is intentional.
4. Leave Whole-Check SKIP IDs empty unless the simplification exempts the entire check across all
   targets. Narrow omissions remain annotations while the check resolves PASS, FAIL, or BLOCKED.
5. Only when a whole-check ID is explicitly listed may the result mark that check SKIP.
