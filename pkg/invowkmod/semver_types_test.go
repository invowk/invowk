// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestSemVer_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sv      SemVer
		want    bool
		wantErr bool
	}{
		{"simple", SemVer("1.0.0"), true, false},
		{"with_v_prefix", SemVer("v2.3.4"), true, false},
		{"with_prerelease", SemVer("v2.3.4-alpha.1"), true, false},
		{"major_only", SemVer("1"), true, false},
		{"major_minor", SemVer("1.2"), true, false},
		{"empty", SemVer(""), false, true},
		{"invalid", SemVer("abc"), false, true},
		{"not_a_version", SemVer("not-a-version"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.sv.IsValid()
			if isValid != tt.want {
				t.Errorf("SemVer(%q).IsValid() = %v, want %v", tt.sv, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("SemVer(%q).IsValid() returned no errors, want error", tt.sv)
				}
				if !errors.Is(errs[0], ErrInvalidSemVer) {
					t.Errorf("error should wrap ErrInvalidSemVer, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("SemVer(%q).IsValid() returned unexpected errors: %v", tt.sv, errs)
			}
		})
	}
}

func TestSemVer_String(t *testing.T) {
	t.Parallel()
	sv := SemVer("1.2.3")
	if sv.String() != "1.2.3" {
		t.Errorf("SemVer.String() = %q, want %q", sv.String(), "1.2.3")
	}
}

func TestSemVerConstraint_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sc      SemVerConstraint
		want    bool
		wantErr bool
	}{
		{"caret", SemVerConstraint("^1.2.0"), true, false},
		{"tilde", SemVerConstraint("~1.0.0"), true, false},
		{"gte", SemVerConstraint(">=1.0.0"), true, false},
		{"exact", SemVerConstraint("1.2.3"), true, false},
		{"lt", SemVerConstraint("<2.0.0"), true, false},
		{"empty", SemVerConstraint(""), false, true},
		{"invalid", SemVerConstraint(">>1.0"), false, true},
		{"not_a_constraint", SemVerConstraint("abc"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.sc.IsValid()
			if isValid != tt.want {
				t.Errorf("SemVerConstraint(%q).IsValid() = %v, want %v", tt.sc, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("SemVerConstraint(%q).IsValid() returned no errors, want error", tt.sc)
				}
				if !errors.Is(errs[0], ErrInvalidSemVerConstraint) {
					t.Errorf("error should wrap ErrInvalidSemVerConstraint, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("SemVerConstraint(%q).IsValid() returned unexpected errors: %v", tt.sc, errs)
			}
		})
	}
}

func TestSemVerConstraint_String(t *testing.T) {
	t.Parallel()
	sc := SemVerConstraint("^1.2.0")
	if sc.String() != "^1.2.0" {
		t.Errorf("SemVerConstraint.String() = %q, want %q", sc.String(), "^1.2.0")
	}
}

func TestConstraintOp_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		op      ConstraintOp
		want    bool
		wantErr bool
	}{
		{"equal", ConstraintOpEqual, true, false},
		{"caret", ConstraintOpCaret, true, false},
		{"tilde", ConstraintOpTilde, true, false},
		{"gt", ConstraintOpGT, true, false},
		{"gte", ConstraintOpGTE, true, false},
		{"lt", ConstraintOpLT, true, false},
		{"lte", ConstraintOpLTE, true, false},
		{"empty", ConstraintOp(""), false, true},
		{"invalid", ConstraintOp("!="), false, true},
		{"double_gt", ConstraintOp(">>"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.op.IsValid()
			if isValid != tt.want {
				t.Errorf("ConstraintOp(%q).IsValid() = %v, want %v", tt.op, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ConstraintOp(%q).IsValid() returned no errors, want error", tt.op)
				}
				if !errors.Is(errs[0], ErrInvalidConstraintOp) {
					t.Errorf("error should wrap ErrInvalidConstraintOp, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ConstraintOp(%q).IsValid() returned unexpected errors: %v", tt.op, errs)
			}
		})
	}
}

func TestConstraintOp_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		op   ConstraintOp
		want string
	}{
		{ConstraintOpEqual, "="},
		{ConstraintOpCaret, "^"},
		{ConstraintOpTilde, "~"},
		{ConstraintOpGT, ">"},
		{ConstraintOpGTE, ">="},
		{ConstraintOpLT, "<"},
		{ConstraintOpLTE, "<="},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.op.String(); got != tt.want {
				t.Errorf("ConstraintOp.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
