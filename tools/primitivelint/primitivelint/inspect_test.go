// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestShouldSkipFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		funcName string
		want     bool
	}{
		{name: "init", funcName: "init", want: true},
		{name: "main", funcName: "main", want: true},
		{name: "Test prefix", funcName: "TestFoo", want: true},
		{name: "Benchmark prefix", funcName: "BenchmarkFoo", want: true},
		{name: "Fuzz prefix", funcName: "FuzzFoo", want: true},
		{name: "Example prefix", funcName: "ExampleFoo", want: true},
		{name: "normal func", funcName: "doSomething", want: false},
		{name: "Testing (still Test prefix)", funcName: "Testing", want: true},
		{name: "test lowercase", funcName: "testHelper", want: false},
		{name: "Mainfunc (not main)", funcName: "Mainfunc", want: false},
		{name: "Init capitalized (not init)", funcName: "Init", want: false},
		{name: "empty name", funcName: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fn := &ast.FuncDecl{
				Name: ast.NewIdent(tt.funcName),
			}
			got := shouldSkipFunc(fn)
			if got != tt.want {
				t.Errorf("shouldSkipFunc(%q) = %v, want %v", tt.funcName, got, tt.want)
			}
		})
	}
}

func TestIsInterfaceMethodReturn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hasRecv bool
		fnName  string
		want    bool
	}{
		{name: "String with receiver", hasRecv: true, fnName: "String", want: true},
		{name: "Error with receiver", hasRecv: true, fnName: "Error", want: true},
		{name: "GoString with receiver", hasRecv: true, fnName: "GoString", want: true},
		{name: "MarshalText with receiver", hasRecv: true, fnName: "MarshalText", want: true},
		{name: "String without receiver", hasRecv: false, fnName: "String", want: false},
		{name: "Error without receiver", hasRecv: false, fnName: "Error", want: false},
		{name: "other method", hasRecv: true, fnName: "GetName", want: false},
		{name: "regular func", hasRecv: false, fnName: "doSomething", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fn := &ast.FuncDecl{
				Name: ast.NewIdent(tt.fnName),
			}
			if tt.hasRecv {
				fn.Recv = &ast.FieldList{
					List: []*ast.Field{{
						Type: ast.NewIdent("T"),
					}},
				}
			}
			got := isInterfaceMethodReturn(fn)
			if got != tt.want {
				t.Errorf("isInterfaceMethodReturn(recv=%v, %q) = %v, want %v",
					tt.hasRecv, tt.fnName, got, tt.want)
			}
		})
	}
}

func TestHasIgnoreDirective(t *testing.T) {
	t.Parallel()

	makeCommentGroup := func(text string) *ast.CommentGroup {
		if text == "" {
			return nil
		}
		return &ast.CommentGroup{
			List: []*ast.Comment{{Text: text}},
		}
	}

	tests := []struct {
		name        string
		doc         string
		lineComment string
		want        bool
	}{
		{
			name:        "primitivelint:ignore in line comment",
			lineComment: "//primitivelint:ignore",
			want:        true,
		},
		{
			name:        "nolint:primitivelint in line comment",
			lineComment: "//nolint:primitivelint",
			want:        true,
		},
		{
			name: "primitivelint:ignore in doc comment",
			doc:  "//primitivelint:ignore -- reason",
			want: true,
		},
		{
			name:        "regular comment, no directive",
			lineComment: "// regular comment",
			want:        false,
		},
		{
			name: "both nil",
			want: false,
		},
		{
			name:        "directive with extra text",
			lineComment: "//primitivelint:ignore -- display label",
			want:        true,
		},
		{
			name:        "directive in doc, regular in line",
			doc:         "//primitivelint:ignore",
			lineComment: "// something else",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := makeCommentGroup(tt.doc)
			lineComment := makeCommentGroup(tt.lineComment)
			got := hasIgnoreDirective(doc, lineComment)
			if got != tt.want {
				t.Errorf("hasIgnoreDirective(doc=%q, line=%q) = %v, want %v",
					tt.doc, tt.lineComment, got, tt.want)
			}
		})
	}
}

func TestReceiverTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{
			name: "simple ident",
			expr: &ast.Ident{Name: "MyType"},
			want: "MyType",
		},
		{
			name: "pointer receiver",
			expr: &ast.StarExpr{X: &ast.Ident{Name: "MyType"}},
			want: "MyType",
		},
		{
			name: "generic single param T[P]",
			expr: &ast.IndexExpr{X: &ast.Ident{Name: "Container"}},
			want: "Container",
		},
		{
			name: "generic multi param T[P1, P2]",
			expr: &ast.IndexListExpr{X: &ast.Ident{Name: "Pair"}},
			want: "Pair",
		},
		{
			name: "star with non-ident inner",
			expr: &ast.StarExpr{X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "pkg"},
				Sel: &ast.Ident{Name: "Type"},
			}},
			want: "",
		},
		{
			name: "index with non-ident inner",
			expr: &ast.IndexExpr{X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "pkg"},
				Sel: &ast.Ident{Name: "Type"},
			}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := receiverTypeName(tt.expr)
			if got != tt.want {
				t.Errorf("receiverTypeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackageName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pkg  *types.Package
		want string
	}{
		{
			name: "multi-segment path",
			pkg:  types.NewPackage("github.com/foo/bar", "bar"),
			want: "bar",
		},
		{
			name: "single segment",
			pkg:  types.NewPackage("mypackage", "mypackage"),
			want: "mypackage",
		},
		{
			name: "nil package",
			pkg:  nil,
			want: "",
		},
		{
			name: "deep nesting",
			pkg:  types.NewPackage("github.com/org/repo/internal/deep/pkg", "pkg"),
			want: "pkg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := packageName(tt.pkg)
			if got != tt.want {
				t.Errorf("packageName(%v) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

// TestIsTestFile verifies _test.go detection by calling the real
// isTestFile function with a minimal analysis.Pass (only Fset populated).
func TestIsTestFile(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()

	testFile := fset.AddFile("foo_test.go", -1, 100)
	testPos := testFile.Pos(0)

	regularFile := fset.AddFile("foo.go", -1, 100)
	regularPos := regularFile.Pos(0)

	// Minimal-named test file — exactly 8 chars ("_test.go").
	minimalTestFile := fset.AddFile("_test.go", -1, 100)
	minimalTestPos := minimalTestFile.Pos(0)

	// analysis.Pass is a struct, so we can construct a partial instance
	// with only the Fset field populated — isTestFile only uses pass.Fset.
	pass := &analysis.Pass{Fset: fset}

	if !isTestFile(pass, testPos) {
		t.Error("expected foo_test.go to be detected as test file")
	}
	if isTestFile(pass, regularPos) {
		t.Error("expected foo.go to NOT be detected as test file")
	}
	if !isTestFile(pass, minimalTestPos) {
		t.Error("expected _test.go (minimal name) to be detected as test file")
	}
}
