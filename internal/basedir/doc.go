// SPDX-License-Identifier: MPL-2.0

// Package basedir resolves every per-user base directory invowk derives from
// the OS environment: the home directory, the config, data, cache and state
// bases, the runtime dir, and the Windows roaming and local AppData bases.
//
// Every directory it returns is absolute, so no directory that invowk trusts
// or writes can resolve under the current directory:
//
//   - A relative or empty XDG_*_HOME, XDG_RUNTIME_DIR, APPDATA or LOCALAPPDATA
//     is treated as unset, as the XDG Base Directory spec requires, and the
//     next base is used. XDG_RUNTIME_DIR has no default, so it is reported as
//     unavailable and the caller uses its own fallback.
//   - A relative home directory or USERPROFILE is an [ErrNotAbsolute] error,
//     and no directory is derived from it.
//
// Why one shared package: the package-design rule normally prefers a copy of
// a helper per domain package over a shared utility. This is a security
// invariant at the OS boundary, not a convenience helper, and copies are the
// drift it removes: before this package, seven call sites each checked the
// home directory and the XDG variables differently or not at all. A guard
// test (guard_test.go) keeps every other non-test file off direct lookups,
// and .agents/rules/package-design.md names this package as the exception.
//
// The package is a leaf: it imports only the standard library, pkg/platform
// and pkg/types, so config, the app services, provisioning, the runtimes, the
// container engine and the test harnesses can all import it without cycles.
package basedir
