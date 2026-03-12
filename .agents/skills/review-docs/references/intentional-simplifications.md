# Intentional Simplifications Registry

Items listed here are DELIBERATELY simplified or incomplete in documentation.
Do NOT flag these as errors during review. Mark findings against these as severity **SKIP**.

## Registry

| Location | What Is Simplified | Rationale |
|---|---|---|
| `website/src/pages/index.tsx` terminal demo | Simplified CLI output, minimal invowkfile | Mobile-friendly marketing page; full accuracy would clutter the hero section |
| `README.md` Quick Start (L151) | Minimal invowkfile without optional fields | First-time user experience; progressive disclosure |
| `website/docs/getting-started/quickstart.mdx` | Omits advanced features (modules, containers, deps) | First-time user onboarding; complexity introduced in later sections |
| `website/src/components/Snippet/data/getting-started.ts` | Minimal CUE examples | Matches quickstart progressive disclosure |
| `website/docs/core-concepts/*.mdx` | One feature per example | Clarity over completeness; each page focuses on one concept |
| `docs/architecture/*.md` | May lag behind minor internal refactors | Architecture docs cover major patterns and relationships, not every internal change |
| `website/docs/architecture/*.mdx` | Website architecture pages may summarize | Readable overview; D2 diagrams carry the detail |

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

1. Add a row to the Registry table above with the exact file path.
2. Describe what is simplified (be specific).
3. Explain why it is intentional (the pedagogical or UX reason).
4. Mark the original finding as severity **SKIP** with a reference to this entry.
