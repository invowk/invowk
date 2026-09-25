// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

// Exports for the external test package modulesync_test, whose trace harness
// also drives moduleops (which imports modulesync).

const (
	// TraceCommitC1 and TraceCommitC2 are the commits LockIntegrity.tla calls
	// "c1" (the locked, trusted content) and "c2" (a re-pointed tag).
	TraceCommitC1 = defaultFakeCommit
	TraceCommitC2 = integrityTestRepoedCommit
)

var (
	// NewResolverWithFetcher builds a Resolver on a fake fetcher.
	NewResolverWithFetcher = newResolverWithFetcher
	// DowngradeLockToV1 rewrites a lock as a v1.0 file without content hashes.
	DowngradeLockToV1 = downgradeLockToV1
)

// TraceFetcher serves one module tree per commit; Commit is the commit the
// version tag points at, so re-pointing it moves the tag.
type TraceFetcher struct {
	Trees    map[GitCommit]types.FilesystemPath
	Commit   GitCommit
	Versions []SemVer
}

// ListVersions returns the configured version tags.
func (f *TraceFetcher) ListVersions(context.Context, GitURL) ([]SemVer, error) {
	return f.Versions, nil
}

// Fetch returns the tree and commit the tag currently points at.
func (f *TraceFetcher) Fetch(context.Context, GitURL, SemVer) (types.FilesystemPath, GitCommit, error) {
	tree, ok := f.Trees[f.Commit]
	if !ok {
		return "", "", fmt.Errorf("trace fetcher: no tree for commit %s", f.Commit)
	}
	return tree, f.Commit, nil
}
