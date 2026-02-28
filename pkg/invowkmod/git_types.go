// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// ErrInvalidGitURL is the sentinel error wrapped by InvalidGitURLError.
	ErrInvalidGitURL = errors.New("invalid git URL")
	// ErrInvalidGitCommit is the sentinel error wrapped by InvalidGitCommitError.
	ErrInvalidGitCommit = errors.New("invalid git commit")

	// gitCommitPattern validates a 40-character lowercase hex SHA.
	gitCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type (
	// GitURL represents a Git repository URL (HTTPS, SSH, or git@ format).
	// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
	GitURL string

	// InvalidGitURLError is returned when a GitURL value does not match
	// the expected URL format.
	InvalidGitURLError struct {
		Value GitURL
	}

	// GitCommit represents a 40-character lowercase hexadecimal Git commit SHA.
	GitCommit string

	// InvalidGitCommitError is returned when a GitCommit value does not match
	// the expected 40-character lowercase hex format.
	InvalidGitCommitError struct {
		Value GitCommit
	}
)

// Error implements the error interface.
func (e *InvalidGitURLError) Error() string {
	return fmt.Sprintf("invalid git URL %q (must start with https://, git@, or ssh://)", e.Value)
}

// Unwrap returns ErrInvalidGitURL so callers can use errors.Is for programmatic detection.
func (e *InvalidGitURLError) Unwrap() error { return ErrInvalidGitURL }

//goplint:nonzero

// Validate returns nil if the GitURL is a valid Git repository URL,
// or an error describing the validation failure.
func (u GitURL) Validate() error {
	s := string(u)
	if s == "" || (!strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "git@") && !strings.HasPrefix(s, "ssh://")) {
		return &InvalidGitURLError{Value: u}
	}
	return nil
}

// String returns the string representation of the GitURL.
func (u GitURL) String() string { return string(u) }

// Error implements the error interface.
func (e *InvalidGitCommitError) Error() string {
	return fmt.Sprintf("invalid git commit %q (must be a 40-character lowercase hex SHA)", e.Value)
}

// Unwrap returns ErrInvalidGitCommit so callers can use errors.Is for programmatic detection.
func (e *InvalidGitCommitError) Unwrap() error { return ErrInvalidGitCommit }

//goplint:nonzero

// Validate returns nil if the GitCommit is a valid 40-character lowercase hex SHA,
// or an error describing the validation failure.
func (c GitCommit) Validate() error {
	if !gitCommitPattern.MatchString(string(c)) {
		return &InvalidGitCommitError{Value: c}
	}
	return nil
}

// String returns the string representation of the GitCommit.
func (c GitCommit) String() string { return string(c) }
