// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/types"
)

const (
	artifactReadBatchSize artifactReadBatchCount = 256
	artifactUnknownValue                         = "unknown"

	artifactWalkContinue    artifactWalkAction = 0
	artifactWalkSkipSubtree artifactWalkAction = 1
)

type (
	artifactWalkAction int

	artifactWalkVisitor func(context.Context, types.FilesystemPath, fs.DirEntry) (artifactWalkAction, error)

	artifactDirectoryReader interface {
		ReadDir(artifactReadBatchCount) ([]fs.DirEntry, error)
		Close() error
	}

	artifactDirectoryOpener func(types.FilesystemPath) (artifactDirectoryReader, error)

	osArtifactDirectory struct {
		file *os.File
	}
)

func (a artifactWalkAction) String() string {
	switch a {
	case artifactWalkContinue:
		return "continue"
	case artifactWalkSkipSubtree:
		return "skip_subtree"
	default:
		return artifactUnknownValue
	}
}

func (a artifactWalkAction) Validate() error {
	switch a {
	case artifactWalkContinue, artifactWalkSkipSubtree:
		return nil
	default:
		return fmt.Errorf("invalid artifact walk action %s", a)
	}
}

func walkArtifactTree(
	ctx context.Context,
	root types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
) error {
	return walkArtifactTreeWithOpener(ctx, root, kind, budget, visitor, openArtifactDirectory)
}

func walkArtifactTreeWithOpener(
	ctx context.Context,
	root types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
	opener artifactDirectoryOpener,
) error {
	if err := validateArtifactWalk(ctx, root, kind, budget, visitor, opener); err != nil {
		return err
	}
	descend, err := visitArtifactRoot(ctx, root, kind, budget, visitor)
	if err != nil || !descend {
		return err
	}
	return walkArtifactDirectories(ctx, root, kind, budget, visitor, opener)
}

func validateArtifactWalk(
	ctx context.Context,
	root types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
	opener artifactDirectoryOpener,
) error {
	if err := scanContextErr(ctx); err != nil {
		return err
	}
	if budget == nil {
		return fmt.Errorf("walking %s artifacts: nil budget", kind)
	}
	if err := errors.Join(root.Validate(), kind.Validate(), budget.Validate()); err != nil {
		return fmt.Errorf("validating %s artifact walk: %w", kind, err)
	}
	if visitor == nil {
		return fmt.Errorf("walking %s artifacts: nil visitor", kind)
	}
	if opener == nil {
		return fmt.Errorf("walking %s artifacts: nil directory opener", kind)
	}
	return nil
}

func visitArtifactRoot(
	ctx context.Context,
	root types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
) (bool, error) {
	rootInfo, err := os.Lstat(root.String())
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		return false, ctxErr
	}
	if err != nil {
		return false, fmt.Errorf("inspecting artifact root %s: %w", root, err)
	}
	rootEntry := fs.FileInfoToDirEntry(rootInfo)
	if consumeErr := budget.consume(kind, root); consumeErr != nil {
		return false, consumeErr
	}
	action, err := visitor(ctx, root, rootEntry)
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		return false, ctxErr
	}
	if err != nil {
		return false, err
	}
	if err := action.Validate(); err != nil {
		return false, err
	}
	return rootEntry.IsDir() && action != artifactWalkSkipSubtree, nil
}

func walkArtifactDirectories(
	ctx context.Context,
	root types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
	opener artifactDirectoryOpener,
) error {
	pending := []types.FilesystemPath{root}
	for len(pending) > 0 {
		if err := scanContextErr(ctx); err != nil {
			return err
		}
		last := len(pending) - 1
		directory := pending[last]
		pending = pending[:last]

		children, err := readArtifactDirectory(ctx, directory, kind, budget, visitor, opener)
		if err != nil {
			return err
		}
		pending = append(pending, children...)
	}
	return nil
}

func readArtifactDirectory(
	ctx context.Context,
	directory types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
	opener artifactDirectoryOpener,
) ([]types.FilesystemPath, error) {
	reader, err := openArtifactDirectoryForWalk(ctx, directory, opener)
	if err != nil {
		return nil, err
	}
	children, walkErr := readArtifactDirectoryEntries(ctx, directory, kind, budget, visitor, reader)
	return finishArtifactDirectory(ctx, directory, reader, children, walkErr)
}

func openArtifactDirectoryForWalk(
	ctx context.Context,
	directory types.FilesystemPath,
	opener artifactDirectoryOpener,
) (artifactDirectoryReader, error) {
	reader, err := opener(directory)
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		if reader != nil {
			if reader.Close() != nil {
				return nil, ctxErr
			}
		}
		return nil, ctxErr
	}
	if err != nil {
		if reader != nil {
			err = errors.Join(err, reader.Close())
		}
		return nil, fmt.Errorf("opening artifact directory %s: %w", directory, err)
	}
	if reader == nil {
		return nil, fmt.Errorf("opening artifact directory %s: nil reader", directory)
	}
	return reader, nil
}

func readArtifactDirectoryEntries(
	ctx context.Context,
	directory types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
	reader artifactDirectoryReader,
) ([]types.FilesystemPath, error) {
	var children []types.FilesystemPath
	for {
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		entries, readErr := reader.ReadDir(budget.nextReadSize())
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		var visitErr error
		children, visitErr = visitArtifactDirectoryEntries(ctx, directory, kind, budget, visitor, children, entries)
		if visitErr != nil {
			return nil, visitErr
		}
		switch {
		case errors.Is(readErr, io.EOF):
			return children, nil
		case readErr != nil:
			return nil, fmt.Errorf("reading artifact directory %s: %w", directory, readErr)
		case len(entries) == 0:
			return nil, fmt.Errorf("reading artifact directory %s: empty batch without EOF", directory)
		}
	}
}

func visitArtifactDirectoryEntries(
	ctx context.Context,
	directory types.FilesystemPath,
	kind ArtifactKind,
	budget *artifactEntryBudget,
	visitor artifactWalkVisitor,
	children []types.FilesystemPath,
	entries []fs.DirEntry,
) ([]types.FilesystemPath, error) {
	for _, entry := range entries {
		entryPath := filesystemPathFromWalk(filepath.Join(directory.String(), entry.Name()))
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		if consumeErr := budget.consume(kind, entryPath); consumeErr != nil {
			return nil, consumeErr
		}
		action, visitErr := visitor(ctx, entryPath, entry)
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		if visitErr != nil {
			return nil, visitErr
		}
		if actionErr := action.Validate(); actionErr != nil {
			return nil, actionErr
		}
		if entry.IsDir() && action != artifactWalkSkipSubtree {
			children = append(children, entryPath)
		}
	}
	return children, nil
}

func finishArtifactDirectory(
	ctx context.Context,
	directory types.FilesystemPath,
	reader artifactDirectoryReader,
	children []types.FilesystemPath,
	walkErr error,
) ([]types.FilesystemPath, error) {
	closeErr := reader.Close()
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err := errors.Join(walkErr, closeErr); err != nil {
		return nil, fmt.Errorf("walking artifact directory %s: %w", directory, err)
	}
	return children, nil
}

func openArtifactDirectory(path types.FilesystemPath) (artifactDirectoryReader, error) {
	file, err := os.Open(path.String())
	if err != nil {
		return nil, fmt.Errorf("open directory: %w", err)
	}
	return &osArtifactDirectory{file: file}, nil
}

func (d *osArtifactDirectory) ReadDir(count artifactReadBatchCount) ([]fs.DirEntry, error) {
	entries, err := d.file.ReadDir(int(count))
	if err != nil {
		return entries, fmt.Errorf("read directory batch: %w", err)
	}
	return entries, nil
}

func (d *osArtifactDirectory) Close() error {
	if err := d.file.Close(); err != nil {
		return fmt.Errorf("close directory: %w", err)
	}
	return nil
}
