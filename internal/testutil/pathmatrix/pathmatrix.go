// SPDX-License-Identifier: MPL-2.0

package pathmatrix

import (
	"errors"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sort"
	"testing"
)

// Constants and enum-like values are grouped into a single const block to
// satisfy decorder. Values that depend on type names defined below use
// explicit numeric assignments because iota is not portable across mixed
// const groups.
const (
	// InputUnixAbsolute is a forward-slash absolute path. Treated as
	// container-absolute on every platform.
	InputUnixAbsolute = "/absolute/path"

	// InputWindowsDriveAbs is a Windows drive-letter absolute path with
	// backslashes. filepath.IsAbs returns true on Windows; false on
	// Linux/macOS.
	InputWindowsDriveAbs = `C:\absolute\path`

	// InputWindowsDriveSlash is the same Windows drive-letter path written
	// with forward slashes — Windows accepts both forms, while non-Windows
	// platforms see only a relative path that contains a colon.
	InputWindowsDriveSlash = "C:/absolute/path"

	// InputWindowsRooted is a backslash-rooted Windows path without a drive
	// letter. filepath.IsAbs returns false on every platform.
	InputWindowsRooted = `\absolute\path`

	// InputUNC is a Windows UNC share path.
	InputUNC = `\\server\share`

	// InputSlashTraversal is a path designed to escape its parent via "..".
	InputSlashTraversal = "a/../../escape"

	// InputBackslashTraversal is the backslash form of slash traversal.
	InputBackslashTraversal = `a\..\..\escape`

	// InputValidRelativeBare is a bare relative directory name.
	InputValidRelativeBare = "tools"

	// InputValidRelativeNested is a nested relative path.
	InputValidRelativeNested = "modules/tools"

	// InputValidRelativeDotted is a dot-prefixed relative path. This form
	// has historically tripped resolvers that strip the leading dot too
	// eagerly, so the matrix runs it as a separate sub-subtest.
	InputValidRelativeDotted = "./tools"

	// outcomeKind values. Numeric constants (not iota) so they coexist
	// with string consts in the same block per decorder.
	outcomeUnset        outcomeKind = 0
	outcomePass         outcomeKind = 1
	outcomePassRelative outcomeKind = 2
	outcomePassAny      outcomeKind = 3
	outcomePassIfTrue   outcomeKind = 4
	outcomeReject       outcomeKind = 5
	outcomeRejectIs     outcomeKind = 6
	outcomeRejectAs     outcomeKind = 7
	outcomeCustom       outcomeKind = 8
	outcomeSkip         outcomeKind = 9

	// matrixMode values, distinguishing validator and resolver behaviors.
	validatorMode matrixMode = 0
	resolverMode  matrixMode = 1
)

// Type definitions grouped into a single block to satisfy decorder.
type (
	// outcomeKind is the discriminant for [Outcome]. The zero value
	// (outcomeUnset) is invalid and triggers a Fatalf at matrix setup,
	// catching the "I forgot to set a vector" mistake at first run rather
	// than only on the platform where the missing vector matters.
	outcomeKind uint8

	// matrixMode controls minor behavioral differences between Validator
	// and Resolver — primarily whether PassAny's asserter receives the
	// resolved string (resolverMode) or an empty placeholder (validatorMode).
	matrixMode uint8

	// Outcome is a discriminated union expressing what should happen for
	// one vector. Construct via the helpers (Pass, Reject, etc.); the
	// zero Outcome is invalid and Fatalfs at setup.
	Outcome struct {
		asserter  func(t testing.TB, got string)
		predicate func(string) bool
		sentinel  error
		target    any
		custom    func(t testing.TB, got string, gotErr error)
		exact     string
		relative  string
		skipMsg   string
		kind      outcomeKind
	}

	// VectorCase pairs a custom input string with an Outcome for use in
	// [Expectations.ExtraVectors].
	VectorCase struct {
		Input  string
		Expect Outcome
	}

	// PlatformOverride supersedes the corresponding base-Expectations
	// field when goruntime.GOOS matches the parent override
	// (OnWindows/OnLinux/OnDarwin). A nil pointer field means "inherit
	// from base".
	PlatformOverride struct {
		UnixAbsolute       *Outcome
		WindowsDriveAbs    *Outcome
		WindowsRooted      *Outcome
		UNC                *Outcome
		SlashTraversal     *Outcome
		BackslashTraversal *Outcome
		ValidRelative      *Outcome
	}

	// Expectations holds one Outcome per canonical vector plus optional
	// per-platform overrides and additional surface-specific vectors. All
	// seven base fields must be set; leaving any base vector unset is a
	// programming error and Fatalfs at setup with the names of the
	// missing vectors.
	Expectations struct {
		ExtraVectors       map[string]VectorCase
		OnWindows          *PlatformOverride
		OnLinux            *PlatformOverride
		OnDarwin           *PlatformOverride
		UnixAbsolute       Outcome
		WindowsDriveAbs    Outcome
		WindowsRooted      Outcome
		UNC                Outcome
		SlashTraversal     Outcome
		BackslashTraversal Outcome
		ValidRelative      Outcome
	}

	// vectorEntry is the per-vector unit the matrix driver iterates.
	vectorEntry struct {
		name          string
		input         string
		outcome       Outcome
		validRelative bool
	}
)

// Pass asserts the function returns nil error and the resolved path equals
// `exact`. Use when the path passes through unchanged or the function
// returns a known constant.
func Pass(exact string) Outcome { return Outcome{kind: outcomePass, exact: exact} }

// PassRelative asserts the function returns nil error and the resolved
// path equals filepath.Join(baseDir, segment). The matrix performs the
// join so callers don't need to write "<base>/segment" sentinel strings.
func PassRelative(segment string) Outcome {
	return Outcome{kind: outcomePassRelative, relative: segment}
}

// PassAny asserts the function returns nil error; if `assert` is non-nil
// it is called with the resolved string for additional checks. For
// validators (which return only error), assert is called with got="".
func PassAny(assert func(t testing.TB, got string)) Outcome {
	return Outcome{kind: outcomePassAny, asserter: assert}
}

// PassIfTrue is a convenience for boolean-returning predicates that have
// been adapted to the validator shape via a sentinel-error wrapper. The
// predicate is recorded for diagnostic purposes only — the helper just
// asserts a nil error.
func PassIfTrue(predicate func(string) bool) Outcome {
	return Outcome{kind: outcomePassIfTrue, predicate: predicate}
}

// Reject asserts the function returns a non-nil error. Prefer [RejectIs]
// or [RejectAs] over bare Reject — bare Reject only proves "something
// failed", not "the right thing failed".
func Reject() Outcome { return Outcome{kind: outcomeReject} }

// RejectIs asserts the function returns an error wrapping `sentinel`
// (errors.Is). Strongly preferred over bare [Reject] for codebases with
// typed sentinel errors.
func RejectIs(sentinel error) Outcome {
	return Outcome{kind: outcomeRejectIs, sentinel: sentinel}
}

// RejectAs asserts the returned error matches the given target via
// errors.As. Pass a pointer-to-target-type, e.g.
// `RejectAs(new(*InvalidSubdirectoryPathError))`.
func RejectAs(targetPtr any) Outcome { return Outcome{kind: outcomeRejectAs, target: targetPtr} }

// Custom is the escape hatch. The provided callback runs with the actual
// (got, gotErr) values returned by the function under test. Use sparingly
// — prefer [Pass]/[Reject]/[RejectIs] when they fit.
func Custom(fn func(t testing.TB, got string, gotErr error)) Outcome {
	return Outcome{kind: outcomeCustom, custom: fn}
}

// Skip declares the vector intentionally inapplicable for this surface.
// The per-vector subtest calls t.Skip(reason). Useful for surfaces that
// genuinely have no defined behavior for one of the seven vectors.
func Skip(reason string) Outcome { return Outcome{kind: outcomeSkip, skipMsg: reason} }

// Validator runs the canonical seven-vector matrix against a Validate-
// shaped function. Use this for value-type `Validate() error` methods such
// as SubdirectoryPath.Validate, FilesystemPath.Validate, ContainerfilePath.Validate.
//
// The caller's parent test should call `t.Parallel()` before invoking this
// helper; per-vector subtests call `sub.Parallel()` automatically.
func Validator(t *testing.T, validate func(input string) error, expect Expectations) {
	t.Helper()
	if missing := missingBaseVectors(expect); len(missing) > 0 {
		t.Fatalf("pathmatrix.Validator: missing base-vector expectations: %v", missing)
	}
	if validate == nil {
		t.Fatal("pathmatrix.Validator: validate func is nil")
	}
	resolve := func(input string) (string, error) { return "", validate(input) }
	runMatrix(t, "" /* baseDir */, resolve, expect, validatorMode)
}

// Resolver runs the canonical seven-vector matrix against a Resolve-shaped
// function. The baseDir argument is used by [PassRelative] expectations to
// build the expected joined path via filepath.Join(baseDir, segment).
//
// The caller's parent test should call `t.Parallel()` before invoking this
// helper; per-vector subtests call `sub.Parallel()` automatically.
func Resolver(t *testing.T, baseDir string, resolve func(input string) (string, error), expect Expectations) {
	t.Helper()
	if missing := missingBaseVectors(expect); len(missing) > 0 {
		t.Fatalf("pathmatrix.Resolver: missing base-vector expectations: %v", missing)
	}
	if resolve == nil {
		t.Fatal("pathmatrix.Resolver: resolve func is nil")
	}
	runMatrix(t, baseDir, resolve, expect, resolverMode)
}

// resolveOutcome returns the effective Outcome for a base vector,
// applying the active platform override (if any) and falling back to the
// base value.
func resolveOutcome(base Outcome, override *Outcome) Outcome {
	if override != nil {
		return *override
	}
	return base
}

// activeOverride returns the platform override matching goruntime.GOOS,
// or nil if none is configured for the current platform.
func activeOverride(e Expectations) *PlatformOverride {
	switch goruntime.GOOS {
	case "windows":
		return e.OnWindows
	case "linux":
		return e.OnLinux
	case "darwin":
		return e.OnDarwin
	}
	return nil
}

// canonicalVectors enumerates the seven base vectors in stable iteration
// order. Used by both Validator and Resolver to drive subtests.
func canonicalVectors(e Expectations) []vectorEntry {
	override := activeOverride(e)
	pick := func(base Outcome, getOverride func(*PlatformOverride) *Outcome) Outcome {
		if override == nil {
			return base
		}
		return resolveOutcome(base, getOverride(override))
	}
	return []vectorEntry{
		{
			name:    "unix_absolute",
			input:   InputUnixAbsolute,
			outcome: pick(e.UnixAbsolute, func(p *PlatformOverride) *Outcome { return p.UnixAbsolute }),
		},
		{
			name:    "windows_drive_abs",
			input:   InputWindowsDriveAbs,
			outcome: pick(e.WindowsDriveAbs, func(p *PlatformOverride) *Outcome { return p.WindowsDriveAbs }),
		},
		{
			name:    "windows_rooted",
			input:   InputWindowsRooted,
			outcome: pick(e.WindowsRooted, func(p *PlatformOverride) *Outcome { return p.WindowsRooted }),
		},
		{
			name:    "unc",
			input:   InputUNC,
			outcome: pick(e.UNC, func(p *PlatformOverride) *Outcome { return p.UNC }),
		},
		{
			name:    "slash_traversal",
			input:   InputSlashTraversal,
			outcome: pick(e.SlashTraversal, func(p *PlatformOverride) *Outcome { return p.SlashTraversal }),
		},
		{
			name:    "backslash_traversal",
			input:   InputBackslashTraversal,
			outcome: pick(e.BackslashTraversal, func(p *PlatformOverride) *Outcome { return p.BackslashTraversal }),
		},
		{
			name:          "valid_relative",
			outcome:       pick(e.ValidRelative, func(p *PlatformOverride) *Outcome { return p.ValidRelative }),
			validRelative: true,
		},
	}
}

// missingBaseVectors returns the names of base-Expectations fields that
// are the zero Outcome. Used to fail-fast at matrix setup with a clear
// list.
func missingBaseVectors(e Expectations) []string {
	checks := []struct {
		name string
		kind outcomeKind
	}{
		{"UnixAbsolute", e.UnixAbsolute.kind},
		{"WindowsDriveAbs", e.WindowsDriveAbs.kind},
		{"WindowsRooted", e.WindowsRooted.kind},
		{"UNC", e.UNC.kind},
		{"SlashTraversal", e.SlashTraversal.kind},
		{"BackslashTraversal", e.BackslashTraversal.kind},
		{"ValidRelative", e.ValidRelative.kind},
	}
	var missing []string
	for _, c := range checks {
		if c.kind == outcomeUnset {
			missing = append(missing, c.name)
		}
	}
	return missing
}

// runMatrix is the shared driver for Validator and Resolver. It iterates
// the seven canonical vectors plus any ExtraVectors, executing each as a
// `t.Run("matrix/<vector>", ...)` subtest. Mode controls minor behavioral
// differences between the two helpers (e.g., Validator's PassAny asserter
// receives "" because validators return only error).
func runMatrix(t *testing.T, baseDir string, resolve func(input string) (string, error), expect Expectations, mode matrixMode) {
	t.Helper()
	t.Run("matrix", func(matrix *testing.T) {
		matrix.Helper()
		vectors := canonicalVectors(expect)
		for i := range vectors {
			vec := vectors[i]
			matrix.Run(vec.name, func(sub *testing.T) {
				sub.Parallel()
				if vec.validRelative {
					runValidRelative(sub, baseDir, resolve, vec.outcome, mode)
					return
				}
				runOneVector(sub, baseDir, resolve, vec.input, vec.outcome, mode)
			})
		}
		if len(expect.ExtraVectors) == 0 {
			return
		}
		extraNames := make([]string, 0, len(expect.ExtraVectors))
		for name := range expect.ExtraVectors {
			extraNames = append(extraNames, name)
		}
		sort.Strings(extraNames)
		for _, name := range extraNames {
			extra := expect.ExtraVectors[name]
			matrix.Run("extra/"+name, func(sub *testing.T) {
				sub.Parallel()
				runOneVector(sub, baseDir, resolve, extra.Input, extra.Expect, mode)
			})
		}
	})
}

// runValidRelative runs the three valid-relative sub-subtests against one
// outcome. Splitting bare/nested/dotted catches resolvers that mishandle
// the dot-prefix form even though the bare form passes.
func runValidRelative(sub *testing.T, baseDir string, resolve func(input string) (string, error), outcome Outcome, mode matrixMode) {
	sub.Helper()
	subForms := []struct {
		name  string
		input string
	}{
		{"bare", InputValidRelativeBare},
		{"nested", InputValidRelativeNested},
		{"dotted", InputValidRelativeDotted},
	}
	for i := range subForms {
		form := subForms[i]
		sub.Run(form.name, func(s *testing.T) {
			s.Parallel()
			runOneVector(s, baseDir, resolve, form.input, outcome, mode)
		})
	}
}

// runOneVector executes one input against the resolver and asserts the
// outcome. All assertion logic lives here so the matrix driver remains
// straightforward.
func runOneVector(t *testing.T, baseDir string, resolve func(input string) (string, error), input string, outcome Outcome, mode matrixMode) {
	t.Helper()
	switch outcome.kind {
	case outcomeUnset:
		t.Fatalf("pathmatrix: vector outcome unset for input %q", input)
	case outcomeSkip:
		t.Skip(outcome.skipMsg)
	case outcomePass, outcomePassRelative, outcomePassAny, outcomePassIfTrue,
		outcomeReject, outcomeRejectIs, outcomeRejectAs, outcomeCustom:
		// Handled below after invoking resolve.
	}
	got, gotErr := resolve(input)
	switch outcome.kind {
	case outcomeUnset, outcomeSkip:
		// Already handled above; defensive — should not reach here.
	case outcomePass:
		assertPassExact(t, input, got, gotErr, outcome.exact)
	case outcomePassRelative:
		assertPassRelative(t, input, got, gotErr, baseDir, outcome.relative)
	case outcomePassAny:
		assertPassAny(t, input, got, gotErr, outcome.asserter, mode)
	case outcomePassIfTrue:
		assertPassIfTrue(t, input, got, gotErr)
	case outcomeReject:
		assertReject(t, input, got, gotErr)
	case outcomeRejectIs:
		assertRejectIs(t, input, got, gotErr, outcome.sentinel)
	case outcomeRejectAs:
		assertRejectAs(t, input, got, gotErr, outcome.target)
	case outcomeCustom:
		outcome.custom(t, got, gotErr)
	}
}

func assertPassExact(t *testing.T, input, got string, gotErr error, want string) {
	t.Helper()
	if gotErr != nil {
		t.Fatalf("input=%q: expected nil error, got %v", input, gotErr)
	}
	if got != want {
		t.Errorf("input=%q: got %q, want exact %q", input, got, want)
	}
}

func assertPassRelative(t *testing.T, input, got string, gotErr error, baseDir, segment string) {
	t.Helper()
	if gotErr != nil {
		t.Fatalf("input=%q: expected nil error, got %v", input, gotErr)
	}
	want := filepath.Join(baseDir, segment)
	if got != want {
		t.Errorf("input=%q: got %q, want filepath.Join(%q, %q) = %q",
			input, got, baseDir, segment, want)
	}
}

func assertPassAny(t *testing.T, input, got string, gotErr error, asserter func(testing.TB, string), mode matrixMode) {
	t.Helper()
	if gotErr != nil {
		t.Fatalf("input=%q: expected nil error, got %v", input, gotErr)
	}
	if asserter == nil {
		return
	}
	gotForAssert := got
	if mode == validatorMode {
		gotForAssert = ""
	}
	asserter(t, gotForAssert)
}

func assertPassIfTrue(t *testing.T, input, _ string, gotErr error) {
	t.Helper()
	if gotErr != nil {
		t.Fatalf("input=%q: expected predicate-true (nil error), got %v", input, gotErr)
	}
}

func assertReject(t *testing.T, input, got string, gotErr error) {
	t.Helper()
	if gotErr == nil {
		t.Errorf("input=%q: expected non-nil error, got nil (resolved=%q)", input, got)
	}
}

func assertRejectIs(t *testing.T, input, got string, gotErr, sentinel error) {
	t.Helper()
	if gotErr == nil {
		t.Errorf("input=%q: expected error wrapping %v, got nil (resolved=%q)", input, sentinel, got)
		return
	}
	if !errors.Is(gotErr, sentinel) {
		t.Errorf("input=%q: error %v does not wrap sentinel %v", input, gotErr, sentinel)
	}
}

func assertRejectAs(t *testing.T, input, got string, gotErr error, target any) {
	t.Helper()
	if gotErr == nil {
		t.Errorf("input=%q: expected error matching target %T, got nil (resolved=%q)", input, target, got)
		return
	}
	if !errorsAs(gotErr, target) {
		t.Errorf("input=%q: error %v does not match target %T", input, gotErr, target)
	}
}

// errorsAs is a thin wrapper around errors.As that uses reflect to
// satisfy the "must be a non-nil pointer" precondition without forcing
// callers to write `var target *MyErr; errors.As(err, &target)` shapes
// inline. Allocates a fresh target of the same type on every call so
// parallel subtests sharing the same Outcome don't race on the prototype.
//
// Uses reflect-based dispatch because the target type is only known at
// runtime; errors.AsType[T] (Go 1.26+) requires a compile-time type
// parameter and is not applicable here.
func errorsAs(err error, targetPtr any) bool {
	if targetPtr == nil {
		return false
	}
	rv := reflect.ValueOf(targetPtr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return false
	}
	fresh := reflect.New(rv.Type().Elem()).Interface()
	return errors.As(err, fresh) //nolint:forbidigo // generic target type forces classic errors.As
}
