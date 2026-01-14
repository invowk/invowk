// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"strings"
	"testing"
)

func TestValidateRegexPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		shouldError bool
		errorMsg    string
	}{
		// Valid patterns
		{name: "empty pattern", pattern: "", shouldError: false},
		{name: "simple literal", pattern: "hello", shouldError: false},
		{name: "simple character class", pattern: "[a-z]+", shouldError: false},
		{name: "email-like pattern", pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, shouldError: false},
		{name: "simple alternation", pattern: "foo|bar", shouldError: false},
		{name: "simple group", pattern: "(abc)+", shouldError: false},
		{name: "non-overlapping alternation", pattern: "(cat|dog)+", shouldError: false},

		// Dangerous patterns - nested quantifiers
		{name: "nested plus", pattern: "(a+)+", shouldError: true, errorMsg: "nested quantifiers"},
		{name: "nested star", pattern: "(a*)*", shouldError: true, errorMsg: "nested quantifiers"},
		{name: "nested plus-star", pattern: "(a+)*", shouldError: true, errorMsg: "nested quantifiers"},
		{name: "nested word plus", pattern: `(\w+)+`, shouldError: true, errorMsg: "nested quantifiers"},
		{name: "nested dot plus", pattern: "(.+)+", shouldError: true, errorMsg: "nested quantifiers"},

		// Dangerous patterns - overlapping alternation
		{name: "overlapping alternation prefix", pattern: "(a|aa)+", shouldError: true, errorMsg: "overlapping"},
		{name: "self alternation", pattern: "(a|a)+", shouldError: true, errorMsg: "overlapping"},

		// Length limits
		{name: "too long pattern", pattern: strings.Repeat("a", MaxRegexPatternLength+1), shouldError: true, errorMsg: "too long"},

		// Nesting depth
		{name: "excessive nesting", pattern: strings.Repeat("(", MaxNestedGroups+1) + "a" + strings.Repeat(")", MaxNestedGroups+1), shouldError: true, errorMsg: "nested groups"},

		// Excessive quantifiers
		{name: "many quantifiers", pattern: strings.Repeat("a+", MaxQuantifierRepeats+1), shouldError: true, errorMsg: "too many quantifiers"},

		// Invalid regex (should be caught)
		{name: "unclosed group", pattern: "(abc", shouldError: true, errorMsg: "invalid regex"},
		{name: "bad character class", pattern: "[z-a]", shouldError: true, errorMsg: "invalid regex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegexPattern(tt.pattern)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateContainerImage(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		shouldError bool
		errorMsg    string
	}{
		// Valid images
		{name: "empty", image: "", shouldError: false},
		{name: "simple name", image: "nginx", shouldError: false},
		{name: "name with tag", image: "nginx:latest", shouldError: false},
		{name: "name with version tag", image: "nginx:1.21.0", shouldError: false},
		{name: "registry/name", image: "docker.io/nginx", shouldError: false},
		{name: "registry/name:tag", image: "docker.io/nginx:latest", shouldError: false},
		{name: "full path", image: "gcr.io/project/image:v1.0", shouldError: false},
		{name: "with digest", image: "nginx@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", shouldError: false},
		{name: "debian stable-slim", image: "debian:stable-slim", shouldError: false},
		{name: "ubuntu version", image: "ubuntu:22.04", shouldError: false},
		{name: "private registry", image: "my-registry.example.com:5000/my-image:tag", shouldError: false},

		// Invalid images
		{name: "shell injection semicolon", image: "nginx; rm -rf /", shouldError: true, errorMsg: "invalid characters"},
		{name: "shell injection pipe", image: "nginx | cat /etc/passwd", shouldError: true, errorMsg: "invalid characters"},
		{name: "shell injection backtick", image: "nginx`whoami`", shouldError: true, errorMsg: "invalid characters"},
		{name: "shell injection dollar", image: "nginx${PATH}", shouldError: true, errorMsg: "invalid characters"},
		{name: "newline injection", image: "nginx\nmalicious", shouldError: true, errorMsg: "invalid characters"},
		{name: "too long", image: strings.Repeat("a", 600), shouldError: true, errorMsg: "too long"},
		{name: "invalid format spaces", image: "nginx latest", shouldError: true, errorMsg: "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerImage(tt.image)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateVolumeMount(t *testing.T) {
	tests := []struct {
		name        string
		volume      string
		shouldError bool
		errorMsg    string
	}{
		// Valid volumes
		{name: "simple", volume: "/host/path:/container/path", shouldError: false},
		{name: "with ro option", volume: "/host/path:/container/path:ro", shouldError: false},
		{name: "with rw option", volume: "/host/path:/container/path:rw", shouldError: false},
		{name: "relative host", volume: "./data:/data", shouldError: false},
		{name: "named volume", volume: "myvolume:/data", shouldError: false},
		{name: "selinux z", volume: "/host:/container:z", shouldError: false},
		{name: "multiple options", volume: "/host:/container:ro,z", shouldError: false},

		// Invalid format
		{name: "empty", volume: "", shouldError: true, errorMsg: "cannot be empty"},
		{name: "no colon", volume: "/just/a/path", shouldError: true, errorMsg: "invalid format"},
		{name: "too many colons", volume: "a:b:c:d:e", shouldError: true, errorMsg: "invalid format"},
		{name: "empty host", volume: ":/container", shouldError: true, errorMsg: "host path cannot be empty"},
		{name: "empty container", volume: "/host:", shouldError: true, errorMsg: "container path cannot be empty"},
		{name: "relative container", volume: "/host:relative", shouldError: true, errorMsg: "must be absolute"},

		// Invalid characters
		{name: "shell injection", volume: "/host;rm -rf /:/container", shouldError: true, errorMsg: "invalid characters"},
		{name: "backtick", volume: "/host`whoami`:/container", shouldError: true, errorMsg: "invalid characters"},

		// Invalid options
		{name: "bad option", volume: "/host:/container:invalid", shouldError: true, errorMsg: "invalid option"},

		// Sensitive paths
		{name: "etc shadow", volume: "/etc/shadow:/data", shouldError: true, errorMsg: "sensitive path"},
		{name: "docker socket", volume: "/var/run/docker.sock:/var/run/docker.sock", shouldError: true, errorMsg: "sensitive path"},
		{name: "ssh dir", volume: "/root/.ssh:/ssh", shouldError: true, errorMsg: "sensitive path"},
		{name: "proc", volume: "/proc:/proc", shouldError: true, errorMsg: "sensitive path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeMount(tt.volume)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidatePortMapping(t *testing.T) {
	tests := []struct {
		name        string
		port        string
		shouldError bool
		errorMsg    string
	}{
		// Valid ports
		{name: "simple", port: "8080:80", shouldError: false},
		{name: "same port", port: "80:80", shouldError: false},
		{name: "with tcp protocol", port: "8080:80/tcp", shouldError: false},
		{name: "with udp protocol", port: "53:53/udp", shouldError: false},
		{name: "with sctp protocol", port: "132:132/sctp", shouldError: false},
		{name: "with host ip", port: "127.0.0.1:8080:80", shouldError: false},
		{name: "port range", port: "8080-8090:80-90", shouldError: false},
		{name: "empty host ip", port: ":8080:80", shouldError: false},

		// Invalid
		{name: "empty", port: "", shouldError: true, errorMsg: "cannot be empty"},
		{name: "invalid characters", port: "80;whoami:80", shouldError: true, errorMsg: "invalid characters"},
		{name: "invalid protocol", port: "80:80/http", shouldError: true, errorMsg: "invalid protocol"},
		{name: "port zero", port: "0:80", shouldError: true, errorMsg: "cannot be 0"},
		{name: "port too high", port: "99999:80", shouldError: true, errorMsg: "out of range"},
		{name: "not a number", port: "abc:80", shouldError: true, errorMsg: "not a valid"},
		{name: "invalid ip", port: "999.999.999.999:80:80", shouldError: true, errorMsg: "invalid host IP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePortMapping(tt.port)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		shouldError bool
		errorMsg    string
	}{
		// Valid filenames
		{name: "simple", filename: "file.txt", shouldError: false},
		{name: "with dash", filename: "my-file.txt", shouldError: false},
		{name: "with underscore", filename: "my_file.txt", shouldError: false},
		{name: "no extension", filename: "README", shouldError: false},
		{name: "multiple dots", filename: "file.tar.gz", shouldError: false},

		// Invalid filenames
		{name: "empty", filename: "", shouldError: true, errorMsg: "cannot be empty"},
		{name: "too long", filename: strings.Repeat("a", 300), shouldError: true, errorMsg: "too long"},
		{name: "contains colon", filename: "file:name.txt", shouldError: true, errorMsg: "invalid character"},
		{name: "contains question", filename: "file?.txt", shouldError: true, errorMsg: "invalid character"},
		{name: "contains asterisk", filename: "file*.txt", shouldError: true, errorMsg: "invalid character"},
		{name: "contains null", filename: "file\x00.txt", shouldError: true, errorMsg: "invalid character"},
		{name: "ends with space", filename: "file.txt ", shouldError: true, errorMsg: "cannot end with"},
		{name: "ends with period", filename: "file.", shouldError: true, errorMsg: "cannot end with"},

		// Windows reserved names
		{name: "CON", filename: "CON", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "PRN", filename: "PRN", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "AUX", filename: "AUX", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "NUL", filename: "NUL", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "COM1", filename: "COM1", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "LPT1", filename: "LPT1", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "con lowercase", filename: "con", shouldError: true, errorMsg: "reserved on Windows"},
		{name: "CON.txt", filename: "CON.txt", shouldError: true, errorMsg: "reserved on Windows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.filename)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsWindowsReservedName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"CON", "CON", true},
		{"con", "con", true},
		{"Con", "Con", true},
		{"CON.txt", "CON.txt", true},
		{"PRN", "PRN", true},
		{"AUX", "AUX", true},
		{"NUL", "NUL", true},
		{"COM1", "COM1", true},
		{"COM9", "COM9", true},
		{"LPT1", "LPT1", true},
		{"LPT9", "LPT9", true},
		{"regular file", "myfile.txt", false},
		{"contains CON", "icon.png", false},
		{"CONNECT", "CONNECT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWindowsReservedName(tt.filename)
			if result != tt.expected {
				t.Errorf("IsWindowsReservedName(%q) = %v, expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		fieldName   string
		maxLen      int
		shouldError bool
	}{
		{"within limit", "hello", "test", 10, false},
		{"at limit", "hello", "test", 5, false},
		{"over limit", "hello world", "test", 5, true},
		{"empty", "", "test", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.value, tt.fieldName, tt.maxLen)
			if tt.shouldError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
