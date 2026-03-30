// SPDX-License-Identifier: MPL-2.0

// Package cli contains CLI integration tests using testscript.
//
// These tests verify invowk command-line behavior with deterministic
// output capture, replacing the flaky VHS-based tests.
//
// Container tests are separated into TestContainerCLI (cmd_container_test.go)
// and pinned to a single verified engine for deterministic execution. The
// runtime retry logic still protects individual container runs, but the test
// harness no longer treats "any healthy engine" as sufficient.
package cli
