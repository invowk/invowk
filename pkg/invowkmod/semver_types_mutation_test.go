// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestSemVerTypesMutationInvalidValuePayloads(t *testing.T) {
	t.Parallel()

	version := SemVer("not-a-version")
	versionErr := version.Validate()
	if !errors.Is(versionErr, ErrInvalidSemVer) {
		t.Fatalf("SemVer(%q).Validate() error = %v, want ErrInvalidSemVer", version, versionErr)
	}
	var invalidVersion *InvalidSemVerError
	if !errors.As(versionErr, &invalidVersion) {
		t.Fatalf("SemVer(%q).Validate() error type = %T, want *InvalidSemVerError", version, versionErr)
	}
	if invalidVersion.Value != version {
		t.Fatalf("InvalidSemVerError.Value = %q, want %q", invalidVersion.Value, version)
	}

	constraint := SemVerConstraint(">>1.0")
	constraintErr := constraint.Validate()
	if !errors.Is(constraintErr, ErrInvalidSemVerConstraint) {
		t.Fatalf("SemVerConstraint(%q).Validate() error = %v, want ErrInvalidSemVerConstraint", constraint, constraintErr)
	}
	var invalidConstraint *InvalidSemVerConstraintError
	if !errors.As(constraintErr, &invalidConstraint) {
		t.Fatalf("SemVerConstraint(%q).Validate() error type = %T, want *InvalidSemVerConstraintError", constraint, constraintErr)
	}
	if invalidConstraint.Value != constraint {
		t.Fatalf("InvalidSemVerConstraintError.Value = %q, want %q", invalidConstraint.Value, constraint)
	}

	op := ConstraintOp("!=")
	opErr := op.Validate()
	if !errors.Is(opErr, ErrInvalidConstraintOp) {
		t.Fatalf("ConstraintOp(%q).Validate() error = %v, want ErrInvalidConstraintOp", op, opErr)
	}
	var invalidOp *InvalidConstraintOpError
	if !errors.As(opErr, &invalidOp) {
		t.Fatalf("ConstraintOp(%q).Validate() error type = %T, want *InvalidConstraintOpError", op, opErr)
	}
	if invalidOp.Value != op {
		t.Fatalf("InvalidConstraintOpError.Value = %q, want %q", invalidOp.Value, op)
	}
}
