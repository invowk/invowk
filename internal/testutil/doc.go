// SPDX-License-Identifier: MPL-2.0

// Package testutil provides helper functions for tests that handle errors
// appropriately, reducing boilerplate and ensuring consistent error handling.
//
// Common helpers include environment variable management (MustSetenv, MustUnsetenv),
// directory operations (MustChdir, MustMkdirAll), resource cleanup (MustClose,
// MustStop, DeferClose, DeferStop), and container test concurrency limiting
// (ContainerSemaphore).
package testutil
