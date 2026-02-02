// SPDX-License-Identifier: MPL-2.0

// Package serverbase provides a reusable state machine and lifecycle infrastructure
// for long-running server components.
//
// This package extracts common patterns from SSH and TUI servers including:
// atomic state reads, mutex-protected transitions, WaitGroup tracking, and
// context-based cancellation.
package serverbase
