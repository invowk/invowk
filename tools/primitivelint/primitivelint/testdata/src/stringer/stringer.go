package stringer

// CommandName has both IsValid and String — no diagnostic.
type CommandName string

func (c CommandName) IsValid() (bool, []error) { return c != "", nil }
func (c CommandName) String() string            { return string(c) }

// MissingStringer has IsValid but no String.
type MissingStringer string // want `named type stringer\.MissingStringer has no String\(\) method`

func (m MissingStringer) IsValid() (bool, []error) { return m != "", nil }

// MissingBoth has neither IsValid nor String.
type MissingBoth int // want `named type stringer\.MissingBoth has no String\(\) method`

// PointerReceiver uses *T — should still be recognized.
type PointerReceiver string

func (p *PointerReceiver) String() string { return string(*p) }

// MyStruct is a struct — checked by primary mode, not by --check-stringer.
type MyStruct struct {
	Name string // want `struct field stringer\.MyStruct\.Name uses primitive type string`
}
