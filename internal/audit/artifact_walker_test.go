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
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

var (
	errArtifactReaderFailure = errors.New("artifact reader failure")
	errArtifactCloseFailure  = errors.New("artifact close failure")
	errArtifactInfoMissing   = errors.New("artifact entry info unavailable")
	errArtifactOpenFailure   = errors.New("artifact open failure")
	errArtifactVisitFailure  = errors.New("artifact visit failure")
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
		emptyBatch   bool
		closed       bool
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
		wants  []error
	}{
		{name: "read", reader: &recordingArtifactDirectory{readErr: errArtifactReaderFailure}, wants: []error{errArtifactReaderFailure}},
		{name: "close", reader: &recordingArtifactDirectory{closeErr: errArtifactCloseFailure}, wants: []error{errArtifactCloseFailure}},
		{
			name:   "read and close",
			reader: &recordingArtifactDirectory{readErr: errArtifactReaderFailure, closeErr: errArtifactCloseFailure},
			wants:  []error{errArtifactReaderFailure, errArtifactCloseFailure},
		},
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
			for _, want := range tt.wants {
				if !errors.Is(err, want) {
					t.Fatalf("walkArtifactTreeWithOpener() error = %v, want %v", err, want)
				}
			}
			if !tt.reader.closed {
				t.Fatal("walkArtifactTreeWithOpener() did not close the directory reader")
			}
		})
	}
}

func TestArtifactWalkerClosesReaderReturnedWithOpenerError(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	reader := &recordingArtifactDirectory{closeErr: errArtifactCloseFailure}
	budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, root)
	err := walkArtifactTreeWithOpener(
		t.Context(), root, ArtifactKindSymlink, &budget, continueArtifactWalk,
		func(types.FilesystemPath) (artifactDirectoryReader, error) {
			return reader, errArtifactOpenFailure
		},
	)
	for _, want := range []error{errArtifactOpenFailure, errArtifactCloseFailure} {
		if !errors.Is(err, want) {
			t.Fatalf("walkArtifactTreeWithOpener() error = %v, want %v", err, want)
		}
	}
	if !reader.closed {
		t.Fatal("walkArtifactTreeWithOpener() did not close the reader returned with an opener error")
	}
}

func TestArtifactWalkerCancellationAfterOpenClosesReaderAndTakesPrecedence(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	reader := &recordingArtifactDirectory{closeErr: errArtifactCloseFailure}
	budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, root)
	err := walkArtifactTreeWithOpener(
		ctx, root, ArtifactKindSymlink, &budget, continueArtifactWalk,
		func(types.FilesystemPath) (artifactDirectoryReader, error) {
			cancel()
			return reader, errArtifactOpenFailure
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("walkArtifactTreeWithOpener() error = %v, want context.Canceled", err)
	}
	if errors.Is(err, errArtifactOpenFailure) || errors.Is(err, errArtifactCloseFailure) {
		t.Fatalf("walkArtifactTreeWithOpener() error = %v, cancellation must take precedence", err)
	}
	if !reader.closed {
		t.Fatal("walkArtifactTreeWithOpener() did not close the reader after cancellation")
	}
}

func TestArtifactWalkerRejectsInvalidDirectoryReaderResults(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	tests := []struct {
		name     string
		opener   artifactDirectoryOpener
		wantText string
	}{
		{
			name: "nil reader",
			opener: func(types.FilesystemPath) (artifactDirectoryReader, error) {
				var openErr error
				return nil, openErr
			},
			wantText: "nil reader",
		},
		{
			name: "empty batch without EOF",
			opener: func(types.FilesystemPath) (artifactDirectoryReader, error) {
				return &recordingArtifactDirectory{emptyBatch: true}, nil
			},
			wantText: "empty batch without EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, root)
			err := walkArtifactTreeWithOpener(t.Context(), root, ArtifactKindLuaFile, &budget, continueArtifactWalk, tt.opener)
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("walkArtifactTreeWithOpener() error = %v, want text %q", err, tt.wantText)
			}
		})
	}
}

func TestArtifactWalkerClosesReaderOnVisitorFailure(t *testing.T) {
	t.Parallel()

	root := types.FilesystemPath(t.TempDir())
	tests := []struct {
		name     string
		action   artifactWalkAction
		err      error
		want     error
		wantText string
	}{
		{name: "visitor error", action: artifactWalkContinue, err: errArtifactVisitFailure, want: errArtifactVisitFailure},
		{name: "invalid action", action: artifactWalkAction(99), wantText: "invalid artifact walk action"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := &recordingArtifactDirectory{entries: []fs.DirEntry{fakeArtifactDirEntry{name: "child"}}}
			budget := newScopedArtifactEntryBudget(DefaultArtifactEntryLimit, root)
			err := walkArtifactTreeWithOpener(
				t.Context(), root, ArtifactKindLuaFile, &budget,
				func(_ context.Context, path types.FilesystemPath, _ fs.DirEntry) (artifactWalkAction, error) {
					if path == root {
						return artifactWalkContinue, nil
					}
					return tt.action, tt.err
				},
				func(types.FilesystemPath) (artifactDirectoryReader, error) { return reader, nil },
			)
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("walkArtifactTreeWithOpener() error = %v, want %v", err, tt.want)
			}
			if tt.wantText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantText)) {
				t.Fatalf("walkArtifactTreeWithOpener() error = %v, want text %q", err, tt.wantText)
			}
			if !reader.closed {
				t.Fatal("walkArtifactTreeWithOpener() did not close the reader after visitor failure")
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
	if r.emptyBatch {
		return nil, nil
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

func (r *recordingArtifactDirectory) Close() error {
	r.closed = true
	return r.closeErr
}

func continueArtifactWalk(context.Context, types.FilesystemPath, fs.DirEntry) (artifactWalkAction, error) {
	return artifactWalkContinue, nil
}
