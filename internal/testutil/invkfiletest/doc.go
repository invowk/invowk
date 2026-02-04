// SPDX-License-Identifier: MPL-2.0

// Package invkfiletest provides test helpers for creating invkfile.Command objects.
//
// This package is separate from testutil to avoid import cycles, since testutil
// is used by pkg/invkmod tests which cannot transitively import pkg/invkfile.
//
// # Usage
//
//	import "invowk-cli/internal/testutil/invkfiletest"
//
//	cmd := invkfiletest.NewTestCommand("hello", invkfiletest.WithScript("echo hello"))
package invkfiletest
