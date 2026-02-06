// SPDX-License-Identifier: MPL-2.0

// Package serverbase provides a reusable state machine and lifecycle infrastructure
// for long-running server components.
//
// This package extracts common patterns from SSH and TUI servers including:
// atomic state reads, mutex-protected transitions, WaitGroup tracking, and
// context-based cancellation.
//
// # State Machine Lifecycle
//
// Valid transitions:
//
//	Created → Starting → Running → Stopping → Stopped
//	Created → Starting → Failed
//
// TransitionToStarting enforces the Created precondition; TransitionToRunning
// enforces Starting. Terminal states (Stopped, Failed) are irreversible — create
// a new Base instance to restart.
//
// Atomic state reads (State(), IsRunning()) are lock-free via atomic.Int32.
// State transitions use CAS (CompareAndSwap) or mutex to prevent concurrent
// corruption while keeping the read path fast.
package serverbase
