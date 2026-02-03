// SPDX-License-Identifier: MPL-2.0

// Package tuiserver provides an HTTP server for TUI rendering requests from child processes.
//
// When commands run in containers or subprocesses, they can request TUI components
// (choose, confirm, input) via HTTP. The server forwards requests to the parent
// Bubble Tea program for rendering as overlays.
package tuiserver
