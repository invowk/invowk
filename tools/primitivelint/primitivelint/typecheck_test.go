// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"go/token"
	"go/types"
	"testing"
)

func TestIsPrimitiveBasic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind types.BasicKind
		want bool
	}{
		// Exempt types
		{name: "Bool", kind: types.Bool, want: false},
		{name: "UntypedBool", kind: types.UntypedBool, want: false},

		// Flagged string types
		{name: "String", kind: types.String, want: true},
		{name: "UntypedString", kind: types.UntypedString, want: true},

		// Flagged int types
		{name: "Int", kind: types.Int, want: true},
		{name: "Int8", kind: types.Int8, want: true},
		{name: "Int16", kind: types.Int16, want: true},
		{name: "Int32", kind: types.Int32, want: true},
		{name: "Int64", kind: types.Int64, want: true},

		// Flagged uint types
		{name: "Uint", kind: types.Uint, want: true},
		{name: "Uint8", kind: types.Uint8, want: true},
		{name: "Uint16", kind: types.Uint16, want: true},
		{name: "Uint32", kind: types.Uint32, want: true},
		{name: "Uint64", kind: types.Uint64, want: true},

		// Flagged float types
		{name: "Float32", kind: types.Float32, want: true},
		{name: "Float64", kind: types.Float64, want: true},

		// Flagged untyped numeric
		{name: "UntypedInt", kind: types.UntypedInt, want: true},
		{name: "UntypedFloat", kind: types.UntypedFloat, want: true},

		// Aliases
		{name: "Byte (uint8 alias)", kind: types.Byte, want: true},
		{name: "Rune (int32 alias)", kind: types.Rune, want: true},

		// Default branch — not flagged
		{name: "Complex64", kind: types.Complex64, want: false},
		{name: "Complex128", kind: types.Complex128, want: false},
		{name: "UntypedNil", kind: types.UntypedNil, want: false},
		{name: "UntypedComplex", kind: types.UntypedComplex, want: false},
		{name: "Uintptr", kind: types.Uintptr, want: false},
		{name: "UnsafePointer", kind: types.UnsafePointer, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isPrimitiveBasic(types.Typ[tt.kind])
			if got != tt.want {
				t.Errorf("isPrimitiveBasic(%v) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

// makeNamedType creates a *types.Named wrapping *types.Basic(String) for testing.
func makeNamedType() *types.Named {
	pkg := types.NewPackage("test/pkg", "pkg")
	typeName := types.NewTypeName(token.NoPos, pkg, "MyString", nil)
	named := types.NewNamed(typeName, types.Typ[types.String], nil)
	return named
}

// makeAliasType creates a *types.Alias wrapping the given type for testing.
func makeAliasType(rhs types.Type) *types.Alias {
	pkg := types.NewPackage("test/pkg", "pkg")
	aliasName := types.NewTypeName(token.NoPos, pkg, "MyAlias", nil)
	return types.NewAlias(aliasName, rhs)
}

func TestIsPrimitive(t *testing.T) {
	t.Parallel()

	named := makeNamedType()

	tests := []struct {
		name string
		typ  types.Type
		want bool
	}{
		// Basic types
		{name: "bare string", typ: types.Typ[types.String], want: true},
		{name: "bare int", typ: types.Typ[types.Int], want: true},
		{name: "bare bool (exempt)", typ: types.Typ[types.Bool], want: false},

		// Pointer types
		{name: "pointer to string", typ: types.NewPointer(types.Typ[types.String]), want: true},
		{name: "pointer to bool", typ: types.NewPointer(types.Typ[types.Bool]), want: false},
		{name: "pointer to named", typ: types.NewPointer(named), want: false},

		// Slice types
		{name: "slice of string", typ: types.NewSlice(types.Typ[types.String]), want: true},
		{name: "slice of int", typ: types.NewSlice(types.Typ[types.Int]), want: true},
		{name: "slice of byte (exempt)", typ: types.NewSlice(types.Typ[types.Byte]), want: false},
		{name: "slice of named", typ: types.NewSlice(named), want: false},

		// Map types
		{name: "map[string]string", typ: types.NewMap(types.Typ[types.String], types.Typ[types.String]), want: true},
		{name: "map[named]string (value prim)", typ: types.NewMap(named, types.Typ[types.String]), want: true},
		{name: "map[string]named (key prim)", typ: types.NewMap(types.Typ[types.String], named), want: true},
		{name: "map[named]named", typ: types.NewMap(named, named), want: false},

		// Named type
		{name: "named type", typ: named, want: false},

		// Alias type — transparent, resolves to underlying
		{name: "alias of string", typ: makeAliasType(types.Typ[types.String]), want: true},
		{name: "alias of bool", typ: makeAliasType(types.Typ[types.Bool]), want: false},
		{name: "alias of named", typ: makeAliasType(named), want: false},

		// Default branch: channel, func, interface
		{name: "channel", typ: types.NewChan(types.SendRecv, types.Typ[types.Int]), want: false},
		{name: "signature (func type)", typ: types.NewSignatureType(nil, nil, nil, nil, nil, false), want: false},
		{name: "interface", typ: types.NewInterfaceType(nil, nil), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isPrimitive(tt.typ)
			if got != tt.want {
				t.Errorf("isPrimitive(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}
