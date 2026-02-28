// SPDX-License-Identifier: MPL-2.0

// Package cfa_castvalidation provides test fixtures for the CFA-enhanced
// cast-validation mode. These fixtures target path-reachability-specific
// behavior where the CFA mode and AST heuristic differ.
package cfa_castvalidation

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

// --- Helpers that provide runtime values ---

func runtimeString() string { return "test" } // want `return value of cfa_castvalidation\.runtimeString uses primitive type string`

func useCmd(_ CommandName) {}

// --- CFA-specific test cases ---

// ValidateInDeadBranch — CFA flags this because the Validate() call is in
// unreachable code (if false). The AST heuristic would miss this because
// it sees "x.Validate()" anywhere in the function body.
func ValidateInDeadBranch(raw string) { // want `parameter "raw" of cfa_castvalidation\.ValidateInDeadBranch uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	useCmd(x)
	if false {
		_ = x.Validate()
	}
}

// ValidateAfterUse — NOT flagged by CFA because x.Validate() is called
// on the return path. CFA checks "path-to-return-without-validate,"
// not "use-before-validate." The fact that useCmd(x) precedes Validate()
// doesn't matter — all paths to return pass through Validate().
func ValidateAfterUse(raw string) error { // want `parameter "raw" of cfa_castvalidation\.ValidateAfterUse uses primitive type string`
	x := CommandName(raw)
	useCmd(x)
	return x.Validate()
}

// ValidateOnOneBranch — CFA flags this because when !strict, the cast
// reaches the useCmd call (and then return) without Validate().
func ValidateOnOneBranch(raw string, strict bool) { // want `parameter "raw" of cfa_castvalidation\.ValidateOnOneBranch uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	if strict {
		if err := x.Validate(); err != nil {
			return
		}
	}
	useCmd(x)
}

// ValidateOnAllBranches — NOT flagged because both branches validate.
func ValidateOnAllBranches(raw string, mode bool) { // want `parameter "raw" of cfa_castvalidation\.ValidateOnAllBranches uses primitive type string`
	x := CommandName(raw)
	if mode {
		if err := x.Validate(); err != nil {
			return
		}
	} else {
		if err := x.Validate(); err != nil {
			return
		}
	}
	useCmd(x)
}

// ValidateBeforeUse — NOT flagged because Validate() is called on all
// paths before the variable is used.
func ValidateBeforeUse(raw string) { // want `parameter "raw" of cfa_castvalidation\.ValidateBeforeUse uses primitive type string`
	x := CommandName(raw)
	if err := x.Validate(); err != nil {
		return
	}
	useCmd(x)
}

// SimpleNoValidation — basic case flagged by both AST and CFA.
func SimpleNoValidation(raw string) { // want `parameter "raw" of cfa_castvalidation\.SimpleNoValidation uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	useCmd(x)
}

// SimpleWithValidation — basic case NOT flagged by either.
func SimpleWithValidation(raw string) { // want `parameter "raw" of cfa_castvalidation\.SimpleWithValidation uses primitive type string`
	x := CommandName(raw)
	if err := x.Validate(); err != nil {
		return
	}
	useCmd(x)
}

// UnassignedReturn — unassigned casts flagged by both modes.
func UnassignedReturn(raw string) CommandName { // want `parameter "raw" of cfa_castvalidation\.UnassignedReturn uses primitive type string`
	return CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
}

// ConstantCast — NOT flagged (constant source).
func ConstantCast() {
	x := CommandName("literal")
	useCmd(x)
}

// UnassignedMapKey — NOT flagged (auto-skip: map index).
func UnassignedMapKey(raw string) { // want `parameter "raw" of cfa_castvalidation\.UnassignedMapKey uses primitive type string`
	m := map[CommandName]bool{}
	_ = m[CommandName(raw)]
}

// SwitchTagAutoSkip — NOT flagged (auto-skip: switch tag).
func SwitchTagAutoSkip(raw string) { // want `parameter "raw" of cfa_castvalidation\.SwitchTagAutoSkip uses primitive type string`
	switch CommandName(raw) {
	case CommandName("test"):
	default:
	}
}

// FuncReturnCast — flagged for cast from function return.
func FuncReturnCast() {
	x := CommandName(runtimeString()) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	useCmd(x)
}

// --- CFA edge case: switch case validation ---

// ValidateInSwitchCase — CFA flags this because the default branch
// reaches useCmd/return without Validate().
func ValidateInSwitchCase(raw string, mode int) { // want `parameter "raw" of cfa_castvalidation\.ValidateInSwitchCase uses primitive type string` `parameter "mode" of cfa_castvalidation\.ValidateInSwitchCase uses primitive type int`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	switch mode {
	case 1:
		if err := x.Validate(); err != nil {
			return
		}
	default:
		// no validation
	}
	useCmd(x)
}

// --- CFA edge case: for loop validation ---

// ValidateInForLoop — CFA correctly flags this because the CFG has a
// "zero-iteration" path that bypasses the loop body entirely. From
// the CFG's perspective, the loop condition could be false initially,
// skipping Validate() and reaching return via ForDone.
func ValidateInForLoop(raw string) { // want `parameter "raw" of cfa_castvalidation\.ValidateInForLoop uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	for i := 0; i < 3; i++ {
		if err := x.Validate(); err != nil {
			return
		}
	}
	useCmd(x)
}

// --- CFA edge case: variable reassignment ---

// ReassignedVariable — CFA flags this because after the second assignment,
// the new cast to x is never validated (only the first had Validate).
func ReassignedVariable(a, b string) { // want `parameter "a" of cfa_castvalidation\.ReassignedVariable uses primitive type string` `parameter "b" of cfa_castvalidation\.ReassignedVariable uses primitive type string`
	x := CommandName(a)
	if err := x.Validate(); err != nil {
		return
	}
	x = CommandName(b) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	useCmd(x)
}

// --- Inline ignore directive — CFA should respect //goplint:ignore on cast lines ---

// InlineIgnoredCast — the cast has an inline ignore directive on the same line,
// so CFA should NOT flag it despite having an unvalidated path to return.
func InlineIgnoredCast(s string) CommandName { // want `parameter "s" of cfa_castvalidation\.InlineIgnoredCast uses primitive type string`
	cmd := CommandName(s) //goplint:ignore -- trusted source, CFA should skip
	return cmd
}

// InlineIgnoredCastDocComment — the cast has an ignore directive on the line above
// (doc comment pattern). CFA should NOT flag it.
func InlineIgnoredCastDocComment(s string) CommandName { // want `parameter "s" of cfa_castvalidation\.InlineIgnoredCastDocComment uses primitive type string`
	//goplint:ignore -- trusted source
	cmd := CommandName(s)
	return cmd
}

// NonIgnoredCast — no inline ignore, should be flagged normally.
func NonIgnoredCast(s string) CommandName { // want `parameter "s" of cfa_castvalidation\.NonIgnoredCast uses primitive type string`
	cmd := CommandName(s) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	return cmd
}
