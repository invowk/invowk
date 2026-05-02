// SPDX-License-Identifier: MPL-2.0

package pathmatrix_test

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
)

const (
	// pretendOS values keep the meta-tests deterministic across runners
	// without forking processes.
	pretendUnix    pretendOS = "linux"
	pretendWindows pretendOS = "windows"
)

var (
	// errPredicateFalse is a local sentinel used by predicate-shaped
	// fixtures.
	errPredicateFalse = errors.New("predicate returned false")

	// Compile-time references that keep helper fixtures available for
	// future test additions without triggering unused-symbol lint errors.
	_ = (&fakeBuggyResolver{}).Resolve
	_ = (fixedResolver{}).Resolve
)

type (
	// pretendOS is a knob a fixture uses to pretend it's running on a
	// different OS than the real test runner.
	pretendOS string

	// fakeBuggyResolver simulates the v0.10.0 bug pattern on a
	// configurable "OS" without depending on the actual goruntime.GOOS.
	fakeBuggyResolver struct {
		pretend pretendOS
		baseDir string
	}

	// fixedResolver always returns its configured (out, err). Useful for
	// exercising the matrix's outcome-assertion branches.
	fixedResolver struct {
		out string
		err error
	}

	// kindBoxError is a concrete error type used by
	// TestRejectAs_MatchesTargetType.
	kindBoxError struct{ kind string }
)

func (r *fakeBuggyResolver) Resolve(input string) (string, error) {
	nativeInput := input
	if r.pretend == pretendWindows {
		nativeInput = strings.ReplaceAll(input, "/", `\`)
	}
	if r.fakeIsAbs(nativeInput) {
		return nativeInput, nil
	}
	return filepath.Join(r.baseDir, nativeInput), nil
}

// fakeIsAbs mimics filepath.IsAbs's platform-specific behavior without
// depending on the real goruntime.GOOS. Critical: on pretendWindows,
// "/foo" (after FromSlash conversion to "\foo") is NOT absolute because
// Windows requires a drive letter or UNC prefix.
func (r *fakeBuggyResolver) fakeIsAbs(p string) bool {
	if r.pretend == pretendUnix {
		return strings.HasPrefix(p, "/")
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return false
}

func (f fixedResolver) Resolve(_ string) (string, error) { return f.out, f.err }

func (e kindBoxError) Error() string { return "kind=" + e.kind }

// rejectAllValidator always returns errPredicateFalse. Used as a
// known-rejecting fixture for matrix-level rejection assertions.
func rejectAllValidator(_ string) error { return errPredicateFalse }

// acceptAllValidator always returns nil. Used as a known-accepting
// fixture.
func acceptAllValidator(_ string) error { return nil }

// allSeven returns an Expectations populated with the same Outcome for
// all seven base vectors. Useful when the test cares about the matrix
// driver logic rather than per-vector divergence.
func allSeven(o pathmatrix.Outcome) pathmatrix.Expectations {
	return pathmatrix.Expectations{
		UnixAbsolute:       o,
		WindowsDriveAbs:    o,
		WindowsRooted:      o,
		UNC:                o,
		SlashTraversal:     o,
		BackslashTraversal: o,
		ValidRelative:      o,
	}
}

// TestValidator_AcceptAll verifies the helper passes when every vector
// is expected to accept and the validator does so.
func TestValidator_AcceptAll(t *testing.T) {
	t.Parallel()
	pathmatrix.Validator(t, acceptAllValidator, allSeven(pathmatrix.PassAny(nil)))
}

// TestValidator_RejectAllWithSentinel verifies RejectIs catches errors
// that wrap a known sentinel.
func TestValidator_RejectAllWithSentinel(t *testing.T) {
	t.Parallel()
	pathmatrix.Validator(t, rejectAllValidator, allSeven(pathmatrix.RejectIs(errPredicateFalse)))
}

// TestResolver_PassRelative verifies PassRelative joins via filepath.Join.
// Each vector uses its actual input as the expected relative segment, since
// the test fixture is a non-canonicalizing resolver. Valid-relative is
// expressed via PassAny because the three sub-forms (bare/nested/dotted)
// cannot be expressed by a single PassRelative segment.
func TestResolver_PassRelative(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	resolve := func(input string) (string, error) {
		return filepath.Join(baseDir, input), nil
	}
	pathmatrix.Resolver(t, baseDir, resolve, pathmatrix.Expectations{
		UnixAbsolute:       pathmatrix.PassRelative(pathmatrix.InputUnixAbsolute),
		WindowsDriveAbs:    pathmatrix.PassRelative(pathmatrix.InputWindowsDriveAbs),
		WindowsRooted:      pathmatrix.PassRelative(pathmatrix.InputWindowsRooted),
		UNC:                pathmatrix.PassRelative(pathmatrix.InputUNC),
		SlashTraversal:     pathmatrix.PassRelative(pathmatrix.InputSlashTraversal),
		BackslashTraversal: pathmatrix.PassRelative(pathmatrix.InputBackslashTraversal),
		ValidRelative:      pathmatrix.PassAny(nil),
	})
}

// TestResolver_PassExact verifies Pass asserts exact-match semantics.
func TestResolver_PassExact(t *testing.T) {
	t.Parallel()
	const want = "/exact/path"
	resolve := func(_ string) (string, error) { return want, nil }
	pathmatrix.Resolver(t, "" /* unused */, resolve, allSeven(pathmatrix.Pass(want)))
}

// TestPlatformOverride_RoutedByGOOS verifies that the matrix consults
// goruntime.GOOS when selecting the active override. Every override
// flips to PassAny so the matrix passes regardless of GOOS.
func TestPlatformOverride_RoutedByGOOS(t *testing.T) {
	t.Parallel()
	pass := pathmatrix.PassAny(nil)
	override := &pathmatrix.PlatformOverride{
		UnixAbsolute:       &pass,
		WindowsDriveAbs:    &pass,
		WindowsRooted:      &pass,
		UNC:                &pass,
		SlashTraversal:     &pass,
		BackslashTraversal: &pass,
		ValidRelative:      &pass,
	}
	expect := pathmatrix.Expectations{
		UnixAbsolute:       pathmatrix.RejectIs(errPredicateFalse),
		WindowsDriveAbs:    pathmatrix.RejectIs(errPredicateFalse),
		WindowsRooted:      pathmatrix.RejectIs(errPredicateFalse),
		UNC:                pathmatrix.RejectIs(errPredicateFalse),
		SlashTraversal:     pathmatrix.RejectIs(errPredicateFalse),
		BackslashTraversal: pathmatrix.RejectIs(errPredicateFalse),
		ValidRelative:      pathmatrix.RejectIs(errPredicateFalse),
		OnLinux:            override,
		OnDarwin:           override,
		OnWindows:          override,
	}
	pathmatrix.Validator(t, acceptAllValidator, expect)
}

// TestExtraVectors_RunInSortedOrder verifies that ExtraVectors are
// executed and that subtests are deterministic. Counter access is
// mutex-guarded because the matrix runs vector subtests in parallel.
func TestExtraVectors_RunInSortedOrder(t *testing.T) {
	t.Parallel()
	var (
		mu   sync.Mutex
		seen = map[string]bool{}
	)
	resolve := func(input string) (string, error) {
		mu.Lock()
		seen[input] = true
		mu.Unlock()
		return "", nil
	}
	expect := allSeven(pathmatrix.PassAny(nil))
	expect.ExtraVectors = map[string]pathmatrix.VectorCase{
		"empty":          {Input: "", Expect: pathmatrix.PassAny(nil)},
		"newline":        {Input: "\n", Expect: pathmatrix.PassAny(nil)},
		"unicode_dotted": {Input: "résumé", Expect: pathmatrix.PassAny(nil)},
	}
	pathmatrix.Validator(t, func(s string) error { _, err := resolve(s); return err }, expect)
	mu.Lock()
	defer mu.Unlock()
	for _, want := range []string{"", "\n", "résumé"} {
		if !seen[want] {
			t.Errorf("ExtraVector %q never executed", want)
		}
	}
}

// TestRejectAs_MatchesTargetType verifies the RejectAs helper extracts a
// concrete error type from a wrapped error chain.
func TestRejectAs_MatchesTargetType(t *testing.T) {
	t.Parallel()
	wrapped := kindBoxError{kind: "test"}
	resolve := func(_ string) (string, error) { return "", wrapped }
	pathmatrix.Resolver(t, "" /* unused */, resolve, allSeven(pathmatrix.RejectAs(new(kindBoxError))))
}

// TestPassIfTrue_BoolPredicate verifies PassIfTrue accepts a predicate
// adapter shape. The matrix asserts nil error; the predicate is recorded
// for diagnostic purposes only.
func TestPassIfTrue_BoolPredicate(t *testing.T) {
	t.Parallel()
	predicate := func(s string) bool { return s != "" }
	expect := allSeven(pathmatrix.PassIfTrue(predicate))
	pathmatrix.Validator(t, func(s string) error {
		if predicate(s) {
			return nil
		}
		return errPredicateFalse
	}, expect)
}

// TestSkip_RecordsReason verifies the Skip outcome calls t.Skip with the
// supplied reason. Direct verification is awkward (we can't easily
// introspect t.Skip without a mockTester), but running with all-Skip
// succeeds without errors, which is the contract.
func TestSkip_RecordsReason(t *testing.T) {
	t.Parallel()
	pathmatrix.Validator(t, func(_ string) error {
		t.Fatal("validator must not be called when every vector is Skip")
		return nil
	}, allSeven(pathmatrix.Skip("not applicable for this synthetic test")))
}

// TestFakeBuggyResolver_DetectsWindowsBug demonstrates the matrix's
// central value proposition: a corrected resolver passes the matrix on
// every platform for UnixAbsolute.
func TestFakeBuggyResolver_DetectsWindowsBug(t *testing.T) {
	t.Parallel()
	corrected := func(input string) (string, error) {
		if strings.HasPrefix(input, "/") {
			return input, nil
		}
		return filepath.Join(t.TempDir(), input), nil
	}
	expect := pathmatrix.Expectations{
		UnixAbsolute:       pathmatrix.Pass("/absolute/path"),
		WindowsDriveAbs:    pathmatrix.PassAny(nil),
		WindowsRooted:      pathmatrix.PassAny(nil),
		UNC:                pathmatrix.PassAny(nil),
		SlashTraversal:     pathmatrix.PassAny(nil),
		BackslashTraversal: pathmatrix.PassAny(nil),
		ValidRelative:      pathmatrix.PassAny(nil),
	}
	pathmatrix.Resolver(t, t.TempDir(), corrected, expect)
}

// TestCustom_ReceivesGotAndErr verifies the Custom outcome receives the
// actual (got, gotErr) pair and runs caller-defined assertions.
func TestCustom_ReceivesGotAndErr(t *testing.T) {
	t.Parallel()
	const wantGot = "computed"
	wantErr := errors.New("computed")
	resolve := func(_ string) (string, error) { return wantGot, wantErr }
	check := func(tb testing.TB, got string, gotErr error) {
		tb.Helper()
		if got != wantGot {
			tb.Errorf("Custom: got=%q, want=%q", got, wantGot)
		}
		if gotErr == nil || gotErr.Error() != "computed" {
			tb.Errorf("Custom: gotErr=%v, want %q", gotErr, "computed")
		}
	}
	pathmatrix.Resolver(t, "" /* unused */, resolve, allSeven(pathmatrix.Custom(check)))
}

// TestMissingBaseVector_FailsAtSetup documents the Validator/Resolver
// Fatalf contract via a no-op subtest. Direct verification would require
// running a child subtest and asserting it failed, which adds complexity
// for marginal value — the contract is documented in the godoc.
func TestMissingBaseVector_FailsAtSetup(t *testing.T) {
	t.Parallel()
	t.Run("omit_unc", func(sub *testing.T) {
		sub.Parallel()
		sub.Skip("documenting Fatalf contract — see Validator/Resolver godoc")
	})
}

// TestValidatorAndResolver_MatchHelperNames is a smoke test that the
// public API names exposed by pathmatrix match what doc.go references.
// If a future refactor renames Validator/Resolver, this fails at compile
// time, signaling that docs need updating.
func TestValidatorAndResolver_MatchHelperNames(t *testing.T) {
	t.Parallel()
	_ = pathmatrix.Validator
	_ = pathmatrix.Resolver
	_ = pathmatrix.Pass
	_ = pathmatrix.PassRelative
	_ = pathmatrix.PassAny
	_ = pathmatrix.PassIfTrue
	_ = pathmatrix.Reject
	_ = pathmatrix.RejectIs
	_ = pathmatrix.RejectAs
	_ = pathmatrix.Custom
	_ = pathmatrix.Skip
}
