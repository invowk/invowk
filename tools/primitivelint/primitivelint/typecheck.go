// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"go/types"
)

// isPrimitive reports whether t is a bare primitive type that should be
// replaced with a DDD Value Type. Named types wrapping primitives (e.g.,
// type CommandName string) return false — they ARE the Value Types.
//
// bool is exempt per design decision: creating type Verbose bool has
// marginal DDD value.
func isPrimitive(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return isPrimitiveBasic(t)
	case *types.Pointer:
		return isPrimitive(t.Elem())
	case *types.Slice:
		// []byte is an I/O boundary type, not a domain type.
		if basic, ok := t.Elem().(*types.Basic); ok && basic.Kind() == types.Byte {
			return false
		}
		return isPrimitive(t.Elem())
	case *types.Map:
		return isPrimitive(t.Key()) || isPrimitive(t.Elem())
	case *types.Named:
		// Named types are DDD Value Types — never flagged.
		return false
	case *types.Alias:
		// Type aliases (type X = string) are transparent — check
		// the underlying type they resolve to.
		return isPrimitive(types.Unalias(t))
	default:
		// Interfaces, channels, funcs, structs, etc.
		return false
	}
}

// isPrimitiveBasic checks whether a basic type is one we flag.
// Bool and untyped bool are exempt.
func isPrimitiveBasic(t *types.Basic) bool {
	switch t.Kind() {
	case types.Bool, types.UntypedBool:
		return false
	case types.String, types.UntypedString:
		return true
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		return true
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return true
	case types.Float32, types.Float64:
		return true
	case types.UntypedInt, types.UntypedFloat:
		return true
	default:
		return false
	}
}

// isPrimitiveUnderlying reports whether t resolves to a basic primitive type.
// Used by --check-isvalid and --check-stringer to restrict checks to types
// backed by string, int, etc. — skipping func types, channels, and other
// non-primitive underlying types that don't need IsValid/String methods.
func isPrimitiveUnderlying(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return isPrimitiveBasic(t) || t.Kind() == types.Bool || t.Kind() == types.UntypedBool
	case *types.Named:
		return isPrimitiveUnderlying(t.Underlying())
	case *types.Alias:
		return isPrimitiveUnderlying(types.Unalias(t))
	default:
		return false
	}
}

// primitiveTypeName returns a human-readable name for the primitive type
// detected in a finding. For composite types (slices, maps, pointers),
// it shows the full composite form. Type aliases are resolved to their
// underlying type so the diagnostic shows the actual primitive.
func primitiveTypeName(t types.Type) string {
	return types.TypeString(types.Unalias(t), nil)
}
