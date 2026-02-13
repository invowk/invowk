// SPDX-License-Identifier: MPL-2.0

// Package invowkfiletest provides test helpers for creating invowkfile.Command objects.
//
// This package is separate from testutil to avoid import cycles, since testutil
// is used by pkg/invowkmod tests which cannot transitively import pkg/invowkfile.
//
// # Usage
//
//	import "invowk-cli/internal/testutil/invowkfiletest"
//
//	cmd := invowkfiletest.NewTestCommand("hello", invowkfiletest.WithScript("echo hello"))
package invowkfiletest
