// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestGitURL_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     GitURL
		want    bool
		wantErr bool
	}{
		{"https", GitURL("https://github.com/user/repo.git"), true, false},
		{"git_at", GitURL("git@github.com:user/repo.git"), true, false},
		{"ssh", GitURL("ssh://git@github.com/user/repo"), true, false},
		{"empty", GitURL(""), false, true},
		{"http", GitURL("http://example.com/repo.git"), false, true},
		{"ftp", GitURL("ftp://example.com/repo.git"), false, true},
		{"plain_string", GitURL("not-a-url"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.url.Validate()
			if (err == nil) != tt.want {
				t.Errorf("GitURL(%q).Validate() error = %v, wantValid %v", tt.url, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("GitURL(%q).Validate() returned nil, want error", tt.url)
				}
				if !errors.Is(err, ErrInvalidGitURL) {
					t.Errorf("error should wrap ErrInvalidGitURL, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("GitURL(%q).Validate() returned unexpected error: %v", tt.url, err)
			}
		})
	}
}

func TestGitURL_String(t *testing.T) {
	t.Parallel()
	u := GitURL("https://github.com/user/repo.git")
	if u.String() != "https://github.com/user/repo.git" {
		t.Errorf("GitURL.String() = %q, want %q", u.String(), "https://github.com/user/repo.git")
	}
}

func TestGitCommit_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		commit  GitCommit
		want    bool
		wantErr bool
	}{
		{"valid_sha", GitCommit("abc123def456789012345678901234567890abcd"), true, false},
		{"all_zeros", GitCommit("0000000000000000000000000000000000000000"), true, false},
		{"empty", GitCommit(""), false, true},
		{"too_short", GitCommit("abc123"), false, true},
		{"uppercase", GitCommit("ABC123DEF456789012345678901234567890ABCD"), false, true},
		{"non_hex", GitCommit("xyz123def456789012345678901234567890abcd"), false, true},
		{"39_chars", GitCommit("abc123def456789012345678901234567890abc"), false, true},
		{"41_chars", GitCommit("abc123def456789012345678901234567890abcde"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.commit.Validate()
			if (err == nil) != tt.want {
				t.Errorf("GitCommit(%q).Validate() error = %v, wantValid %v", tt.commit, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("GitCommit(%q).Validate() returned nil, want error", tt.commit)
				}
				if !errors.Is(err, ErrInvalidGitCommit) {
					t.Errorf("error should wrap ErrInvalidGitCommit, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("GitCommit(%q).Validate() returned unexpected error: %v", tt.commit, err)
			}
		})
	}
}

func TestGitCommit_String(t *testing.T) {
	t.Parallel()
	c := GitCommit("abc123def456789012345678901234567890abcd")
	if c.String() != "abc123def456789012345678901234567890abcd" {
		t.Errorf("GitCommit.String() = %q, want expected value", c.String())
	}
}
