// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestGitTypesMutationInvalidValuePayloads(t *testing.T) {
	t.Parallel()

	url := GitURL("http://example.com/repo.git")
	urlErr := url.Validate()
	if !errors.Is(urlErr, ErrInvalidGitURL) {
		t.Fatalf("GitURL(%q).Validate() error = %v, want ErrInvalidGitURL", url, urlErr)
	}
	var invalidURL *InvalidGitURLError
	if !errors.As(urlErr, &invalidURL) {
		t.Fatalf("GitURL(%q).Validate() error type = %T, want *InvalidGitURLError", url, urlErr)
	}
	if invalidURL.Value != url {
		t.Fatalf("InvalidGitURLError.Value = %q, want %q", invalidURL.Value, url)
	}

	commit := GitCommit("ABC123DEF456789012345678901234567890ABCD")
	commitErr := commit.Validate()
	if !errors.Is(commitErr, ErrInvalidGitCommit) {
		t.Fatalf("GitCommit(%q).Validate() error = %v, want ErrInvalidGitCommit", commit, commitErr)
	}
	var invalidCommit *InvalidGitCommitError
	if !errors.As(commitErr, &invalidCommit) {
		t.Fatalf("GitCommit(%q).Validate() error type = %T, want *InvalidGitCommitError", commit, commitErr)
	}
	if invalidCommit.Value != commit {
		t.Fatalf("InvalidGitCommitError.Value = %q, want %q", invalidCommit.Value, commit)
	}
}
