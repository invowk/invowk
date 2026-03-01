// SPDX-License-Identifier: MPL-2.0

package castvalidation_method_value

import "fmt"

type CommandName string

func (c CommandName) Validate() error {
	if c == "" {
		return fmt.Errorf("invalid command")
	}
	return nil
}

func useCmd(_ CommandName) {}

// MethodValueValidate should not be flagged: calling a bound method value
// invokes Validate() on x before use.
func MethodValueValidate(raw string) { // want `parameter "raw" of castvalidation_method_value\.MethodValueValidate uses primitive type string`
	x := CommandName(raw)
	validateFn := x.Validate
	if err := validateFn(); err != nil {
		return
	}
	useCmd(x)
}

// MethodValueAliasValidate should also not be flagged when the method value
// is copied through an alias variable before invocation.
func MethodValueAliasValidate(raw string) { // want `parameter "raw" of castvalidation_method_value\.MethodValueAliasValidate uses primitive type string`
	x := CommandName(raw)
	validateFn := x.Validate
	alias := validateFn
	if err := alias(); err != nil {
		return
	}
	useCmd(x)
}

// MethodValueStoredNotCalled should be flagged because binding a method value
// alone does not validate the cast.
func MethodValueStoredNotCalled(raw string) { // want `parameter "raw" of castvalidation_method_value\.MethodValueStoredNotCalled uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	validateFn := x.Validate
	_ = validateFn
	useCmd(x)
}

// MethodValueConditionalCall should be flagged because Validate() only runs on
// one branch and therefore does not cover all return paths.
func MethodValueConditionalCall(raw string, strict bool) { // want `parameter "raw" of castvalidation_method_value\.MethodValueConditionalCall uses primitive type string`
	x := CommandName(raw) // want `type conversion to CommandName from non-constant without Validate\(\) check`
	validateFn := x.Validate
	if strict {
		if err := validateFn(); err != nil {
			return
		}
	}
	useCmd(x)
}
