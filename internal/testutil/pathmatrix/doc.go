// SPDX-License-Identifier: MPL-2.0

// Package pathmatrix provides table-driven test helpers that exercise path
// validators and resolvers against the seven canonical cross-platform input
// vectors documented in `.agents/rules/testing.md` ("Cross-Platform Path
// Validator Matrix").
//
// The package exposes two helpers:
//
//   - [Validator] runs the matrix against a `func(string) error` shape.
//   - [Resolver]  runs the matrix against a `func(string) (string, error)` shape.
//
// Both helpers iterate the seven base vectors (UnixAbsolute, WindowsDriveAbs,
// WindowsRooted, UNC, SlashTraversal, BackslashTraversal, ValidRelative) and
// any caller-supplied [Expectations.ExtraVectors]. Each vector runs as a
// `t.Run("matrix/<vector>", ...)` subtest in source-stable order. Per-platform
// overrides ([PlatformOverride]) let a single matrix express divergence
// without an out-of-helper `runtime.GOOS` conditional.
//
// # Why this package exists
//
// Before pathmatrix landed, every path-validator test re-built its own
// 4–7-vector table inline. The drift was material: some tests omitted UNC,
// others omitted backslash traversal, the canonical `TestSubdirectoryPath_Validate`
// covered all 7 but `TestFilesystemPath_Validate` covered only 6. Worse, the
// `skipOnWindows` field pattern silently suppressed the exact vector — Unix
// absolute on Windows — that the validator had to handle correctly.
// pathmatrix codifies the contract: every Validator/Resolver test states what
// each of the 7 vectors is expected to do, and the helper proves it on
// every platform CI runs.
//
// # Use Validator for value-type Validate() methods
//
//	pathmatrix.Validator(t, func(s string) error {
//	    return SubdirectoryPath(s).Validate()
//	}, pathmatrix.Expectations{
//	    UnixAbsolute:       pathmatrix.RejectIs(ErrInvalidSubdirectoryPath),
//	    WindowsDriveAbs:    pathmatrix.RejectIs(ErrInvalidSubdirectoryPath),
//	    WindowsRooted:      pathmatrix.RejectIs(ErrInvalidSubdirectoryPath),
//	    UNC:                pathmatrix.RejectIs(ErrInvalidSubdirectoryPath),
//	    SlashTraversal:     pathmatrix.RejectIs(ErrInvalidSubdirectoryPath),
//	    BackslashTraversal: pathmatrix.RejectIs(ErrInvalidSubdirectoryPath),
//	    ValidRelative:      pathmatrix.PassAny(nil),
//	})
//
// # Use Resolver for higher-level path resolvers
//
//	pathmatrix.Resolver(t, tmpDir, func(input string) (string, error) {
//	    return string(module.ResolveScriptPath(types.FilesystemPath(input))), nil
//	}, pathmatrix.Expectations{
//	    UnixAbsolute:       pathmatrix.Pass("/absolute/path"),       // pass-through
//	    WindowsDriveAbs:    pathmatrix.PassRelative("C:\\absolute\\path"), // joined on Linux
//	    OnWindows: &pathmatrix.PlatformOverride{
//	        WindowsDriveAbs: ptr(pathmatrix.Pass(`C:\absolute\path`)),
//	    },
//	    WindowsRooted:      pathmatrix.PassRelative(`\absolute\path`),
//	    UNC:                pathmatrix.PassRelative(`\\server\share`),
//	    SlashTraversal:     pathmatrix.PassRelative("a/../../escape"),
//	    BackslashTraversal: pathmatrix.PassRelative(`a\..\..\escape`),
//	    ValidRelative:      pathmatrix.PassRelative("tools"),
//	})
//
// # Cross-platform expectation expressions
//
//   - [Pass]              — exact pass-through string (the function returns the input unchanged or a known constant).
//   - [PassRelative]      — the function joins the input with the test's baseDir using filepath.Join.
//   - [PassHostNativeAbs] — like Pass when filepath.IsAbs(input) is true on the running platform; like PassRelative otherwise. Use for resolvers that follow the host filepath package's absoluteness contract — the right outcome for platform-divergent inputs (UNC, Windows-drive, Windows-rooted).
//   - [PassAny]           — accept any non-error result; optional inspector callback.
//   - [PassIfTrue]        — for boolean-returning predicates wrapped via `func(s) error { if pred(s) { return nil }; return errFalse }`.
//   - [Reject]            — assert the function returns a non-nil error (use sparingly; prefer [RejectIs] / [RejectAs]).
//   - [RejectIs]          — assert the returned error wraps a specific sentinel via errors.Is.
//   - [RejectAs]          — assert the returned error matches a target type via errors.As.
//   - [Custom]             — caller-defined assertion against (got, gotErr).
//   - [Skip]              — declare the vector intentionally inapplicable; recorded via t.Skip in the per-vector subtest.
//
// # Choosing between PassRelative and PassHostNativeAbs
//
// Most resolvers under test follow the contract: "if filepath.IsAbs(input)
// is true, return input unchanged; else, return filepath.Join(baseDir, input)".
// For these, [PassHostNativeAbs] is the right outcome: a single declaration
// produces correct expectations on every platform without per-platform
// overrides.
//
// [PassRelative] encodes "joined to baseDir on every platform". This is
// only correct when the resolver explicitly normalizes the input — for
// example, a validator that rejects every absolute form regardless of
// platform. For inputs whose absoluteness depends on filepath.IsAbs (UNC
// "\\server\share", Windows drive "C:\foo", Windows-rooted "\foo"),
// [PassRelative] without an [Expectations.OnWindows] override will fail
// on Windows where filepath.IsAbs returns true and the resolver passes
// through. Use [PassHostNativeAbs] instead. The cross-platform-paths
// goplint analyzer flags PassRelative on these three input constants
// (InputUNC, InputWindowsDriveAbs, InputWindowsRooted) at commit time.
//
// # Subtest naming
//
// Subtests run as `matrix/<vector>` so per-vector failures are individually
// addressable in CI logs. The `valid_relative` vector runs three sub-subtests
// (`valid_relative/bare`, `valid_relative/nested`, `valid_relative/dotted`)
// because the dot-prefix form has historically tripped resolvers in ways the
// bare form did not.
//
// # Helper takes *testing.T, not testing.TB
//
// `t.Run` requires `*testing.T`, so the matrix helpers are not callable from
// benchmarks. Validator and resolver correctness is tested on the standard
// `*testing.T` path; perf benchmarks are out of scope.
//
// # Parallelism
//
// The helper itself never calls `t.Parallel()`. Its caller (the parent test
// function) calls `t.Parallel()`, and the helper calls `sub.Parallel()` inside
// each `t.Run` subtest. This satisfies the `tparallel` linter (see
// `.agents/rules/testing.md` § Test Parallelism).
package pathmatrix
