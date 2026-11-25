# Proposal

## Why

Eight follow-ups from PRs #207, #213, #214 and #215, the formal finding F11 and the tracker (N27, decisions 98a, 99, 101e, 120, 125 and 132) touch the TUI, the two servers, provisioning and one dependency. Each one is small, and two of them are user-visible bugs: a drawn `invowk tui table` shows no data rows, and `Server.Stop` leaves authenticated SSH connections open. The maintainer grouped them into one change (group W2, decision 124) because they share no files with the parallel module work.

## What Changes

- **`invowk tui table` draws its data rows.** A table drawn on the terminal showed only the header and the rule, with or without `--selectable`: the rows were rendered into a view zero columns wide. The drawn table now shows every visible row (decision 98a).
- **Plain-text table cells.** Besides control characters, format characters (Unicode category Cf: bidi overrides, zero-width characters, soft hyphen), the line separator U+2028 and the paragraph separator U+2029 are printed as a space; invalid UTF-8 is printed as U+FFFD, which is now documented. A testscript covers a quoted CSV field with a newline end to end. The constant `errTUISpinNotRunMsg` is renamed `tuiSpinNotRunErrMsg` (decisions 99 and 125.4).
- **A second `Start` never fails a server it does not own.** `Start` with an already-cancelled context on a server that is not in Created returns an error and changes nothing; until now it marked a Starting or Running server Failed and cancelled it, and on a stopped server it returned `nil`. This covers the SSH host-callback server and the TUI server. The "outside the contract" wording that #213 added for this case goes (decision 120).
- **`Server.Stop` closes every SSH connection (finding F11, user-approved in decision 125.2).** Stop now closes the connections the server accepted, authenticated or not, and returns without waiting for the shutdown timeout. It no longer uses the SSH library's graceful shutdown, which closed no connection and whose connection wait races a handshake that completes during Stop (gliderlabs/ssh#117): that upstream residual no longer applies to invowk.
- **`TestWatcherDebounce` no longer flakes on Windows.** The real-filesystem test stops asserting exactly one callback, which a late Windows change notification breaks; the exact coalescing contract moves to a deterministic test on the fake backend (decision 101e).
- **`github.com/coder/acp-go-sdk` 0.13.5 -> 0.20.0, pinned exactly (tracker N27).** `internal/acpclient` is migrated to the 0.20 API (patch fields, 64-bit integers, the elicitation methods of the client interface). Elicitation is not offered: the client keeps it out of its capabilities and answers its requests with method-not-found. No user-facing behaviour changes; `internal/acpclient` has no production caller.
- **The provisioned invowk binary is the one that was checked (follow-up of #214).** The image build copies `binary_path` into the build context once and checks and hashes that copy. A binary replaced while invowk provisions is never installed unchecked: the build fails instead.
- **A command that never reads its input is hung up when the viewer quits early (follow-up of #215).** On Unix, the viewer's pty master is written through a non-blocking descriptor, so closing the pty no longer waits for a write the command never reads, and SIGHUP reaches the command when the viewer ends rather than when invowk exits. This item is conditional: if the investigation (design D8) shows that it needs a redesign of the `xpty` package, the item changes no code and is recorded in the result instead.

No change is breaking. Two observable behaviours change on purpose: `Server.Stop` now ends SSH sessions that are still open (an internal API; production calls `Stop` after every execution has ended), and plain-text table cells print format characters as spaces.

## Capabilities

### New Capabilities

- `server-lifecycle-base`: the lifecycle contract that the shared server base gives every long-running server (the SSH host-callback server and the TUI server): which `Start` calls may change a server's state.

### Modified Capabilities

- `tui-component-terminal-io`: the plain-text table format replaces format and separator characters and documents U+FFFD; a new requirement that a drawn table shows its data rows.
- `host-callback-server-lifecycle`: Stop closes every connection and does not use the library's graceful shutdown; the "started at most once" exclusion goes.
- `agent-client-protocol-foundation`: the ACP client declines elicitation explicitly.
- `container-auto-provision`: the installed binary is the copy that was checked and hashed.
- `interactive-mode-terminal`: leaving the viewer early hangs up a command that does not read its input (conditional, design D8).

## Impact

- Code: `internal/tui/table.go`, `internal/tui/table_text.go`, `cmd/invowk/tui_spin.go`; `internal/core/serverbase/base.go`; `internal/sshserver/server_lifecycle.go`, `server_conns.go`, `server.go`; `internal/watch/watcher_test.go`; `internal/acpclient/`; `internal/provision/` (binary check, hash and build-context copy); `internal/app/commandadapters/interactive_session*.go`.
- Formal (all bound by `.agents/rules/formal-bindings.md`): `formal/tla/Serverbase.tla` and `formal/tla/HostCallbackToken.tla` with their manifest commands, correspondence rows, Go replays and trace harnesses; F11 row in `formal/README.md`; a targeted formal-bindings ledger refresh after one report-only `make mutation-formal` run.
- Dependencies: `go.mod`/`go.sum` (`github.com/coder/acp-go-sdk v0.20.0`, Apache-2.0, no new transitive module).
- Docs: `website/docs/tui/table-and-spin.mdx` and its pt-BR copy; `.agents/skills/server/SKILL.md`; the Purpose of `openspec/specs/host-callback-server-lifecycle/spec.md`.
- Tests: tmux test for a drawn table, a `tui_no_terminal.txtar` case for a quoted CSV newline, serverbase/sshserver/tuiserver double-Start tests, the inverted F11 replay, a Stop-during-handshake race test, provisioning replacement tests that fail on the base, a Unix non-reading `-i` hang-up test.
