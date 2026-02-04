// SPDX-License-Identifier: MPL-2.0

// Package testutil provides helper functions for tests that handle errors
// appropriately, reducing boilerplate and ensuring consistent error handling.
//
// Common helpers include environment variable management (MustSetenv, MustUnsetenv),
// directory operations (MustChdir, MustMkdirAll), and resource cleanup (MustClose,
// MustStop, DeferClose, DeferStop).
package testutil
