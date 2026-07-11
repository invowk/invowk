// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestArtifactEntryLimitValidate(t *testing.T) {
	t.Parallel()

	if err := ArtifactEntryLimit(1).Validate(); err != nil {
		t.Fatalf("ArtifactEntryLimit(1).Validate() error = %v", err)
	}
	if err := ArtifactEntryLimit(0).Validate(); !errors.Is(err, ErrInvalidArtifactEntryLimit) {
		t.Fatalf("ArtifactEntryLimit(0).Validate() error = %v, want ErrInvalidArtifactEntryLimit", err)
	}
}

func TestLuaArtifactBudgetExactBoundaryAndOverflow(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	writeAuditArtifact(t, filepath.Join(moduleDir, "script.lua"))
	module := &ScannedModule{Path: types.FilesystemPath(moduleDir)}

	exact := newArtifactEntryBudget(2)
	refs, err := appendLuaFilesFromModule(t.Context(), nil, module, &exact)
	if err != nil {
		t.Fatalf("appendLuaFilesFromModule() exact boundary error = %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("appendLuaFilesFromModule() refs = %d, want 1", len(refs))
	}

	overflow := newArtifactEntryBudget(1)
	_, err = appendLuaFilesFromModule(t.Context(), nil, module, &overflow)
	assertArtifactLimitError(t, err, ArtifactKindLuaFile, 1)
}

func TestSymlinkArtifactBudgetExactBoundaryAndOverflow(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	writeAuditArtifact(t, filepath.Join(moduleDir, "file.txt"))

	exact := newArtifactEntryBudget(2)
	if _, err := scanModuleSymlinksWithBudget(t.Context(), types.FilesystemPath(moduleDir), &exact); err != nil {
		t.Fatalf("scanModuleSymlinksWithBudget() exact boundary error = %v", err)
	}

	overflow := newArtifactEntryBudget(1)
	_, err := scanModuleSymlinksWithBudget(t.Context(), types.FilesystemPath(moduleDir), &overflow)
	assertArtifactLimitError(t, err, ArtifactKindSymlink, 1)
}

func TestArtifactBudgetsAreScanWideAndIndependent(t *testing.T) {
	t.Parallel()

	firstDir := t.TempDir()
	secondDir := t.TempDir()
	first := &ScannedModule{Path: types.FilesystemPath(firstDir)}
	second := &ScannedModule{Path: types.FilesystemPath(secondDir)}

	luaBudget := newArtifactEntryBudget(1)
	if _, err := appendLuaFilesFromModule(t.Context(), nil, first, &luaBudget); err != nil {
		t.Fatalf("first Lua walk error = %v", err)
	}
	_, err := appendLuaFilesFromModule(t.Context(), nil, second, &luaBudget)
	assertArtifactLimitError(t, err, ArtifactKindLuaFile, 1)

	symlinkBudget := newArtifactEntryBudget(1)
	if _, err := scanModuleSymlinksWithBudget(t.Context(), types.FilesystemPath(firstDir), &symlinkBudget); err != nil {
		t.Fatalf("independent symlink walk error = %v", err)
	}
}

func TestBuildScriptRefsUsesUnscopedBudgetForRealModule(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	writeAuditArtifact(t, filepath.Join(moduleDir, "script.lua"))
	refs, err := buildScriptRefs(t.Context(), nil, []*ScannedModule{{
		Path: types.FilesystemPath(moduleDir),
	}})
	if err != nil {
		t.Fatalf("buildScriptRefs() error = %v", err)
	}
	if len(refs) != 1 || refs[0].ScriptPath == "" {
		t.Fatalf("buildScriptRefs() refs = %+v, want one real Lua file", refs)
	}
}

func TestBuildScanContextFailsClosedOnArtifactBudget(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "io.example.limit.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.limit", "limited")

	sc, err := buildScanContext(t.Context(), types.FilesystemPath(moduleDir), nil, false, 1)
	if sc != nil {
		t.Fatalf("buildScanContext() context = %#v, want nil", sc)
	}
	if !errors.Is(err, ErrScanContextBuild) {
		t.Fatalf("buildScanContext() error = %v, want ErrScanContextBuild", err)
	}
	assertArtifactLimitError(t, err, ArtifactKindSymlink, 1)
}

func TestDirectoryModuleLoadFailsClosedOnArtifactBudget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	moduleDir := filepath.Join(root, "io.example.directorylimit.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.directorylimit", "limited")

	sc := newScanContext(types.FilesystemPath(root), 1)
	err := sc.loadDirectoryModules(t.Context(), types.FilesystemPath(root))
	assertArtifactLimitError(t, err, ArtifactKindSymlink, 1)
	if len(sc.Diagnostics()) != 0 {
		t.Fatalf("loadDirectoryModules() diagnostics = %v, want fail-closed error without skip diagnostic", sc.Diagnostics())
	}
}

func TestDiscoveryModuleLoadFailsClosedOnArtifactBudget(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "io.example.discoverylimit.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.discoverylimit", "limited")
	module, err := invowkmod.Load(types.FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("Load(module) error = %v", err)
	}

	sc := newScanContext(types.FilesystemPath(moduleDir), 1)
	err = sc.mergeDiscoveryResults(t.Context(), []*discovery.DiscoveredFile{{Module: module}})
	assertArtifactLimitError(t, err, ArtifactKindSymlink, 1)
	if len(sc.Diagnostics()) != 0 {
		t.Fatalf("mergeDiscoveryResults() diagnostics = %v, want fail-closed error without skip diagnostic", sc.Diagnostics())
	}
}

func TestVendoredModuleLoadFailsClosedOnArtifactBudget(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "io.example.vendoredlimit.invowkmod")
	createAuditTestModule(t, moduleDir, "io.example.vendoredlimit", "limited")
	module, err := invowkmod.Load(types.FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("Load(module) error = %v", err)
	}

	sc := newScanContext(types.FilesystemPath(moduleDir), 1)
	err = sc.appendVendoredScannedModules(t.Context(), vendoredModuleArtifacts{{
		Path:   types.FilesystemPath(moduleDir),
		Module: module,
	}}, false)
	assertArtifactLimitError(t, err, ArtifactKindSymlink, 1)
	if len(sc.Diagnostics()) != 0 {
		t.Fatalf("appendVendoredScannedModules() diagnostics = %v, want fail-closed error without skip diagnostic", sc.Diagnostics())
	}
}

func TestArtifactWalkCancellationTakesPrecedenceOverExhaustedBudget(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	module := &ScannedModule{Path: types.FilesystemPath(moduleDir)}
	budget := newArtifactEntryBudget(1)
	budget.visited = artifactEntryCount(budget.limit)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := appendLuaFilesFromModule(ctx, nil, module, &budget)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("appendLuaFilesFromModule() error = %v, want context.Canceled", err)
	}
	if errors.Is(err, ErrArtifactEntryLimitExceeded) {
		t.Fatalf("appendLuaFilesFromModule() error = %v, budget masked cancellation", err)
	}
}

func assertArtifactLimitError(t *testing.T, err error, wantKind ArtifactKind, wantLimit ArtifactEntryLimit) {
	t.Helper()

	if !errors.Is(err, ErrArtifactEntryLimitExceeded) {
		t.Fatalf("error = %v, want ErrArtifactEntryLimitExceeded", err)
	}
	limitErr, ok := errors.AsType[*ArtifactEntryLimitError](err)
	if !ok {
		t.Fatalf("error = %T, want *ArtifactEntryLimitError", err)
	}
	if limitErr.Kind != wantKind || limitErr.Limit != wantLimit || limitErr.Path == nil || *limitErr.Path == "" {
		t.Fatalf("ArtifactEntryLimitError = %+v, want kind=%s limit=%s and non-empty path", limitErr, wantKind, wantLimit)
	}
}

func writeAuditArtifact(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("artifact"), 0o600); err != nil {
		t.Fatalf("write artifact %s: %v", path, err)
	}
}
