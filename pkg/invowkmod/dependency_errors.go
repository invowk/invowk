// SPDX-License-Identifier: MPL-2.0

package invowkmod

import "errors"

var (
	// ErrGitURLRequired is returned when a module ref is missing the git_url field.
	ErrGitURLRequired = errors.New("git_url is required")

	// ErrUnsupportedGitURLScheme is returned when a git_url uses an unsupported scheme.
	ErrUnsupportedGitURLScheme = errors.New("git_url must start with https://, git@, or ssh://")

	// ErrVersionRequired is returned when a module ref is missing the version field.
	ErrVersionRequired = errors.New("version is required")
)
