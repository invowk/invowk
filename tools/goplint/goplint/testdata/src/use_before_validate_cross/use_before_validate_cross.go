// SPDX-License-Identifier: MPL-2.0

// Package use_before_validate_cross provides test fixtures for the
// --check-use-before-validate-cross mode. This mode extends UBV detection
// across CFG block boundaries: it flags cases where a DDD Value Type
// variable is used in a successor block before any block on that path
// contains a Validate() call.
package use_before_validate_cross

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

func runtimeString() string { return "test" } // want `return value of use_before_validate_cross\.runtimeString uses primitive type string`

func useCmd(_ CommandName) {}

// --- Cross-block UBV test cases ---

// CrossBlockUseBeforeValidate — SHOULD be flagged. The variable x is
// cast in the main block, used in the if-body block (useCmd(x)), and
// validated after the if statement. The if-body block uses x before
// any Validate() call on that path.
func CrossBlockUseBeforeValidate(raw string, cond bool) { // want `parameter "raw" of use_before_validate_cross\.CrossBlockUseBeforeValidate uses primitive type string`
	x := CommandName(raw) // want `variable x of type CommandName used before Validate\(\) across blocks`
	if cond {
		useCmd(x) // use in successor block — before validate
	}
	if err := x.Validate(); err != nil {
		return
	}
}

// CrossBlockValidateFirst — should NOT be flagged. Validate() is called
// in the same block as the cast, before the if statement uses x.
func CrossBlockValidateFirst(raw string, cond bool) { // want `parameter "raw" of use_before_validate_cross\.CrossBlockValidateFirst uses primitive type string`
	x := CommandName(raw)
	if err := x.Validate(); err != nil {
		return
	}
	if cond {
		useCmd(x) // use after validate — safe
	}
}

// CrossBlockValidateInBranch — should NOT be flagged. Both branches
// validate before using.
func CrossBlockValidateInBranch(raw string, cond bool) { // want `parameter "raw" of use_before_validate_cross\.CrossBlockValidateInBranch uses primitive type string`
	x := CommandName(raw)
	if cond {
		if err := x.Validate(); err != nil {
			return
		}
		useCmd(x) // validate precedes use in this block
	} else {
		if err := x.Validate(); err != nil {
			return
		}
		useCmd(x) // validate precedes use in this block
	}
}

// CrossBlockUseOnOnePath — SHOULD be flagged. The if-body uses x in a
// successor block before the Validate() call that happens after the if.
// All paths DO reach validate (the `mode == 1` branch falls through to
// the validate call), so hasPathToReturnWithoutValidate returns false.
// But the mode=1 path uses x before reaching the validate block.
func CrossBlockUseOnOnePath(raw string, mode int) { // want `parameter "raw" of use_before_validate_cross\.CrossBlockUseOnOnePath uses primitive type string` `parameter "mode" of use_before_validate_cross\.CrossBlockUseOnOnePath uses primitive type int`
	x := CommandName(raw) // want `variable x of type CommandName used before Validate\(\) across blocks`
	if mode == 1 {
		useCmd(x) // use before validate on the mode=1 path
	}
	if err := x.Validate(); err != nil {
		return
	}
}

// CrossBlockNoUse — should NOT be flagged. The variable is validated
// but never used in a non-display context.
func CrossBlockNoUse(raw string, cond bool) { // want `parameter "raw" of use_before_validate_cross\.CrossBlockNoUse uses primitive type string`
	x := CommandName(raw)
	if cond {
		fmt.Println(x.String()) // display-only — not a use
	}
	_ = x.Validate()
}

// SameBlockUBVNotCrossBlock — SHOULD be flagged by same-block UBV
// (which runs first), NOT by cross-block UBV. This verifies the
// mutual exclusion: same-block check has priority.
func SameBlockUBVNotCrossBlock(raw string) error { // want `parameter "raw" of use_before_validate_cross\.SameBlockUBVNotCrossBlock uses primitive type string`
	x := CommandName(raw) // want `variable x of type CommandName used before Validate\(\) in same block`
	useCmd(x)
	return x.Validate()
}
