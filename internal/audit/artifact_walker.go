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

	rootInfo, err := os.Lstat(root.String())
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return fmt.Errorf("inspecting artifact root %s: %w", root, err)
	}
	rootEntry := fs.FileInfoToDirEntry(rootInfo)
	if consumeErr := budget.consume(kind, root); consumeErr != nil {
		return consumeErr
	}
	action, err := visitor(ctx, root, rootEntry)
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	if err := action.Validate(); err != nil {
		return err
	}
	if !rootEntry.IsDir() || action == artifactWalkSkipSubtree {
		return nil
	}

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
	reader, err := opener(directory)
	if ctxErr := scanContextErr(ctx); ctxErr != nil {
		if reader != nil {
			closeErr := reader.Close()
			if closeErr != nil {
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

	var children []types.FilesystemPath
	var walkErr error
	for walkErr == nil {
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			walkErr = ctxErr
			break
		}
		entries, readErr := reader.ReadDir(budget.nextReadSize())
		if ctxErr := scanContextErr(ctx); ctxErr != nil {
			walkErr = ctxErr
			break
		}
		for _, entry := range entries {
			entryPath := filesystemPathFromWalk(filepath.Join(directory.String(), entry.Name()))
			if ctxErr := scanContextErr(ctx); ctxErr != nil {
				walkErr = ctxErr
				break
			}
			if consumeErr := budget.consume(kind, entryPath); consumeErr != nil {
				walkErr = consumeErr
				break
			}
			action, visitErr := visitor(ctx, entryPath, entry)
			if ctxErr := scanContextErr(ctx); ctxErr != nil {
				walkErr = ctxErr
				break
			}
			if visitErr != nil {
				walkErr = visitErr
				break
			}
			if actionErr := action.Validate(); actionErr != nil {
				walkErr = actionErr
				break
			}
			if entry.IsDir() && action != artifactWalkSkipSubtree {
				children = append(children, entryPath)
			}
		}
		if walkErr != nil {
			break
		}
		switch {
		case errors.Is(readErr, io.EOF):
			walkErr = nil
			goto closeDirectory
		case readErr != nil:
			walkErr = fmt.Errorf("reading artifact directory %s: %w", directory, readErr)
		case len(entries) == 0:
			walkErr = fmt.Errorf("reading artifact directory %s: empty batch without EOF", directory)
		}
	}

closeDirectory:
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
