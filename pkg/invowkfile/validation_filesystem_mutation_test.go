// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

func TestFilesystemValidationMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "filename length and control characters", run: testFilesystemMutationFilenameBoundaries},
		{name: "containerfile path length", run: testFilesystemMutationContainerfilePathLength},
		{name: "env file path length and parent segments", run: testFilesystemMutationEnvFilePathBoundaries},
		{name: "filepath dependency indexes and lengths", run: testFilesystemMutationFilepathDependencyIndexes},
		{name: "command dependency name length", run: testFilesystemMutationCommandDependencyNameLength},
		{name: "absolute path dialect boundaries", run: testFilesystemMutationAbsolutePathDialects},
		{name: "windows drive letter byte boundaries", run: testFilesystemMutationWindowsDriveLetterBoundaries},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testFilesystemMutationFilenameBoundaries(t *testing.T) {
	t.Helper()

	requireFilesystemMutationNoError(t, "filename exact max", ValidateFilename(strings.Repeat("a", 255)))
	requireFilesystemMutationError(
		t,
		"filename over max",
		ValidateFilename(strings.Repeat("a", 256)),
		"filename too long (256 chars, max 255)",
	)
	requireFilesystemMutationError(
		t,
		"control character after first rune",
		ValidateFilename("ok\x1f"),
		"filename contains control character",
	)
}

func testFilesystemMutationContainerfilePathLength(t *testing.T) {
	t.Helper()

	requireFilesystemMutationNoError(
		t,
		"containerfile exact max",
		ValidateContainerfilePath(filesystemMutationPathOfLength(MaxPathLength), "/ignored"),
	)
	requireFilesystemMutationError(
		t,
		"containerfile over max",
		ValidateContainerfilePath(filesystemMutationPathOfLength(MaxPathLength+1), "/ignored"),
		"containerfile path too long (4097 chars, max 4096)",
	)
}

func testFilesystemMutationEnvFilePathBoundaries(t *testing.T) {
	t.Helper()

	requireFilesystemMutationNoError(t, "env file exact max", ValidateEnvFilePath(filesystemMutationPathOfLength(MaxPathLength)))
	requireFilesystemMutationError(
		t,
		"env file over max",
		ValidateEnvFilePath(filesystemMutationPathOfLength(MaxPathLength+1)),
		"env file path too long (4097 chars, max 4096)",
	)
	requireFilesystemMutationError(
		t,
		"bare parent segment",
		ValidateEnvFilePath(".."),
		"env file path cannot contain '..': ..",
	)
	requireFilesystemMutationError(
		t,
		"leading parent segment",
		ValidateEnvFilePath("../.env"),
		"env file path cannot contain '..': ../.env",
	)
	requireFilesystemMutationError(
		t,
		"middle parent segment",
		ValidateEnvFilePath("config/../.env"),
		"env file path cannot contain '..': config/../.env",
	)
}

func testFilesystemMutationFilepathDependencyIndexes(t *testing.T) {
	t.Helper()

	exactMaxPath := FilesystemPath(strings.Repeat("a", MaxPathLength))
	requireFilesystemMutationNoError(
		t,
		"filepath dependency exact max",
		ValidateFilepathDependency([]FilesystemPath{FilesystemPath("first"), exactMaxPath}),
	)
	requireFilesystemMutationError(
		t,
		"filepath dependency second empty",
		ValidateFilepathDependency([]FilesystemPath{FilesystemPath("first"), FilesystemPath("")}),
		"filepath alternative #2 cannot be empty",
	)
	requireFilesystemMutationError(
		t,
		"filepath dependency second over max",
		ValidateFilepathDependency([]FilesystemPath{FilesystemPath("first"), FilesystemPath(strings.Repeat("a", MaxPathLength+1))}),
		"filepath alternative #2 too long (4097 chars, max 4096)",
	)
}

func testFilesystemMutationCommandDependencyNameLength(t *testing.T) {
	t.Helper()

	requireFilesystemMutationNoError(
		t,
		"command dependency name exact max",
		ValidateCommandDependencyName(CommandName(strings.Repeat("a", MaxNameLength))),
	)
	requireFilesystemMutationError(
		t,
		"command dependency name over max",
		ValidateCommandDependencyName(CommandName(strings.Repeat("a", MaxNameLength+1))),
		"command name too long (257 chars, max 256)",
	)
}

func testFilesystemMutationAbsolutePathDialects(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "three byte drive absolute", path: "C:/", want: true},
		{name: "two byte drive prefix is relative", path: "C:", want: false},
		{name: "non-letter drive prefix is relative", path: "1:/", want: false},
		{name: "missing drive colon is relative", path: "C//", want: false},
		{name: "lowercase drive absolute", path: "z:\\", want: true},
		{name: "windows rooted absolute", path: `\root`, want: true},
		{name: "unix rooted absolute", path: "/root", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isAbsolutePath(tt.path); got != tt.want {
				t.Fatalf("isAbsolutePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func testFilesystemMutationWindowsDriveLetterBoundaries(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		in   byte
		want bool
	}{
		{name: "uppercase lower bound", in: 'A', want: true},
		{name: "uppercase upper bound", in: 'Z', want: true},
		{name: "lowercase lower bound", in: 'a', want: true},
		{name: "lowercase upper bound", in: 'z', want: true},
		{name: "before uppercase lower bound", in: '@', want: false},
		{name: "after uppercase upper bound", in: '[', want: false},
		{name: "before lowercase lower bound", in: '`', want: false},
		{name: "after lowercase upper bound", in: '{', want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isWindowsDriveLetter(tt.in); got != tt.want {
				t.Fatalf("isWindowsDriveLetter(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func filesystemMutationPathOfLength(length int) string {
	if length <= 0 {
		return ""
	}
	if length%2 == 0 {
		return strings.Repeat("a/", (length-2)/2) + "aa"
	}
	return strings.Repeat("a/", (length-1)/2) + "a"
}

func requireFilesystemMutationNoError(t *testing.T, label string, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s error = %v, want nil", label, err)
	}
}

func requireFilesystemMutationError(t *testing.T, label string, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("%s error = nil, want %q", label, want)
	}
	if got := err.Error(); got != want {
		t.Fatalf("%s error = %q, want %q", label, got, want)
	}
}
