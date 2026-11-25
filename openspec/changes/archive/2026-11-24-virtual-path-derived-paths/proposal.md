## Why

Under `restricted` access the virtual runtimes check every operand of a validated u-root built-in before it runs (`urootCommandOperands`, `internal/uroot/registry_operands.go:139`), and `gzip`'s output path as well (`checkGzipOutputPaths`, `:380`). `cp` and `tar -x` also use paths that they build from a checked operand: `cp` joins the destination directory with each source's base name and, with `-r`, with every relative path below the source; `tar -x` joins the extraction directory with every entry name in the archive. This change hardens the harness so that, under restricted access, every path `cp` and `tar -x` create, write or read, including the destinations they derive from their operands, is checked against the allowed roots before it is used.

## What Changes

- Under `restricted` access, before `cp` runs, Invowk computes every path it will create or write: the destination directory joined with each source's base name, and with `-r` every destination path from a walk of the source tree that does not follow links. It checks each one with the full (following) resolution and the allowed-root check. Without `-P` it also checks the followed target of every link in the source tree. Any denial refuses the whole command before anything is written, with `virtual path denied`.
- Under `restricted` access, before `tar -x` runs, Invowk reads the archive once and computes every entry's destination with u-root's own join and skip rules, then checks each one the same way. Any denial refuses the whole command before anything is extracted.
- To keep those predictions exact, under `restricted` access only: `cp` refuses a source operand after the first that does not exist when the command starts (an earlier source could create it; a missing first source keeps u-root's own error), `cp -r` refuses to write into a source tree of the same command (a directory copied into itself, or an earlier source copied into a later one), `cp -P` refuses a command that would create a link and also has any other destination, and `tar -x` refuses an archive that is not a regular file, because the archive is read twice. The source-tree rule compares file identity, never the spelling of a path, and the link rule compares no names, so both hold on case-insensitive volumes too. Every refusal carries `virtual path denied`.
- `cp -P` does not follow or check a source link. Under `restricted` access it creates a link only when that link is the command's only destination, so `cp -P -r` of a tree that holds a link is refused; a link named as an operand is resolved, as before, so `cp -P lnk d` copies its target.
- `full` access is unchanged: none of these checks run.
- virtual-lua reaches `cp` and `tar` through the same registry (`invowk.cmd`, `invowk.capture`) and gets the same checks.
- README, the website's virtual-sh and virtual-lua pages (en and pt-BR) and the `uroot` skill describe the checks. The virtual runtimes remain a harness, not a sandbox.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `virtual-runtime-sandbox`: adds requirements for the paths `cp` and `tar -x` derive from their operands, checked before use under restricted access.

## Impact

- Code: `internal/uroot` (the operand collector gains derived-path checks for `cp` and `tar`, and `HandlerContext` a `FullAccess` flag whose zero value checks); `internal/runtime/sh.go` and `internal/runtime/lua.go` set that flag under full access. No new module requirement. No formal-bound symbol changes.
- Behavior: under `restricted` access, `cp` and `tar -x` refuse commands whose derived paths resolve outside every allowed root, and the four prediction refusals above. Links and junctions whose targets stay inside the allowed roots keep working. `full` access is unchanged.
- Tests: unit tests in `internal/uroot`, runtime and virtual-lua tests in `internal/runtime`, a differential test of the predicted paths against what upstream touches, Windows junction variants, and a CLI testscript.
- Docs: README, `website/docs/runtime-modes/virtual.mdx`, `virtual-lua.mdx` and their pt-BR translations, `.agents/skills/uroot/SKILL.md`.
- Not **BREAKING** for scripts that keep their paths inside the allowed roots.
