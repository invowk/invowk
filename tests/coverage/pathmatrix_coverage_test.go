// SPDX-License-Identifier: MPL-2.0

// Package coverage holds cross-cutting coverage guardrails that don't fit
// inside any single domain package's test suite. The guardrails are static
// AST scans; they don't import the helpers they're checking, so cycles are
// avoided.
package coverage_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// pathMatrixSurfaces lists every test file that should call into
// internal/testutil/pathmatrix to cover its path-validator or
// path-resolver surface against the canonical seven-vector matrix. The
// list is reviewed when adding new path-resolving functions: if a new
// surface is introduced without matrix coverage, this guardrail fails.
// If a listed surface is removed or renamed, the guardrail also fails
// (forcing the list to stay in sync with the codebase).
//
// pathMatrixExemptions documents path-touching test files that are
// intentionally NOT migrated to pathmatrix in this batch. Each entry
// must include a justification. The two-way verification fails if an
// exempt file no longer exists (stale exemption) or if an exempt file
// actually does contain a pathmatrix call (unnecessary exemption —
// promote it to the surfaces list).
var (
	pathMatrixSurfaces = []string{
		"pkg/types/filesystem_path_test.go",
		"pkg/invowkmod/module_types_test.go",                         // SubdirectoryPath
		"pkg/invowkmod/operations_test.go",                           // ResolveScriptPath, ValidateScriptPath
		"pkg/invowkfile/volume_mount_test.go",                        // VolumeMountSpec
		"pkg/invowkfile/implementation_get_script_file_path_test.go", // GetScriptFilePathWithModule (NEW)
		"pkg/invowkfile/containerfile_path_test.go",                  // ContainerfilePath
		"pkg/invowkfile/validation_test.go",                          // isAbsolutePath
		"pkg/invowkfile/invowkfile_workdir_matrix_test.go",           // GetEffectiveWorkDir
		"internal/app/deps/filepaths_test.go",                        // ValidateFilepathAlternativesWithProbe (recording probe)
	}

	pathMatrixExemptions = map[string]string{
		"internal/runtime/container_test.go": "TestGetContainerWorkDir tests the container-runtime path resolver, which has different vectors than host-side resolvers (deferred until ContainerWorkDirMatrix exists)",
	}
)

// TestPathMatrixSurfaces_AreCovered verifies every listed surface contains
// at least one call to a pathmatrix.{Validator,Resolver,VolumeMount} top-level
// function. The check uses go/parser to walk each file's AST rather than
// substring scanning, so prose comments mentioning pathmatrix don't false-
// positive.
func TestPathMatrixSurfaces_AreCovered(t *testing.T) {
	t.Parallel()
	repoRoot := mustRepoRoot(t)
	for _, rel := range pathMatrixSurfaces {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			abs := filepath.Join(repoRoot, rel)
			if _, err := os.Stat(abs); err != nil {
				t.Fatalf("listed surface file does not exist: %s (err: %v)", rel, err)
			}
			if !fileCallsPathmatrix(t, abs) {
				t.Errorf("listed surface %s contains no pathmatrix.{Validator,Resolver,VolumeMount} call", rel)
			}
		})
	}
}

// TestPathMatrixExemptions_AreCurrent verifies each exempted file (a) still
// exists and (b) does NOT call pathmatrix (in which case it should be
// promoted to the surfaces list).
func TestPathMatrixExemptions_AreCurrent(t *testing.T) {
	t.Parallel()
	repoRoot := mustRepoRoot(t)
	for rel, reason := range pathMatrixExemptions {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			abs := filepath.Join(repoRoot, rel)
			if _, err := os.Stat(abs); err != nil {
				t.Errorf("stale exemption: %s no longer exists (reason was: %s)", rel, reason)
				return
			}
			if fileCallsPathmatrix(t, abs) {
				t.Errorf("unnecessary exemption: %s already calls pathmatrix; promote to pathMatrixSurfaces (reason was: %s)", rel, reason)
			}
		})
	}
}

// fileCallsPathmatrix parses the file and reports whether any call
// expression's selector matches one of the three top-level pathmatrix
// functions. Restricted to the three explicit functions so that helper
// usage like a private wrapper named `validate` doesn't accidentally count.
func fileCallsPathmatrix(t *testing.T, path string) bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	hit := false
	ast.Inspect(f, func(n ast.Node) bool {
		if hit {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "pathmatrix" {
			return true
		}
		switch sel.Sel.Name {
		case "Validator", "Resolver", "VolumeMount":
			hit = true
			return false
		}
		return true
	})
	return hit
}

// mustRepoRoot walks parents from this file until it finds go.mod, then
// returns the directory containing it. Fatals if not found.
func mustRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repo root from %s", wd)
		}
		dir = parent
	}
}
