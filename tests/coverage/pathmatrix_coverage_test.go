// SPDX-License-Identifier: MPL-2.0

// Package coverage holds cross-cutting coverage guardrails that don't fit
// inside any single domain package's test suite. The guardrails are static
// AST scans; they don't import the helpers they're checking, so cycles are
// avoided.
package coverage_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// pathMatrixCoverage maps production path-contract symbols to the test file
// that exercises their cross-platform behavior through pathmatrix. Production
// symbols are discovered independently, so adding an eligible validator or
// resolver without registering coverage or a justified exemption fails.
var (
	pathMatrixCoverage = map[string]pathMatrixCoverageEntry{
		"internal/app/deps.ValidateFilepathAlternativesWithProbe":   {testFile: "internal/app/deps/filepaths_test.go", testFunc: "TestValidateFilepathAlternatives_Matrix"},
		"internal/config.ModuleIncludePath.Validate":                {testFile: "internal/config/types_test.go", testFunc: "TestModuleIncludePath_Validate_Matrix"},
		"internal/container.ResolveDockerfilePath":                  {testFile: "internal/container/engine_base_volume_test.go", testFunc: "TestResolveDockerfilePath_Matrix"},
		"internal/container.VolumeMountSpec.Validate":               {testFile: "internal/container/engine_types_test.go", testFunc: "TestVolumeMountSpec_Validate_Matrix"},
		"pkg/invowkfile.ContainerfilePath.Validate":                 {testFile: "pkg/invowkfile/containerfile_path_test.go", testFunc: "TestContainerfilePath_Validate"},
		"pkg/invowkfile.Implementation.GetScriptFilePathWithModule": {testFile: "pkg/invowkfile/implementation_get_script_file_path_test.go", testFunc: "TestGetScriptFilePathWithModule_Matrix"},
		"pkg/invowkfile.Invowkfile.GetEffectiveWorkDir":             {testFile: "pkg/invowkfile/invowkfile_workdir_matrix_test.go", testFunc: "TestGetEffectiveWorkDir_Matrix"},
		"pkg/invowkfile.ScriptFilePath.ResolveFromModule":           {testFile: "pkg/invowkfile/script_file_path_test.go", testFunc: "TestScriptFilePath_ResolveFromModule_Matrix"},
		"pkg/invowkfile.ScriptFilePath.Validate":                    {testFile: "pkg/invowkfile/script_file_path_test.go", testFunc: "TestScriptFilePath_Validate_Matrix"},
		"pkg/invowkfile.ValidateContainerfilePath":                  {testFile: "pkg/invowkfile/validation_pathmatrix_test.go", testFunc: "TestValidateContainerfilePath_Matrix"},
		"pkg/invowkfile.ValidateEnvFilePath":                        {testFile: "pkg/invowkfile/validation_pathmatrix_test.go", testFunc: "TestValidateEnvFilePath_Matrix"},
		"pkg/invowkfile.VolumeMountSpec.Validate":                   {testFile: "pkg/invowkfile/volume_mount_test.go", testFunc: "TestVolumeMountSpec_Validate_Matrix"},
		"pkg/invowkfile.isAbsolutePath":                             {testFile: "pkg/invowkfile/validation_test.go", testFunc: "TestIsAbsolutePath"},
		"pkg/invowkmod.Module.ResolveScriptPath":                    {testFile: "pkg/invowkmod/operations_test.go", testFunc: "TestModule_ResolveScriptPath"},
		"pkg/invowkmod.Module.ValidateScriptPath":                   {testFile: "pkg/invowkmod/operations_test.go", testFunc: "TestModule_ValidateScriptPath"},
		"pkg/invowkmod.SubdirectoryPath.Validate":                   {testFile: "pkg/invowkmod/module_types_test.go", testFunc: "TestSubdirectoryPath_Validate"},
		"pkg/types.FilesystemPath.Validate":                         {testFile: "pkg/types/filesystem_path_test.go", testFunc: "TestFilesystemPath_Validate"},
	}

	pathMatrixExemptions = map[string]pathMatrixExemption{
		"internal/agentcmd.resolveModulePath": {
			testFile: "internal/agentcmd/module_test.go",
			reason:   "agent module targets are resolved with host-native filepath.Abs; they do not promise dialect-neutral path interpretation",
		},
		"internal/app/commandsvc.isPathInside": {
			testFile: "internal/app/commandsvc/watch_test.go",
			reason:   "watch containment compares already resolved host-native paths rather than accepting portable path syntax",
		},
		"internal/app/deps.ValidateFilepathAlternatives": {
			testFile: "internal/app/deps/filepaths_test.go",
			reason:   "thin default-probe wrapper; the path dialect contract is exercised through ValidateFilepathAlternativesWithProbe",
		},
		"internal/app/deps.resolveHostFilepathAlternative": {
			testFile: "internal/app/deps/filepaths_test.go",
			reason:   "private resolver is exercised by the registered ValidateFilepathAlternativesWithProbe matrix",
		},
		"internal/app/moduleops.validateDestinationPath": {
			testFile: "internal/app/moduleops/packaging_test.go",
			reason:   "ZIP extraction containment validates normalized archive entries, not raw host path dialects",
		},
		"internal/app/modulesync.GitFetcher.getRepoCachePath": {
			testFile: "internal/app/modulesync/git_test.go",
			reason:   "cache paths are derived from normalized Git identities and are not user-supplied path syntax",
		},
		"internal/app/modulesync.Resolver.getCachePath": {
			testFile: "internal/app/modulesync/resolver_cache_test.go",
			reason:   "cache paths are deterministic children of a validated cache root and module identity",
		},
		"internal/config.BinaryFilePath.Validate": {
			testFile: "internal/config/types_test.go",
			reason:   "BinaryFilePath.Validate is a scalar optional-value check and does not classify path dialects",
		},
		"internal/config.CacheDirPath.Validate": {
			testFile: "internal/config/types_test.go",
			reason:   "CacheDirPath.Validate is a scalar optional-value check and does not classify path dialects",
		},
		"internal/container.HostFilesystemPath.Validate": {
			testFile: "internal/container/engine_base_volume_test.go",
			reason:   "HostFilesystemPath.Validate only enforces nonblank input; volume parsing owns path dialect handling",
		},
		"internal/container.MountTargetPath.Validate": {
			testFile: "internal/container/engine_base_volume_test.go",
			reason:   "mount targets use the Linux-container path dialect and Validate only enforces nonblank input",
		},
		"internal/discovery.Discovery.getAliasForModulePath": {
			testFile: "internal/discovery/discovery_mutation_test.go",
			reason:   "alias derivation consumes an already discovered host path and only extracts its module basename",
		},
		"internal/provision.provisionedModuleDestinationPath.Validate": {
			testFile: "internal/provision/provisioner_validate_test.go",
			reason:   "provisioned destinations use canonical slash-separated Linux layer paths, not host path dialects",
		},
		"internal/runtime.ContainerRuntime.getContainerWorkDir": {
			testFile: "internal/runtime/container_test.go",
			reason:   "container workdirs use a Linux-container path dialect rather than the host-path seven-vector contract",
		},
		"internal/runtime.isContainerAbsolutePath": {
			testFile: "internal/runtime/container_provision_test.go",
			reason:   "predicate intentionally recognizes only Linux-container absolute paths",
		},
		"internal/runtime.resolveVirtualFilesystemPaths": {
			testFile: "internal/runtime/virtual_policy_test.go",
			reason:   "virtual filesystem mappings are resolved by the virtual runtime policy and its configured path dialect",
		},
		"internal/runtime.validateWorkDir": {
			testFile: "internal/runtime/env_test.go",
			reason:   "runtime workdir validation checks resolved host directory existence and permissions, not portable syntax",
		},
		"internal/runtime.virtualPathResolver.resolveBridgePath": {
			testFile: "internal/runtime/runtime_env_test.go",
			reason:   "bridge paths use virtual-runtime anchors and policy semantics rather than raw host path syntax",
		},
		"internal/uroot.HandlerContext.ResolvePath": {
			testFile: "internal/uroot/realpath_test.go",
			reason:   "u-root resolution uses the virtual filesystem namespace and virtual working directory",
		},
		"internal/uroot.validateFindPathArgs": {
			testFile: "internal/uroot/find_test.go",
			reason:   "find arguments are validated against the virtual filesystem policy",
		},
		"internal/uroot.validateNonOptionPathArgs": {
			testFile: "internal/uroot/registry_preprocessing_test.go",
			reason:   "utility arguments are validated against the virtual filesystem policy",
		},
		"internal/uroot.validatePathArg": {
			testFile: "internal/uroot/registry_preprocessing_test.go",
			reason:   "single utility arguments are validated against the virtual filesystem policy",
		},
		"internal/uroot.validateShasumPathArgs": {
			testFile: "internal/uroot/shasum_test.go",
			reason:   "shasum arguments are validated against the virtual filesystem policy",
		},
		"internal/uroot.validateTarPathArgs": {
			testFile: "internal/uroot/tar_test.go",
			reason:   "tar operands use archive and virtual-filesystem semantics",
		},
		"internal/uroot.validateUrootCommandPathArgs": {
			testFile: "internal/uroot/registry_preprocessing_test.go",
			reason:   "dispatcher validation routes utility arguments into the virtual filesystem policy",
		},
		"pkg/containerargs.isPathBoundaryMatch": {
			testFile: "pkg/containerargs/container_name_test.go",
			reason:   "security denylist boundary matching uses canonical Linux sensitive paths rather than host dialects",
		},
		"pkg/containerargs.validateSensitiveVolumeMountPath": {
			testFile: "pkg/containerargs/container_name_test.go",
			reason:   "sensitive mount validation is a Linux path denylist, not a host path validator",
		},
		"pkg/cueutil.CUEPath.Validate": {
			testFile: "pkg/cueutil/cuepath_test.go",
			reason:   "CUEPath is a schema selector path, not a filesystem path",
		},
		"pkg/invowkfile.CustomCheckScript.GetScriptFilePathWithModule": {
			testFile: "pkg/invowkfile/dependency_mutation_test.go",
			reason:   "thin delegation to ScriptFilePath.ResolveFromModule, whose dialect contract has dedicated matrix coverage",
		},
		"pkg/invowkfile.DotenvFilePath.Validate": {
			testFile: "pkg/invowkfile/dotenv_path_test.go",
			reason:   "DotenvFilePath.Validate is a scalar nonblank check; ValidateEnvFilePath owns portable path security",
		},
		"pkg/invowkfile.Implementation.GetScriptFilePath": {
			testFile: "pkg/invowkfile/invowkfile_parsing_test.go",
			reason:   "non-module compatibility wrapper; module-aware resolution is covered by GetScriptFilePathWithModule",
		},
		"pkg/invowkfile.Invowkfile.GetScriptBasePath": {
			testFile: "pkg/invowkfile/invowkfile_workdir_test.go",
			reason:   "selects between already resolved module and invowkfile directories without interpreting path syntax",
		},
		"pkg/invowkfile.ValidateFilepathDependency": {
			testFile: "pkg/invowkfile/validation_test.go",
			reason:   "schema-level collection validation delegates host resolution to the registered dependency resolver matrix",
		},
		"pkg/invowkfile.VirtualFilesystemPath.Validate": {
			testFile: "pkg/invowkfile/virtual_filesystem_test.go",
			reason:   "virtual filesystem values are scalar configured handles; runtime policy owns their path dialect",
		},
		"pkg/invowkfile.validateScriptPathContainment": {
			testFile: "pkg/invowkfile/implementation_test.go",
			reason:   "containment receives resolved host paths; raw syntax is covered by ScriptFilePath validation and resolution matrices",
		},
		"pkg/invowkfile.WorkDir.Validate": {
			testFile: "pkg/invowkfile/workdir_type_test.go",
			testFunc: "TestWorkDir_Validate",
			reason:   "WorkDir.Validate enforces value-object emptiness rules; path dialect resolution belongs to GetEffectiveWorkDir",
		},
		"pkg/invowkmod.isWindowsDrivePath": {
			testFile: "pkg/invowkmod/invowkmod_mutation_test.go",
			reason:   "private lexical predicate is exercised through the registered SubdirectoryPath validator matrix",
		},
		"pkg/invowkmod.resolveValidatedModulePath": {
			testFile: "pkg/invowkmod/operations_validate_mutation_test.go",
			reason:   "module validation first accepts an already host-native module path, then resolves symlinks and metadata",
		},
		"pkg/invowkmod.validateNormalizedSubdirectoryPath": {
			testFile: "pkg/invowkmod/module_types_test.go",
			reason:   "private implementation of the registered SubdirectoryPath.Validate matrix contract",
		},
		"pkg/types.ShellPath.Validate": {
			testFile: "pkg/invowkfile/workdir_type_test.go",
			reason:   "ShellPath.Validate is a scalar optional-value check and does not classify path dialects",
		},
	}
)

type (
	pathMatrixCoverageEntry struct {
		testFile string
		testFunc string
	}

	pathMatrixExemption struct {
		testFile string
		testFunc string
		reason   string
	}

	productionGoFile struct {
		relPath string
		pkgPath string
		file    *ast.File
	}

	pathContractEvidenceVisitor struct {
		member            string
		receiver          string
		matrixCall        bool
		memberReference   bool
		receiverReference bool
	}
)

// TestPathMatrixSurfaces_AreCovered verifies every discovered production path
// contract is registered, and every registered test file invokes a top-level
// pathmatrix helper. AST inspection prevents prose comments from satisfying
// either side of the contract.
func TestPathMatrixSurfaces_AreCovered(t *testing.T) {
	t.Parallel()
	repoRoot := mustRepoRoot(t)
	discovered, err := discoverPathContractSymbols(repoRoot)
	if err != nil {
		t.Fatalf("discover production path contracts: %v", err)
	}
	for _, issue := range reconcilePathMatrixContracts(discovered, pathMatrixCoverage, pathMatrixExemptions) {
		t.Error(issue)
	}

	for symbol, entry := range pathMatrixCoverage {
		t.Run(symbol, func(t *testing.T) {
			t.Parallel()
			abs := filepath.Join(repoRoot, entry.testFile)
			if _, err := os.Stat(abs); err != nil {
				t.Fatalf("listed surface file does not exist: %s (err: %v)", entry.testFile, err)
			}
			matrixCall, symbolReference := testFunctionContractEvidence(t, abs, entry.testFunc, symbol)
			if !matrixCall {
				t.Errorf("registered test %s in %s contains no pathmatrix.{Validator,Resolver,VolumeMount} call", entry.testFunc, entry.testFile)
			}
			if !symbolReference {
				t.Errorf("registered test %s in %s does not reference %s", entry.testFunc, entry.testFile, symbol)
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
	for symbol, exemption := range pathMatrixExemptions {
		t.Run(symbol, func(t *testing.T) {
			t.Parallel()
			abs := filepath.Join(repoRoot, exemption.testFile)
			if _, err := os.Stat(abs); err != nil {
				t.Errorf("stale exemption test file: %s no longer exists (reason was: %s)", exemption.testFile, exemption.reason)
				return
			}
			if exemption.testFunc == "" {
				return
			}
			matrixCall, symbolReference := testFunctionContractEvidence(t, abs, exemption.testFunc, symbol)
			if !symbolReference {
				t.Errorf("exemption test %s in %s does not reference %s", exemption.testFunc, exemption.testFile, symbol)
			}
			if matrixCall {
				t.Errorf("unnecessary exemption: %s already calls pathmatrix in %s (reason was: %s)", symbol, exemption.testFile, exemption.reason)
			}
		})
	}
}

func TestDiscoverPathContractSymbols(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "pkg", "demo")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	source := `package demo
type ScriptFilePath string
type DotenvFilePath string
type ShellPath string
type Other string
func (p ScriptFilePath) Validate() error { return nil }
func (p ScriptFilePath) ResolveFromModule() ScriptFilePath { return p }
func (p DotenvFilePath) Validate() error { return nil }
func (p ShellPath) Validate() error { return nil }
func ResolveConfigPath(path string) (string, error) { return path, nil }
func validateHostFilepath(path string) error { return nil }
func ConfigPath() string { return "config" }
func ValidateToken(path string) error { return nil }
func (p ScriptFilePath) String() string { return string(p) }
func (o Other) Validate() error { return nil }
`
	if err := os.WriteFile(filepath.Join(dir, "path.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := discoverPathContractSymbols(root)
	if err != nil {
		t.Fatalf("discover symbols: %v", err)
	}
	want := map[string]string{
		"pkg/demo.DotenvFilePath.Validate":          "pkg/demo/path.go",
		"pkg/demo.ResolveConfigPath":                "pkg/demo/path.go",
		"pkg/demo.ScriptFilePath.ResolveFromModule": "pkg/demo/path.go",
		"pkg/demo.ScriptFilePath.Validate":          "pkg/demo/path.go",
		"pkg/demo.ShellPath.Validate":               "pkg/demo/path.go",
		"pkg/demo.validateHostFilepath":             "pkg/demo/path.go",
	}
	if !maps.Equal(got, want) {
		t.Errorf("symbols = %v, want %v", got, want)
	}
}

func TestReconcilePathMatrixContracts_RequiresTwoWayRegistration(t *testing.T) {
	t.Parallel()

	discovered := map[string]string{"pkg/demo.NewPath.Validate": "pkg/demo/path.go"}
	coverage := map[string]pathMatrixCoverageEntry{
		"pkg/demo.StalePath.Validate": {testFile: "pkg/demo/path_test.go", testFunc: "TestStalePath"},
	}
	exemptions := map[string]pathMatrixExemption{
		"pkg/demo.StaleExemption.Validate": {testFile: "pkg/demo/stale_test.go", testFunc: "TestStaleExemption", reason: "different contract"},
	}
	want := []string{
		"stale pathmatrix coverage registration: pkg/demo.StalePath.Validate is not an eligible production path contract",
		"stale pathmatrix exemption: pkg/demo.StaleExemption.Validate is not an eligible production path contract",
		"unregistered production path contract: pkg/demo.NewPath.Validate (pkg/demo/path.go)",
	}
	got := reconcilePathMatrixContracts(discovered, coverage, exemptions)
	if !slices.Equal(got, want) {
		t.Errorf("issues = %v, want %v", got, want)
	}
}

func TestTestFunctionContractEvidence_RequiresExactBinding(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "path_test.go")
	source := `package demo_test
import "example/pathmatrix"
func TestUnrelated(t *T) { pathmatrix.Validator(t, nil, Expectations{}) }
func TestBound(t *T) {
	pathmatrix.Validator(t, func(value string) error { return ScriptFilePath(value).Validate() }, Expectations{})
}
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matrixCall, symbolReference := testFunctionContractEvidence(t, path, "TestUnrelated", "pkg/demo.ScriptFilePath.Validate")
	if !matrixCall || symbolReference {
		t.Errorf("unrelated evidence = (%v, %v), want (true, false)", matrixCall, symbolReference)
	}
	matrixCall, symbolReference = testFunctionContractEvidence(t, path, "TestBound", "pkg/demo.ScriptFilePath.Validate")
	if !matrixCall || !symbolReference {
		t.Errorf("bound evidence = (%v, %v), want (true, true)", matrixCall, symbolReference)
	}
}

// discoverPathContractSymbols derives the inventory from production syntax,
// independently of the coverage and exemption registries. It finds validation
// and resolution methods on string-backed path value types, plus path-named
// validation and resolution functions with compatible signatures.
func discoverPathContractSymbols(repoRoot string) (map[string]string, error) {
	files, err := loadProductionGoFiles(repoRoot)
	if err != nil {
		return nil, err
	}
	pathTypes := discoverStringBackedPathTypes(files)
	return discoverEligiblePathFunctions(files, pathTypes), nil
}

func loadProductionGoFiles(repoRoot string) ([]productionGoFile, error) {
	var files []productionGoFile
	for _, top := range []string{"cmd", "internal", "pkg"} {
		root := filepath.Join(repoRoot, top)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		discovered, err := loadProductionGoRoot(repoRoot, root)
		if err != nil {
			return nil, fmt.Errorf("walk production tree %s: %w", root, err)
		}
		files = append(files, discovered...)
	}
	return files, nil
}

func loadProductionGoRoot(repoRoot, root string) ([]productionGoFile, error) {
	var files []productionGoFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return productionDirectoryAction(entry.Name())
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parseProductionGoFile(repoRoot, path)
		if err != nil {
			return err
		}
		files = append(files, parsed)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files under %s: %w", root, err)
	}
	return files, nil
}

func productionDirectoryAction(name string) error {
	if name == "testdata" || name == "pathmatrix" {
		return filepath.SkipDir
	}
	return nil
}

func parseProductionGoFile(repoRoot, path string) (productionGoFile, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return productionGoFile{}, fmt.Errorf("parse %s: %w", path, err)
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return productionGoFile{}, fmt.Errorf("relative path for %s: %w", path, err)
	}
	return productionGoFile{
		relPath: filepath.ToSlash(rel),
		pkgPath: filepath.ToSlash(filepath.Dir(rel)),
		file:    file,
	}, nil
}

func discoverStringBackedPathTypes(files []productionGoFile) map[string]map[string]bool {
	pathTypes := make(map[string]map[string]bool)
	for _, parsed := range files {
		for _, name := range stringBackedPathTypeNames(parsed.file) {
			if pathTypes[parsed.pkgPath] == nil {
				pathTypes[parsed.pkgPath] = make(map[string]bool)
			}
			pathTypes[parsed.pkgPath][name] = true
		}
	}
	return pathTypes
}

func stringBackedPathTypeNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if ok && isStringBackedPathType(typeSpec) {
				names = append(names, typeSpec.Name.Name)
			}
		}
	}
	return names
}

func discoverEligiblePathFunctions(files []productionGoFile, pathTypes map[string]map[string]bool) map[string]string {
	discovered := make(map[string]string)
	for _, parsed := range files {
		for _, decl := range parsed.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			receiver := receiverTypeName(fn)
			if !isEligiblePathContract(fn, receiver, pathTypes[parsed.pkgPath][receiver]) {
				continue
			}
			symbol := parsed.pkgPath + "."
			if receiver != "" {
				symbol += receiver + "."
			}
			symbol += fn.Name.Name
			discovered[symbol] = parsed.relPath
		}
	}
	return discovered
}

func isStringBackedPathType(spec *ast.TypeSpec) bool {
	underlying, ok := spec.Type.(*ast.Ident)
	if !ok || underlying.Name != "string" {
		return false
	}
	name := strings.ToLower(spec.Name.Name)
	return strings.HasSuffix(name, "path") || name == "workdir" || name == "volumemountspec"
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return ""
	}
	receiver := fn.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	ident, _ := receiver.(*ast.Ident)
	if ident == nil {
		return ""
	}
	return ident.Name
}

func isEligiblePathContract(fn *ast.FuncDecl, receiver string, pathReceiver bool) bool {
	name := strings.ToLower(fn.Name.Name)
	if pathReceiver && (name == "validate" || strings.HasPrefix(name, "resolve")) {
		return true
	}
	if !strings.Contains(name, "path") && !strings.Contains(name, "workdir") {
		return false
	}
	if !strings.HasPrefix(name, "validate") && !strings.HasPrefix(name, "resolve") &&
		!strings.HasPrefix(name, "get") && !strings.HasPrefix(name, "is") {
		return false
	}
	return hasPathContractSignature(fn.Type, receiver != "")
}

func hasPathContractSignature(fn *ast.FuncType, hasReceiver bool) bool {
	if fn == nil || fn.Results == nil || len(fn.Results.List) == 0 {
		return false
	}
	for _, result := range fn.Results.List {
		if isPathContractResult(result.Type) {
			return true
		}
	}
	return hasReceiver && len(fn.Params.List) > 0
}

func isPathContractResult(expr ast.Expr) bool {
	switch result := expr.(type) {
	case *ast.Ident:
		name := strings.ToLower(result.Name)
		return name == "error" || name == "bool" || name == "string" || strings.HasSuffix(name, "path") || name == "workdir"
	case *ast.SelectorExpr:
		name := strings.ToLower(result.Sel.Name)
		return strings.HasSuffix(name, "path") || name == "workdir"
	case *ast.ArrayType:
		return isPathContractResult(result.Elt)
	default:
		return false
	}
}

func reconcilePathMatrixContracts(
	discovered map[string]string,
	coverage map[string]pathMatrixCoverageEntry,
	exemptions map[string]pathMatrixExemption,
) []string {
	var issues []string
	for symbol, productionFile := range discovered {
		_, covered := coverage[symbol]
		_, exempt := exemptions[symbol]
		if !covered && !exempt {
			issues = append(issues, fmt.Sprintf("unregistered production path contract: %s (%s)", symbol, productionFile))
		}
		if covered && exempt {
			issues = append(issues, fmt.Sprintf("path contract %s is both covered and exempt", symbol))
		}
	}
	for symbol := range coverage {
		if _, ok := discovered[symbol]; !ok {
			issues = append(issues, fmt.Sprintf("stale pathmatrix coverage registration: %s is not an eligible production path contract", symbol))
		}
	}
	for symbol, exemption := range exemptions {
		if strings.TrimSpace(exemption.reason) == "" {
			issues = append(issues, fmt.Sprintf("pathmatrix exemption for %s has no reason", symbol))
		}
		if _, ok := discovered[symbol]; !ok {
			issues = append(issues, fmt.Sprintf("stale pathmatrix exemption: %s is not an eligible production path contract", symbol))
		}
	}
	sort.Strings(issues)
	return issues
}

// testFunctionContractEvidence verifies that one exact test function both
// invokes a pathmatrix driver and references the registered production symbol.
// Keeping both checks in the same function prevents unrelated matrix coverage
// elsewhere in a shared test file from satisfying a registration.
func testFunctionContractEvidence(t *testing.T, path, testFunc, symbol string) (hasMatrixCall, hasSymbolReference bool) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	testDecl := findTestFunction(f, testFunc)
	if testDecl == nil {
		t.Fatalf("test function %s not found in %s", testFunc, path)
	}

	member, receiver := splitPathContractSymbol(symbol)
	visitor := &pathContractEvidenceVisitor{
		member:            member,
		receiver:          receiver,
		receiverReference: receiver == "",
	}
	ast.Walk(visitor, testDecl)
	return visitor.matrixCall, visitor.memberReference && visitor.receiverReference
}

func findTestFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func splitPathContractSymbol(symbol string) (member, receiver string) {
	parts := strings.Split(symbol, ".")
	member = parts[len(parts)-1]
	if len(parts) >= 3 {
		receiver = parts[len(parts)-2]
	}
	return member, receiver
}

func (v *pathContractEvidenceVisitor) Visit(node ast.Node) ast.Visitor {
	switch current := node.(type) {
	case *ast.CallExpr:
		v.recordPathmatrixCall(current)
	case *ast.Ident:
		v.recordIdentifier(current)
	case *ast.SelectorExpr:
		if current.Sel.Name == v.member {
			v.memberReference = true
		}
	}
	return v
}

func (v *pathContractEvidenceVisitor) recordPathmatrixCall(call *ast.CallExpr) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "pathmatrix" {
		return
	}
	switch selector.Sel.Name {
	case "Validator", "Resolver", "VolumeMount":
		v.matrixCall = true
	}
}

func (v *pathContractEvidenceVisitor) recordIdentifier(identifier *ast.Ident) {
	if identifier.Name == v.member {
		v.memberReference = true
	}
	if identifier.Name == v.receiver {
		v.receiverReference = true
	}
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
