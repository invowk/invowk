// SPDX-License-Identifier: MPL-2.0

package acpclient

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const acpSDKImportPath = "github.com/coder/acp-go-sdk"

type listedPackage struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func TestACPSDKImportsStayInsideFoundation(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cmd := exec.CommandContext(
		t.Context(),
		"go",
		"list",
		"-json",
		"./cmd/...",
		"./internal/...",
		"./pkg/...",
	)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("go list failed: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("go list failed: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.Fatalf("decode go list output: %v", err)
		}
		if packageImportsACP(pkg) && !strings.HasPrefix(pkg.ImportPath, "github.com/invowk/invowk/internal/acpclient") {
			t.Fatalf("%s imports %s outside internal/acpclient", pkg.ImportPath, acpSDKImportPath)
		}
	}
}

func packageImportsACP(pkg listedPackage) bool {
	return slices.Contains(pkg.Imports, acpSDKImportPath) ||
		slices.Contains(pkg.TestImports, acpSDKImportPath) ||
		slices.Contains(pkg.XTestImports, acpSDKImportPath)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root")
		}
		dir = parent
	}
}
