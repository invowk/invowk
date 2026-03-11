// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPrerel string
		wantErr    bool
	}{
		{"full", "1.2.3", 1, 2, 3, "", false},
		{"v_prefix", "v2.3.4", 2, 3, 4, "", false},
		{"major_only", "1", 1, 0, 0, "", false},
		{"major_minor", "1.2", 1, 2, 0, "", false},
		{"zeros", "0.0.0", 0, 0, 0, "", false},
		{"large_numbers", "999.888.777", 999, 888, 777, "", false},
		{"prerelease", "1.0.0-alpha.1", 1, 0, 0, "alpha.1", false},
		{"v_prerelease", "v1.2.3-rc.1", 1, 2, 3, "rc.1", false},
		{"prerelease_beta", "2.0.0-beta", 2, 0, 0, "beta", false},
		{"build_metadata", "1.2.3+build.123", 1, 2, 3, "", false},
		{"prerelease_and_build", "1.2.3-alpha+build", 1, 2, 3, "alpha", false},

		// Error cases
		{"empty", "", 0, 0, 0, "", true},
		{"letters", "abc", 0, 0, 0, "", true},
		{"v_only", "v", 0, 0, 0, "", true},
		{"leading_dot", ".1.0", 0, 0, 0, "", true},
		{"double_dot", "1..0", 0, 0, 0, "", true},
		{"negative", "-1.0.0", 0, 0, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseVersion(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q) unexpected error: %v", tt.input, err)
			}
			if v.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", v.Major, tt.wantMajor)
			}
			if v.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", v.Minor, tt.wantMinor)
			}
			if v.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", v.Patch, tt.wantPatch)
			}
			if v.Prerelease != tt.wantPrerel {
				t.Errorf("Prerelease = %q, want %q", v.Prerelease, tt.wantPrerel)
			}
			if v.Original != tt.input {
				t.Errorf("Original = %q, want %q", v.Original, tt.input)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"1.0.0", "1.0.0"},
		{"v2.3.4-alpha", "v2.3.4-alpha"},
		{"0.1.0", "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			v, err := ParseVersion(tt.input)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error: %v", tt.input, err)
			}
			if got := v.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersion_Compare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		// Equal
		{"equal_simple", "1.0.0", "1.0.0", 0},
		{"equal_prerelease", "1.0.0-alpha", "1.0.0-alpha", 0},

		// Major diff
		{"major_greater", "2.0.0", "1.0.0", 1},
		{"major_less", "1.0.0", "2.0.0", -1},

		// Minor diff
		{"minor_greater", "1.2.0", "1.1.0", 1},
		{"minor_less", "1.1.0", "1.2.0", -1},

		// Patch diff
		{"patch_greater", "1.0.2", "1.0.1", 1},
		{"patch_less", "1.0.1", "1.0.2", -1},

		// Prerelease semantics: release > prerelease
		{"release_gt_prerelease", "1.0.0", "1.0.0-alpha", 1},
		{"prerelease_lt_release", "1.0.0-alpha", "1.0.0", -1},

		// Prerelease lexicographic ordering
		{"prerelease_alpha_lt_beta", "1.0.0-alpha", "1.0.0-beta", -1},
		{"prerelease_beta_gt_alpha", "1.0.0-beta", "1.0.0-alpha", 1},
		{"prerelease_rc_gt_beta", "1.0.0-rc.1", "1.0.0-beta.1", 1},

		// Known limitation: lexicographic comparison of numeric identifiers
		{"prerelease_lex_9_gt_10", "1.0.0-alpha.9", "1.0.0-alpha.10", 1},

		// Mixed major with prerelease
		{"higher_major_trumps_prerelease", "2.0.0-alpha", "1.9.9", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			va, err := ParseVersion(tt.a)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error: %v", tt.a, err)
			}
			vb, err := ParseVersion(tt.b)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error: %v", tt.b, err)
			}
			if got := va.Compare(vb); got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSemverResolver_ParseConstraint(t *testing.T) {
	t.Parallel()

	r := NewSemverResolver()

	tests := []struct {
		name    string
		input   string
		wantOp  ConstraintOp
		wantErr bool
	}{
		{"caret", "^1.2.3", ConstraintOpCaret, false},
		{"tilde", "~1.0.0", ConstraintOpTilde, false},
		{"gt", ">1.0.0", ConstraintOpGT, false},
		{"gte", ">=1.0.0", ConstraintOpGTE, false},
		{"lt", "<2.0.0", ConstraintOpLT, false},
		{"lte", "<=2.0.0", ConstraintOpLTE, false},
		{"exact", "=1.0.0", ConstraintOpEqual, false},
		{"implicit_equal", "1.2.3", ConstraintOpEqual, false},
		{"whitespace", " ^1.0.0 ", ConstraintOpCaret, false},
		{"v_prefix", "^v1.0.0", ConstraintOpCaret, false},

		// Error cases
		{"empty", "", ConstraintOpEqual, true},
		{"invalid_op", ">>1.0.0", ConstraintOpEqual, true},
		{"no_version", "^", ConstraintOpEqual, true},
		{"letters_only", "abc", ConstraintOpEqual, true},
		{"unsupported_op", "!=1.0.0", ConstraintOpEqual, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := r.ParseConstraint(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseConstraint(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConstraint(%q) unexpected error: %v", tt.input, err)
			}
			if c.Op != tt.wantOp {
				t.Errorf("Op = %q, want %q", c.Op, tt.wantOp)
			}
			if c.Version == nil {
				t.Fatal("Version is nil")
			}
		})
	}
}

func TestConstraint_Matches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		// Equal
		{"eq_match", "=1.0.0", "1.0.0", true},
		{"eq_no_match", "=1.0.0", "1.0.1", false},

		// Caret: non-zero major → same major
		{"caret_exact", "^1.2.3", "1.2.3", true},
		{"caret_higher_patch", "^1.2.3", "1.2.9", true},
		{"caret_higher_minor", "^1.2.3", "1.9.0", true},
		{"caret_next_major", "^1.2.3", "2.0.0", false},
		{"caret_lower", "^1.2.3", "1.2.2", false},

		// Caret: zero major, non-zero minor → same major.minor
		{"caret_zero_major_match", "^0.2.3", "0.2.3", true},
		{"caret_zero_major_higher_patch", "^0.2.3", "0.2.9", true},
		{"caret_zero_major_next_minor", "^0.2.3", "0.3.0", false},
		{"caret_zero_major_next_major", "^0.2.3", "1.0.0", false},

		// Caret: zero major, zero minor → exact patch
		{"caret_zero_zero_match", "^0.0.3", "0.0.3", true},
		{"caret_zero_zero_higher_patch", "^0.0.3", "0.0.4", false},
		{"caret_zero_zero_higher_minor", "^0.0.3", "0.1.0", false},

		// Tilde: same major.minor
		{"tilde_exact", "~1.2.3", "1.2.3", true},
		{"tilde_higher_patch", "~1.2.3", "1.2.9", true},
		{"tilde_next_minor", "~1.2.3", "1.3.0", false},
		{"tilde_lower", "~1.2.3", "1.2.2", false},
		{"tilde_next_major", "~1.2.3", "2.0.0", false},

		// Greater than
		{"gt_higher", ">1.0.0", "1.0.1", true},
		{"gt_much_higher", ">1.0.0", "2.0.0", true},
		{"gt_equal", ">1.0.0", "1.0.0", false},
		{"gt_lower", ">1.0.0", "0.9.0", false},

		// Greater than or equal
		{"gte_equal", ">=1.0.0", "1.0.0", true},
		{"gte_higher", ">=1.0.0", "1.0.1", true},
		{"gte_lower", ">=1.0.0", "0.9.9", false},

		// Less than
		{"lt_lower", "<2.0.0", "1.9.9", true},
		{"lt_much_lower", "<2.0.0", "0.0.1", true},
		{"lt_equal", "<2.0.0", "2.0.0", false},
		{"lt_higher", "<2.0.0", "2.0.1", false},

		// Less than or equal
		{"lte_equal", "<=2.0.0", "2.0.0", true},
		{"lte_lower", "<=2.0.0", "1.0.0", true},
		{"lte_higher", "<=2.0.0", "2.0.1", false},
	}

	r := NewSemverResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := r.ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error: %v", tt.constraint, err)
			}
			v, err := ParseVersion(tt.version)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error: %v", tt.version, err)
			}
			if got := c.Matches(v); got != tt.want {
				t.Errorf("Constraint(%q).Matches(%q) = %v, want %v",
					tt.constraint, tt.version, got, tt.want)
			}
		})
	}
}

func TestConstraint_Matches_UnknownOp(t *testing.T) {
	t.Parallel()

	v, err := ParseVersion("1.0.0")
	if err != nil {
		t.Fatalf("ParseVersion error: %v", err)
	}
	c := &Constraint{Op: ConstraintOp("??"), Version: v}
	if c.Matches(v) {
		t.Error("unknown operator should not match")
	}
}

func TestSemverResolver_Resolve(t *testing.T) {
	t.Parallel()

	r := NewSemverResolver()

	tests := []struct {
		name       string
		constraint string
		available  []SemVer
		want       SemVer
		wantErr    bool
	}{
		{
			name:       "highest_match",
			constraint: "^1.0.0",
			available:  []SemVer{"1.0.0", "1.1.0", "1.2.0", "2.0.0"},
			want:       "1.2.0",
		},
		{
			name:       "single_match",
			constraint: "=1.0.0",
			available:  []SemVer{"1.0.0", "2.0.0"},
			want:       "1.0.0",
		},
		{
			name:       "no_match",
			constraint: "^3.0.0",
			available:  []SemVer{"1.0.0", "2.0.0"},
			wantErr:    true,
		},
		{
			name:       "empty_versions",
			constraint: "^1.0.0",
			available:  []SemVer{},
			wantErr:    true,
		},
		{
			name:       "invalid_versions_skipped",
			constraint: "^1.0.0",
			available:  []SemVer{"invalid", "1.0.0", "bad", "1.1.0"},
			want:       "1.1.0",
		},
		{
			name:       "all_invalid_versions",
			constraint: "^1.0.0",
			available:  []SemVer{"invalid", "bad"},
			wantErr:    true,
		},
		{
			name:       "invalid_constraint",
			constraint: ">>bad",
			available:  []SemVer{"1.0.0"},
			wantErr:    true,
		},
		{
			name:       "tilde_highest_patch",
			constraint: "~1.2.0",
			available:  []SemVer{"1.2.0", "1.2.5", "1.3.0"},
			want:       "1.2.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := r.Resolve(tt.constraint, tt.available)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Resolve(%q) expected error, got %q", tt.constraint, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q) unexpected error: %v", tt.constraint, err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.constraint, got, tt.want)
			}
		})
	}
}

func TestSortVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []SemVer
		want  []SemVer
	}{
		{
			name:  "descending",
			input: []SemVer{"1.0.0", "3.0.0", "2.0.0"},
			want:  []SemVer{"3.0.0", "2.0.0", "1.0.0"},
		},
		{
			name:  "already_sorted",
			input: []SemVer{"3.0.0", "2.0.0", "1.0.0"},
			want:  []SemVer{"3.0.0", "2.0.0", "1.0.0"},
		},
		{
			name:  "with_prerelease",
			input: []SemVer{"1.0.0", "1.0.0-alpha", "1.0.0-beta"},
			want:  []SemVer{"1.0.0", "1.0.0-beta", "1.0.0-alpha"},
		},
		{
			name:  "invalid_filtered",
			input: []SemVer{"2.0.0", "invalid", "1.0.0"},
			want:  []SemVer{"2.0.0", "1.0.0"},
		},
		{
			name:  "empty",
			input: []SemVer{},
			want:  []SemVer{},
		},
		{
			name:  "nil",
			input: nil,
			want:  []SemVer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SortVersions(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("SortVersions() returned %d elements, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("SortVersions()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilterVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		constraint string
		versions   []SemVer
		wantLen    int
		wantErr    bool
	}{
		{
			name:       "caret_filter",
			constraint: "^1.0.0",
			versions:   []SemVer{"1.0.0", "1.1.0", "2.0.0", "0.9.0"},
			wantLen:    2,
		},
		{
			name:       "no_matches",
			constraint: "^3.0.0",
			versions:   []SemVer{"1.0.0", "2.0.0"},
			wantLen:    0,
		},
		{
			name:       "invalid_constraint",
			constraint: ">>bad",
			versions:   []SemVer{"1.0.0"},
			wantErr:    true,
		},
		{
			name:       "invalid_versions_skipped",
			constraint: "^1.0.0",
			versions:   []SemVer{"invalid", "1.0.0"},
			wantLen:    1,
		},
		{
			name:       "empty_versions",
			constraint: "^1.0.0",
			versions:   []SemVer{},
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := FilterVersions(tt.constraint, tt.versions)
			if tt.wantErr {
				if err == nil {
					t.Fatal("FilterVersions() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("FilterVersions() unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("FilterVersions() returned %d elements, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestIsValidConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"^1.0.0", true},
		{"~1.2.3", true},
		{">=1.0.0", true},
		{"1.0.0", true},
		{"", false},
		{">>bad", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := IsValidConstraint(tt.input); got != tt.want {
				t.Errorf("IsValidConstraint(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"1.0.0", true},
		{"v2.3.4", true},
		{"1.0.0-alpha", true},
		{"1", true},
		{"", false},
		{"abc", false},
		{"v", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := isValidVersionString(tt.input); got != tt.want {
				t.Errorf("isValidVersionString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
