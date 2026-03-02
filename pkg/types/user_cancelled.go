// SPDX-License-Identifier: MPL-2.0

package types

import "errors"

// ErrUserCancelled is returned when a user cancels an interactive operation
// (e.g., via Ctrl+C or Esc in a TUI component). It lives in pkg/types so that
// both internal/tui and internal/tuiserver can share the same sentinel without
// circular imports (tui imports tuiserver).
//
// Callers can check for this error using errors.Is(err, types.ErrUserCancelled).
var ErrUserCancelled = errors.New("user cancelled")
