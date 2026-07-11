// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

var (
	errArtifactReaderFailure = errors.New("artifact reader failure")
	errArtifactCloseFailure  = errors.New("artifact close failure")
	errArtifactInfoMissing   = errors.New("artifact entry info unavailable")
)

type (
	fakeArtifactDirEntry struct {
		name string
		dir  bool
	}

	recordingArtifactDirectory struct {
		entries      []fs.DirEntry
		requests     []artifactReadBatchCount
		offset       int
		readErr      error
		closeErr     error
		cancelOnRead context.CancelFunc
	}
)

func TestArtifactWalkerBoundsSingleDirectoryReads(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	entries := make([]fs.DirEntry, artifactReadBatchSize*3)
	for i := range entries {
		entries[i] = fakeArtifactDirEntry{name: "entry-" + strconv.Itoa(i)}
	}
	reader := &recordingArtifactDirectory{entries: entries}
	budget := newScopedArtifactEntryBudget(5, root)
	err := walkArtifactTreeWithOpener(
		t.Context(),
		root,
		ArtifactKindLuaFile,
		&budget,
		func(context.Context, types.FilesystemPath, fs.DirEntry) (artifactWalkAction, error) {
			return artifactWalkContinue, nil
		},
		func(types.FilesystemPath) (artifactDirectoryReader, error) { return reader, nil },
	)
	assertArtifactLimitError(t, err, ArtifactKindLuaFile, 5)
	limitErr, ok := errors.AsType[*ArtifactEntryLimitError](err)
	if !ok || limitErr.Path == nil || *limitErr.Path != root {
		t.Fatalf("ArtifactEntryLimitError path = %v, want stable scope %s", limitErr, root)
	}
	if budget.visited != artifactEntryCount(budget.limit) {
		t.Fatalf("visited = %s, want limit %s", budget.visited, budget.limit)
	}
	if !slices.Equal(reader.requests, []artifactReadBatchCount{5}) {
		t.Fatalf("ReadDir requests = %v, want one remaining-plus-one bounded read", reader.requests)
	}
}

func TestArtifactWalkerCancellationBetweenBatchesTakesPrecedence(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	entries := make([]fs.DirEntry, artifactReadBatchSize+1)
	for i := range entries {
		entries[i] = fakeArtifactDirEntry{name: "entry"}
	}
	reader := &recordingArtifactDirectory{entries: entries, cancelOnRead: cancel}
	budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, root)
	err := walkArtifactTreeWithOpener(
		ctx,
		root,
		ArtifactKindLuaFile,
		&budget,
		func(context.Context, types.FilesystemPath, fs.DirEntry) (artifactWalkAction, error) {
			return artifactWalkContinue, nil
		},
		func(types.FilesystemPath) (artifactDirectoryReader, error) { return reader, nil },
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("walkArtifactTreeWithOpener() error = %v, want context.Canceled", err)
	}
}

func TestArtifactWalkerFailsClosedOnReadAndCloseErrors(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	tests := []struct {
		name   string
		reader *recordingArtifactDirectory
		want   error
	}{
		{name: "read", reader: &recordingArtifactDirectory{readErr: errArtifactReaderFailure}, want: errArtifactReaderFailure},
		{name: "close", reader: &recordingArtifactDirectory{closeErr: errArtifactCloseFailure}, want: errArtifactCloseFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, root)
			err := walkArtifactTreeWithOpener(
				t.Context(), root, ArtifactKindSymlink, &budget,
				func(context.Context, types.FilesystemPath, fs.DirEntry) (artifactWalkAction, error) {
					return artifactWalkContinue, nil
				},
				func(types.FilesystemPath) (artifactDirectoryReader, error) { return tt.reader, nil },
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("walkArtifactTreeWithOpener() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLuaArtifactWalkerSkipsVendoredSubtreeWithinBudget(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	vendorDir := filepath.Join(moduleDir, invowkmod.VendoredModulesDir)
	if err := os.Mkdir(vendorDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.lua", "two.lua", "three.lua"} {
		writeAuditArtifact(t, filepath.Join(vendorDir, name))
	}
	budget := newScopedArtifactEntryBudget(2, types.FilesystemPath(moduleDir))
	refs, err := appendLuaFilesFromModule(
		t.Context(), nil, &ScannedModule{Path: types.FilesystemPath(moduleDir)}, &budget,
	)
	if err != nil {
		t.Fatalf("appendLuaFilesFromModule() error = %v", err)
	}
	if len(refs) != 0 || budget.visited != 2 {
		t.Fatalf("appendLuaFilesFromModule() refs=%d visited=%s, want 0 refs and 2 visited", len(refs), budget.visited)
	}
}

func TestArtifactConsumersReturnDeterministicSortedOutput(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	for _, name := range []string{"z.lua", "a.lua", "m.lua"} {
		writeAuditArtifact(t, filepath.Join(moduleDir, name))
	}
	module := &ScannedModule{Path: types.FilesystemPath(moduleDir)}
	budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, module.Path)
	refs, err := appendLuaFilesFromModule(t.Context(), nil, module, &budget)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(refs))
	for i := range refs {
		paths[i] = refs[i].ScriptPath.String()
	}
	if !slices.IsSorted(paths) {
		t.Fatalf("Lua paths are not sorted: %v", paths)
	}
}

func TestArtifactWalkerReportsButDoesNotFollowSymlinkDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires elevated privileges on Windows")
	}

	moduleDir := t.TempDir()
	targetDir := t.TempDir()
	writeAuditArtifact(t, filepath.Join(targetDir, "hidden.lua"))
	linkPath := filepath.Join(moduleDir, "linked-directory")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Fatal(err)
	}

	module := &ScannedModule{Path: types.FilesystemPath(moduleDir)}
	luaBudget := newScopedArtifactEntryBudget(2, module.Path)
	refs, err := appendLuaFilesFromModule(t.Context(), nil, module, &luaBudget)
	if err != nil {
		t.Fatalf("appendLuaFilesFromModule() error = %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("appendLuaFilesFromModule() refs = %v, want symlink target excluded", refs)
	}

	symlinkBudget := newScopedArtifactEntryBudget(2, module.Path)
	symlinks, err := scanModuleSymlinksWithBudget(t.Context(), module.Path, &symlinkBudget)
	if err != nil {
		t.Fatalf("scanModuleSymlinksWithBudget() error = %v", err)
	}
	if len(symlinks) != 1 || symlinks[0].Path.String() != linkPath {
		t.Fatalf("symlinks = %+v, want only %s", symlinks, linkPath)
	}
}

func (e fakeArtifactDirEntry) Name() string { return e.name }
func (e fakeArtifactDirEntry) IsDir() bool  { return e.dir }
func (e fakeArtifactDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}
func (e fakeArtifactDirEntry) Info() (fs.FileInfo, error) { return nil, errArtifactInfoMissing }

func (r *recordingArtifactDirectory) ReadDir(count artifactReadBatchCount) ([]fs.DirEntry, error) {
	r.requests = append(r.requests, count)
	if r.cancelOnRead != nil {
		r.cancelOnRead()
		r.cancelOnRead = nil
	}
	if r.offset >= len(r.entries) {
		if r.readErr != nil {
			return nil, r.readErr
		}
		return nil, io.EOF
	}
	end := min(r.offset+int(count), len(r.entries))
	batch := r.entries[r.offset:end]
	r.offset = end
	if r.offset == len(r.entries) {
		if r.readErr != nil {
			return batch, r.readErr
		}
		return batch, io.EOF
	}
	return batch, nil
}

func (r *recordingArtifactDirectory) Close() error { return r.closeErr }
