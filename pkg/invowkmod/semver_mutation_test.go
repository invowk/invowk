// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"strings"
	"testing"
)

func TestSemverMutationResolverContracts(t *testing.T) {
	t.Parallel()

	resolver := NewSemverResolver()
	if resolver == nil {
		t.Fatal("NewSemverResolver() = nil, want resolver instance")
	}

	constraint, err := resolver.ParseConstraint("  ^v1.2.3-alpha  ")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v, want nil", err)
	}
	if constraint.Op != ConstraintOpCaret {
		t.Fatalf("Constraint.Op = %q, want %q", constraint.Op, ConstraintOpCaret)
	}
	if constraint.Version == nil {
		t.Fatal("Constraint.Version = nil, want parsed version")
	}
	if constraint.Version.Major != 1 || constraint.Version.Minor != 2 ||
		constraint.Version.Patch != 3 || constraint.Version.Prerelease != "alpha" {
		t.Fatalf("Constraint.Version = %+v, want 1.2.3-alpha", constraint.Version)
	}
	if constraint.Original != "^v1.2.3-alpha" {
		t.Fatalf("Constraint.Original = %q, want trimmed original", constraint.Original)
	}
}

func TestSemverMutationParseOverflowErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "major overflow",
			in:   "999999999999999999999999999999999999.1.2",
			want: "invalid major version",
		},
		{
			name: "minor overflow",
			in:   "1.999999999999999999999999999999999999.2",
			want: "invalid minor version",
		},
		{
			name: "patch overflow",
			in:   "1.2.999999999999999999999999999999999999",
			want: "invalid patch version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseVersion(tt.in)
			if err == nil {
				t.Fatalf("ParseVersion(%q) error = nil, want %q", tt.in, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseVersion(%q) error = %q, want %q", tt.in, err.Error(), tt.want)
			}
		})
	}
}

func TestSemverMutationResolveErrorBoundaries(t *testing.T) {
	t.Parallel()

	resolver := NewSemverResolver()

	_, err := resolver.Resolve("^1.0.0", []SemVer{"bad", "also-bad"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want no-valid-versions error")
	}
	if got := err.Error(); got != "no valid versions available" {
		t.Fatalf("Resolve() error = %q, want no-valid-versions message", got)
	}

	available := []SemVer{"1.0.0", "2.0.0"}
	_, err = resolver.Resolve("^3.0.0", available)
	if err == nil {
		t.Fatal("Resolve() error = nil, want no-match error")
	}
	if !strings.Contains(err.Error(), `no version matches constraint "^3.0.0"`) ||
		!strings.Contains(err.Error(), "[1.0.0 2.0.0]") {
		t.Fatalf("Resolve() error = %q, want constraint and available versions", err.Error())
	}
}

func TestSemverMutationCollectionContracts(t *testing.T) {
	t.Parallel()

	sorted := SortVersions([]SemVer{"1.0.0", "v2.0.0", "bad", "2.0.0-alpha"})
	requireSemverMutationSlice(t, sorted, []SemVer{"v2.0.0", "2.0.0-alpha", "1.0.0"})

	filtered, err := FilterVersions("^0.0.3", []SemVer{"0.0.2", "0.0.3", "0.0.4", "bad"})
	if err != nil {
		t.Fatalf("FilterVersions() error = %v, want nil", err)
	}
	requireSemverMutationSlice(t, filtered, []SemVer{"0.0.3"})
}

func requireSemverMutationSlice(t *testing.T, got, want []SemVer) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("slice length = %d, want %d; got %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q; got %v", i, got[i], want[i], got)
		}
	}
}
