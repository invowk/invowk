// SPDX-License-Identifier: MPL-2.0

package isvalidusage

import "errors"

// DddType is a DDD value type with IsValid.
type DddType string

// ErrInvalid is the sentinel error.
var ErrInvalid = errors.New("invalid ddd type")

// IsValid validates the DDD type.
func (d DddType) IsValid() (bool, []error) {
	if d == "" {
		return false, []error{ErrInvalid}
	}
	return true, nil
}

// String returns the string representation.
func (d DddType) String() string { return string(d) }

// NonDddType does not have IsValid — calls on this should be ignored.
type NonDddType struct {
	value string // want `struct field isvalidusage\.NonDddType\.value uses primitive type string`
}

// IsValid on NonDddType returns only bool — wrong signature, not a DDD type.
func (n NonDddType) IsValid() bool { return n.value != "" }

// --- Discarded result tests ---

func discardedResult() {
	d := DddType("test")
	d.IsValid() // want `IsValid\(\) result discarded`
}

func blankedResult() {
	d := DddType("test")
	_, _ = d.IsValid() // want `IsValid\(\) result discarded`
}

// --- Truncated errors tests ---

func truncatedErrors() {
	d := DddType("test")
	_, errs := d.IsValid()
	if len(errs) > 0 {
		_ = errs[0] // want `IsValid\(\) errors truncated via errs\[0\]`
	}
}

func truncatedErrorsDifferentName() {
	d := DddType("test")
	_, fieldErrs := d.IsValid()
	if len(fieldErrs) > 0 {
		_ = fieldErrs[0] // want `IsValid\(\) errors truncated via fieldErrs\[0\]`
	}
}

// --- Correct usage tests (should NOT be flagged) ---

func correctUsageJoin() {
	d := DddType("test")
	isValid, errs := d.IsValid()
	if !isValid {
		_ = errors.Join(errs...)
	}
}

func correctUsageCheckBool() {
	d := DddType("test")
	if ok, _ := d.IsValid(); !ok {
		return
	}
}

func correctUsageIfInit() {
	d := DddType("test")
	if isValid, errs := d.IsValid(); !isValid {
		_ = errors.Join(errs...)
	}
}

func correctUsageAssignBool() {
	d := DddType("test")
	ok, _ := d.IsValid()
	_ = ok
}

func correctUsageRangeErrors() {
	d := DddType("test")
	_, errs := d.IsValid()
	for _, e := range errs {
		_ = e
	}
}

// --- Non-DDD type calls should NOT be flagged ---

func nonDddTypeDiscarded() {
	n := NonDddType{value: "test"}
	n.IsValid() // NOT flagged — wrong IsValid() signature (returns bool only)
}

// --- Closure isolation ---

func closureNotAnalyzed() {
	d := DddType("test")
	go func() {
		d.IsValid() // NOT flagged — closure body is skipped
	}()
	_ = d
}

// --- Index with non-zero value should NOT be flagged ---

func indexNonZero() {
	d := DddType("test")
	_, errs := d.IsValid()
	if len(errs) > 1 {
		_ = errs[1] // NOT flagged — not [0] truncation
	}
}

// --- Index with variable should NOT be flagged ---

func indexWithVariable() {
	d := DddType("test")
	_, errs := d.IsValid()
	i := 0
	if len(errs) > 0 {
		_ = errs[i] // NOT flagged — not a literal 0
	}
}

// --- Error slice assigned to blank should NOT track ---

func errSliceBlank() {
	d := DddType("test")
	ok, _ := d.IsValid()
	_ = ok
	// No errs[0] possible since errs is _.
}
