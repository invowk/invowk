// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	gocfg "golang.org/x/tools/go/cfg"
)

// parseFuncBody parses a Go source snippet and returns the first function's
// body and its CFG for test purposes.
func parseFuncBody(t *testing.T, src string) (*ast.BlockStmt, *gocfg.CFG) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		cfg := buildFuncCFG(fn.Body)
		return fn.Body, cfg
	}
	t.Fatal("no function found in source")
	return nil, nil
}

func TestBuildFuncCFG_NilBody(t *testing.T) {
	g := buildFuncCFG(nil)
	if g != nil {
		t.Error("expected nil CFG for nil body")
	}
}

func TestBuildFuncCFG_SimpleFunction(t *testing.T) {
	src := `package p
func f() {
	x := 1
	_ = x
}`
	_, cfg := parseFuncBody(t, src)
	if cfg == nil {
		t.Fatal("expected non-nil CFG")
	}
	if len(cfg.Blocks) == 0 {
		t.Error("expected at least one block")
	}
}

func TestFindDefiningBlock_Found(t *testing.T) {
	src := `package p
func f() {
	x := 1
	y := 2
	_ = x + y
}`
	body, cfg := parseFuncBody(t, src)
	if cfg == nil {
		t.Fatal("expected non-nil CFG")
	}

	// Find the first statement (x := 1) in the CFG.
	firstStmt := body.List[0]
	block, idx := findDefiningBlock(cfg, firstStmt)
	if block == nil {
		t.Fatal("expected to find block for first statement")
	}
	if idx < 0 {
		t.Error("expected non-negative index")
	}
}

func TestFindDefiningBlock_NotFound(t *testing.T) {
	src := `package p
func f() {
	x := 1
	_ = x
}`
	_, cfg := parseFuncBody(t, src)

	// Create a node that doesn't exist in the CFG.
	fakeNode := &ast.Ident{Name: "fake"}
	block, idx := findDefiningBlock(cfg, fakeNode)
	if block != nil {
		t.Error("expected nil block for non-existent node")
	}
	if idx != -1 {
		t.Errorf("expected index -1, got %d", idx)
	}
}

func TestContainsValidateCall_Found(t *testing.T) {
	src := `package p
type T string
func (t T) Validate() error { return nil }
func f() {
	var x T
	x.Validate()
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Find the expression statement x.Validate()
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "f" {
			continue
		}
		// The second statement should be x.Validate()
		if len(fn.Body.List) < 2 {
			t.Fatal("expected at least 2 statements")
		}
		stmt := fn.Body.List[1]
		if containsValidateCall(stmt, "x", nil) {
			return // found, test passes
		}
		t.Error("expected to find Validate call on x")
		return
	}
	t.Fatal("function f not found")
}

func TestContainsValidateCall_WrongVar(t *testing.T) {
	src := `package p
type T string
func (t T) Validate() error { return nil }
func f() {
	var x T
	x.Validate()
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "f" {
			continue
		}
		stmt := fn.Body.List[1]
		if containsValidateCall(stmt, "y", nil) {
			t.Error("should not find Validate call on y when only x.Validate() exists")
		}
		return
	}
	t.Fatal("function f not found")
}

func TestCollectImmediateClosureLits(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantLen int
	}{
		{
			name: "iife only",
			src: `package p
func f() {
	func() {}()
}`,
			wantLen: 1,
		},
		{
			name: "goroutine excluded",
			src: `package p
func f() {
	go func() {}()
}`,
			wantLen: 0,
		},
		{
			name: "defer excluded",
			src: `package p
func f() {
	defer func() {}()
}`,
			wantLen: 0,
		},
		{
			name: "mixed immediate and wrappers",
			src: `package p
func f() {
	func() {}()
	go func() {}()
	defer func() {}()
	func() { func() {}() }()
}`,
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := parseFuncBody(t, tt.src)
			got := collectImmediateClosureLits(body)
			if len(got) != tt.wantLen {
				t.Fatalf("len(collectImmediateClosureLits) = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestCollectImmediateClosureLits_NilBody(t *testing.T) {
	got := collectImmediateClosureLits(nil)
	if got != nil {
		t.Fatalf("collectImmediateClosureLits(nil) = %v, want nil", got)
	}
}

func TestCollectSynchronousClosureLits_ClassifiesExactClosures(t *testing.T) {
	src := `package p
func f() {
	defer func() {}()
	go func() {}()
	func() {}()
}`
	body, _ := parseFuncBody(t, src)
	if len(body.List) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(body.List))
	}

	deferStmt, ok := body.List[0].(*ast.DeferStmt)
	if !ok {
		t.Fatalf("expected defer stmt at index 0, got %T", body.List[0])
	}
	deferLit, ok := deferStmt.Call.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatal("expected defer function literal")
	}

	goStmt, ok := body.List[1].(*ast.GoStmt)
	if !ok {
		t.Fatalf("expected go stmt at index 1, got %T", body.List[1])
	}
	goLit, ok := goStmt.Call.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatal("expected go function literal")
	}

	exprStmt, ok := body.List[2].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected expression stmt at index 2, got %T", body.List[2])
	}
	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected call expr at index 2, got %T", exprStmt.X)
	}
	immediateLit, ok := call.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatal("expected immediate function literal")
	}

	deferred := collectDeferredClosureLits(body)
	immediate := collectImmediateClosureLits(body)
	sync := collectSynchronousClosureLits(body)

	if !deferred[deferLit] {
		t.Fatal("expected defer closure to be classified as deferred")
	}
	if deferred[goLit] || deferred[immediateLit] {
		t.Fatal("did not expect go/IIFE closures in deferred set")
	}

	if !immediate[immediateLit] {
		t.Fatal("expected IIFE closure to be classified as immediate")
	}
	if immediate[deferLit] || immediate[goLit] {
		t.Fatal("did not expect defer/go closures in immediate set")
	}

	if !sync[deferLit] || !sync[immediateLit] {
		t.Fatal("expected sync set to include deferred + immediate closures")
	}
	if sync[goLit] {
		t.Fatal("did not expect goroutine closure in sync set")
	}
}
