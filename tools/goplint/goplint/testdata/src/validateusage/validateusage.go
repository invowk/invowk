// SPDX-License-Identifier: MPL-2.0

package validateusage

import "fmt"

// DddType is a DDD value type with Validate.
type DddType string

// ErrInvalid is the sentinel error.
var ErrInvalid = fmt.Errorf("invalid ddd type")

// Validate validates the DDD type.
func (d DddType) Validate() error {
	if d == "" {
		return ErrInvalid
	}
	return nil
}

// String returns the string representation.
func (d DddType) String() string { return string(d) }

// NonDddType does not have Validate — calls on this should be ignored.
type NonDddType struct {
	value string // want `struct field validateusage\.NonDddType\.value uses primitive type string`
}

// Validate on NonDddType returns only bool — wrong signature, not a DDD type.
func (n NonDddType) Validate() bool { return n.value != "" }

// --- Discarded result tests ---

func discardedResult() {
	d := DddType("test")
	d.Validate() // want `Validate\(\) result discarded`
}

func blankedResult() {
	d := DddType("test")
	_ = d.Validate() // want `Validate\(\) result discarded`
}

// --- Correct usage tests (should NOT be flagged) ---

func correctUsageCheckErr() {
	d := DddType("test")
	if err := d.Validate(); err != nil {
		_ = err
	}
}

func correctUsageAssignErr() {
	d := DddType("test")
	err := d.Validate()
	_ = err
}

func correctUsageIfInit() {
	d := DddType("test")
	if err := d.Validate(); err != nil {
		_ = err
	}
}

// --- Non-DDD type calls should NOT be flagged ---

func nonDddTypeDiscarded() {
	n := NonDddType{value: "test"}
	n.Validate() // NOT flagged — wrong Validate() signature (returns bool only)
}

// --- Closure isolation ---

func closureNotAnalyzed() {
	d := DddType("test")
	go func() {
		d.Validate() // NOT flagged — closure body is skipped
	}()
	_ = d
}
