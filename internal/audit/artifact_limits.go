// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// DefaultArtifactEntryLimit bounds each artifact traversal class across one scan.
	DefaultArtifactEntryLimit ArtifactEntryLimit = 100_000

	// ArtifactKindLuaFile identifies traversal used to discover Lua source files.
	ArtifactKindLuaFile ArtifactKind = "lua_file"
	// ArtifactKindSymlink identifies traversal used to inventory symbolic links.
	ArtifactKindSymlink ArtifactKind = "symlink"
)

type (
	// ArtifactEntryLimit is the maximum number of filesystem entries visited for
	// each artifact class across one audit scan.
	ArtifactEntryLimit int

	// ArtifactKind identifies the filesystem artifact class whose traversal is bounded.
	ArtifactKind string

	// artifactEntryCount tracks a non-negative number of visited filesystem entries.
	artifactEntryCount int

	// artifactReadBatchCount bounds one directory read without exposing a primitive in scanner policy.
	artifactReadBatchCount int

	// artifactEntryBudget tracks build-time traversal consumption for one artifact class.
	artifactEntryBudget struct {
		limit   ArtifactEntryLimit
		visited artifactEntryCount
		scope   *types.FilesystemPath
	}
)

// String returns the decimal representation of the entry limit.
func (l ArtifactEntryLimit) String() string { return strconv.Itoa(int(l)) }

// Validate returns nil when the entry limit is positive.
func (l ArtifactEntryLimit) Validate() error {
	if l <= 0 {
		return fmt.Errorf("%w: %s", ErrInvalidArtifactEntryLimit, l)
	}
	return nil
}

// String returns the stable artifact-kind identifier.
func (k ArtifactKind) String() string { return string(k) }

// Validate returns nil for a recognized artifact kind.
func (k ArtifactKind) Validate() error {
	switch k {
	case ArtifactKindLuaFile, ArtifactKindSymlink:
		return nil
	default:
		return fmt.Errorf("invalid artifact kind %q", k)
	}
}

// String returns the decimal representation of the entry count.
func (c artifactEntryCount) String() string { return strconv.Itoa(int(c)) }

// Validate returns nil when the entry count is non-negative.
func (c artifactEntryCount) Validate() error {
	if c < 0 {
		return fmt.Errorf("invalid artifact entry count %s", c)
	}
	return nil
}

// String returns the decimal representation of the directory read batch count.
func (c artifactReadBatchCount) String() string { return strconv.Itoa(int(c)) }

// Validate returns nil when the directory read batch count is positive.
func (c artifactReadBatchCount) Validate() error {
	if c <= 0 {
		return fmt.Errorf("invalid artifact read batch count %s", c)
	}
	return nil
}

// Validate returns nil when the budget limit and current consumption are valid.
func (b *artifactEntryBudget) Validate() error {
	if b == nil {
		return errors.New("nil artifact entry budget")
	}
	var scopeErr error
	if b.scope != nil {
		scopeErr = b.scope.Validate()
	}
	return errors.Join(b.limit.Validate(), b.visited.Validate(), scopeErr)
}

func newArtifactEntryBudget(limit ArtifactEntryLimit) artifactEntryBudget {
	return artifactEntryBudget{limit: limit}
}

func newScopedArtifactEntryBudget(limit ArtifactEntryLimit, scope types.FilesystemPath) artifactEntryBudget {
	return artifactEntryBudget{limit: limit, scope: &scope}
}

func (b *artifactEntryBudget) consume(kind ArtifactKind, path types.FilesystemPath) error {
	if b == nil {
		return fmt.Errorf("consuming %s artifact entry budget: nil budget", kind)
	}
	if err := errors.Join(kind.Validate(), path.Validate(), b.Validate()); err != nil {
		return fmt.Errorf("validating %s artifact entry budget: %w", kind, err)
	}
	if b.visited >= artifactEntryCount(b.limit) {
		limitPath := path
		if b.scope != nil {
			limitPath = *b.scope
		}
		return &ArtifactEntryLimitError{
			Kind:  kind,
			Path:  &limitPath,
			Limit: b.limit,
		}
	}
	b.visited++
	return nil
}

func (b *artifactEntryBudget) nextReadSize() artifactReadBatchCount {
	remaining := artifactEntryCount(b.limit) - b.visited
	if remaining >= artifactEntryCount(artifactReadBatchSize) {
		return artifactReadBatchSize
	}
	if remaining < 0 {
		return 1
	}
	// Read one entry beyond the remaining allowance so an exact-boundary
	// directory can be distinguished from an exhausted traversal.
	return artifactReadBatchCount(remaining) + 1
}

func newScanContext(scanPath types.FilesystemPath, limit ArtifactEntryLimit) *ScanContext {
	luaBudget := newArtifactEntryBudget(limit)
	symlinkBudget := newArtifactEntryBudget(limit)
	if scanPath != "" {
		luaBudget = newScopedArtifactEntryBudget(limit, scanPath)
		symlinkBudget = newScopedArtifactEntryBudget(limit, scanPath)
	}
	return &ScanContext{
		rootPath:      scanPath,
		luaFileBudget: luaBudget,
		symlinkBudget: symlinkBudget,
	}
}
