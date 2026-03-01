// SPDX-License-Identifier: MPL-2.0

// Package use_before_validate provides test fixtures for the
// --check-use-before-validate mode. This mode detects when a DDD Value
// Type variable is used (passed as a function argument or non-display
// method receiver) before Validate() is called in the same basic block.
package use_before_validate

import "fmt"

// --- DDD Value Types for testing ---

// CommandName is a DDD Value Type with Validate.
type CommandName string

func (c CommandName) Validate() error {
	if c == "" {
		return fmt.Errorf("invalid command name")
	}
	return nil
}

func (c CommandName) String() string { return string(c) }

// --- Helper functions ---

func runtimeString() string { return "test" } // want `return value of use_before_validate\.runtimeString uses primitive type string`

func useCmd(_ CommandName) {}

// Server is a type with methods for receiver-use testing.
type Server struct {
	cmd CommandName
}

func (s *Server) Setup()         {}
func (s *Server) Validate() error { return nil }

// --- Use-before-validate test cases ---

// UseBeforeValidate — SHOULD be flagged. The variable x is passed to
// useCmd() before x.Validate() is called in the same block.
func UseBeforeValidate(raw string) error { // want `parameter "raw" of use_before_validate\.UseBeforeValidate uses primitive type string`
	x := CommandName(raw) // want `variable x of type CommandName used before Validate\(\) in same block`
	useCmd(x)
	return x.Validate()
}

// ValidateBeforeUse — should NOT be flagged. Validate() is called
// before the variable is used.
func ValidateBeforeUse(raw string) { // want `parameter "raw" of use_before_validate\.ValidateBeforeUse uses primitive type string`
	x := CommandName(raw)
	if err := x.Validate(); err != nil {
		return
	}
	useCmd(x)
}

// StringBeforeValidate — should NOT be flagged. String() is a
// display-only method and does not count as a "use."
func StringBeforeValidate(raw string) error { // want `parameter "raw" of use_before_validate\.StringBeforeValidate uses primitive type string`
	x := CommandName(raw)
	fmt.Println(x.String())
	return x.Validate()
}

// ErrorBeforeValidate — should NOT be flagged. Error() is display-only.
func ErrorBeforeValidate(raw string) error { // want `parameter "raw" of use_before_validate\.ErrorBeforeValidate uses primitive type string`
	x := CommandName(raw)
	_ = fmt.Sprintf("%s", x.String())
	return x.Validate()
}

// NoUseAtAll — should NOT be flagged. The cast is assigned and validated
// without any intervening use.
func NoUseAtAll(raw string) error { // want `parameter "raw" of use_before_validate\.NoUseAtAll uses primitive type string`
	x := CommandName(raw)
	return x.Validate()
}

// NoValidateButNoUse — NOT flagged by UBV (no use before validate).
// This IS flagged by the path-to-return check (unvalidated-cast).
func NoValidateButNoUse(raw string) { // want `parameter "raw" of use_before_validate\.NoValidateButNoUse uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	_ = x
}

// UseInFuncArgBeforeValidate — SHOULD be flagged. The variable is
// passed as a function argument before Validate().
func UseInFuncArgBeforeValidate(raw string) error { // want `parameter "raw" of use_before_validate\.UseInFuncArgBeforeValidate uses primitive type string`
	x := CommandName(raw) // want `variable x of type CommandName used before Validate\(\) in same block`
	fmt.Println(x) // passes x as an argument (not x.String())
	return x.Validate()
}
