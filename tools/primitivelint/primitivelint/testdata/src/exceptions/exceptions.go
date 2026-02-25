package exceptions

// ExceptedStruct has a field with an ignore directive.
type ExceptedStruct struct {
	Name string //primitivelint:ignore -- display-only label
	Age  int    //nolint:primitivelint
	Bad  string // want `struct field exceptions\.ExceptedStruct\.Bad uses primitive type string`
}

// ExceptedFunc has an ignore directive on the whole function.
//
//primitivelint:ignore
func ExceptedFunc(name string) string {
	return name
}

// NotExceptedFunc does NOT have an ignore directive.
func NotExceptedFunc(name string) string { // want `parameter "name" of exceptions\.NotExceptedFunc uses primitive type string` `return value of exceptions\.NotExceptedFunc uses primitive type string`
	return name
}
