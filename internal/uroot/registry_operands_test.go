// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

var errOperandTestDenied = errors.New("operand test: denied")

type (
	// operandTestValidator records every hook call and refuses each path
	// outside root.
	operandTestValidator struct {
		root  string
		mu    sync.Mutex
		calls []operandTestCall
		deny  func(path string) bool
	}

	operandTestCall struct {
		path   string
		follow bool
	}

	// recordingWrapper stands for a validated upstream wrapper: it has the
	// wrapper's name and FlagInfo and records the arguments it receives.
	recordingWrapper struct {
		baseWrapper
		mu     sync.Mutex
		called bool
		args   []string
	}
)

func (v *operandTestValidator) resolve(cwd, path string, follow bool) (string, error) {
	v.mu.Lock()
	v.calls = append(v.calls, operandTestCall{path: path, follow: follow})
	v.mu.Unlock()
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(v.root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errOperandTestDenied
	}
	if v.deny != nil && v.deny(resolved) {
		return "", errOperandTestDenied
	}
	return resolved, nil
}

func (v *operandTestValidator) follow(cwd, path string) (string, error) {
	return v.resolve(cwd, path, true)
}

func (v *operandTestValidator) recorded() []operandTestCall {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.calls)
}

func (v *operandTestValidator) recordedPaths() []string {
	var paths []string
	for _, call := range v.recorded() {
		paths = append(paths, call.path)
	}
	return paths
}

func (v *operandTestValidator) handler(dir string, stdout, stderr *bytes.Buffer) *HandlerContext {
	hc := &HandlerContext{
		Stdin:        strings.NewReader(""),
		Stdout:       stdout,
		Stderr:       stderr,
		Dir:          dir,
		LookupEnv:    os.LookupEnv,
		ValidatePath: v.follow,
	}
	return hc
}

func (w *recordingWrapper) Run(_ context.Context, args []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.called = true
	w.args = slices.Clone(args)
	return nil
}

func (w *recordingWrapper) received() ([]string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.args), w.called
}

// validatedWrappers returns the twelve validated upstream wrappers.
func validatedWrappers() map[string]Command {
	return map[string]Command{
		urootCommandBase64: newBase64Command(),
		"cat":              newCatCommand(),
		"cp":               newCpCommand(),
		"find":             newFindCommand(),
		"gzip":             newGzipCommand(),
		"ls":               newLsCommand(),
		"mkdir":            newMkdirCommand(),
		"mv":               newMvCommand(),
		"rm":               newRmCommand(),
		urootCommandShasum: newShasumCommand(),
		"tar":              newTarCommand(),
		"touch":            newTouchCommand(),
	}
}

// newRecordingRegistry registers a recording wrapper with the name and
// FlagInfo of the validated wrapper name.
func newRecordingRegistry(name string) (*Registry, *recordingWrapper) {
	upstream := validatedWrappers()[name]
	wrapper := &recordingWrapper{}
	wrapper.name = name
	wrapper.flags = upstream.SupportedFlags()
	r := NewRegistry()
	r.Register(wrapper)
	return r, wrapper
}

// runRecorded runs args through a recording registry under a validating
// context rooted at root, with dir as the working directory.
func runRecorded(t *testing.T, root, dir string, args []string) (validator *operandTestValidator, received []string, called bool, err error) {
	t.Helper()
	validator = &operandTestValidator{root: root}
	r, wrapper := newRecordingRegistry(args[0])
	var stdout, stderr bytes.Buffer
	err = r.Run(WithHandlerContext(t.Context(), validator.handler(dir, &stdout, &stderr)), args[0], args)
	received, called = wrapper.received()
	return validator, received, called, err
}

// operandTestRoot returns an absolute fixed root and a working directory
// inside it.
func operandTestRoot(t *testing.T) (root, dir string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "root")
	return root, filepath.Join(root, "work")
}

// readReceived reads a received argument list as the built-in does, with
// only its own preprocessing.
func readReceived(t *testing.T, name string, received []string) urootArgs {
	t.Helper()
	parsed, _, err := readUrootArgs(urootOperandRules[name], validatedWrappers()[name].SupportedFlags(), received[1:], false)
	if err != nil {
		t.Fatalf("%s received %q, which its parser rejects: %v", name, received, err)
	}
	return parsed
}

func TestUrootCommandOperandsDashOperandDenied(t *testing.T) {
	t.Parallel()
	outside := "-" + string(filepath.Separator) + filepath.Join("..", "..", "..", "outside", "x")
	for _, args := range [][]string{
		{urootCommandBase64, "a", outside},
		{"cat", "a", outside},
		{"cp", "a", outside},
		{"gzip", "a", outside},
		{"ls", "a", outside},
		{"mkdir", "a", outside},
		{"mv", "a", "b", outside},
		{"rm", "keep", outside},
		{urootCommandShasum, "a", outside},
		{"touch", "a", outside},
		{"tar", "-c", "-f", "a.tar", "src", outside},
	} {
		t.Run(args[0], func(t *testing.T) {
			t.Parallel()
			root, dir := operandTestRoot(t)
			validator, _, called, err := runRecorded(t, root, dir, args)
			if !errors.Is(err, errOperandTestDenied) {
				t.Fatalf("Run(%q) error = %v, want the denial", args, err)
			}
			if called {
				t.Errorf("Run(%q) ran the command after a denial", args)
			}
			if !slices.Contains(validator.recordedPaths(), outside) {
				t.Errorf("the dash-prefixed operand was not validated: calls %v", validator.recordedPaths())
			}
		})
	}
}

func TestUrootCommandOperandsDoubleDash(t *testing.T) {
	t.Parallel()
	for _, name := range []string{urootCommandBase64, "cat", "cp", "ls", "mkdir", "mv", "rm", urootCommandShasum, "touch", "gzip"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, dir := operandTestRoot(t)
			args := []string{name, "--", "-x"}
			if name == "cp" || name == "mv" {
				args = append(args, "dest")
			}
			validator, received, called, err := runRecorded(t, root, dir, args)
			if err != nil || !called {
				t.Fatalf("Run(%q) = (called %v, %v), want it run", args, called, err)
			}
			if !slices.Contains(validator.recordedPaths(), "-x") {
				t.Errorf("-x after -- was not validated: calls %v", validator.recordedPaths())
			}
			got := readReceived(t, name, received)
			if len(got.options) != 0 {
				t.Errorf("%s received options %v, want none", name, got.options)
			}
			if got.operands[0] != filepath.Join(dir, "-x") {
				t.Errorf("%s received operands %q, want %s first", name, got.operands, filepath.Join(dir, "-x"))
			}
		})
	}
}

func TestUrootCommandOperandsValueOptionsKept(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args   []string
		option urootOption
		want   []string
	}{
		{[]string{"mkdir", "-m", "755", "d"}, urootOption{name: "m", value: "755"}, []string{"d"}},
		{[]string{"touch", "-d", "2024-01-02T03:04:05Z", "f"}, urootOption{name: "d", value: "2024-01-02T03:04:05Z"}, []string{"f"}},
		{[]string{urootCommandShasum, "-a", "256", "f"}, urootOption{name: "a", value: "256"}, []string{"f"}},
		{[]string{"gzip", "-S", ".z", "f"}, urootOption{name: "S", value: ".z"}, []string{"f"}},
		{[]string{"find", "-name", "x", "d"}, urootOption{name: "name", value: "x"}, []string{"d"}},
		{[]string{"mkdir", "--m=-x", "d"}, urootOption{name: "m", value: "-x"}, []string{"d"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()
			root, dir := operandTestRoot(t)
			validator, received, called, err := runRecorded(t, root, dir, tc.args)
			if err != nil || !called {
				t.Fatalf("Run = (called %v, %v)", called, err)
			}
			got := readReceived(t, tc.args[0], received)
			if !slices.Contains(got.options, tc.option) {
				t.Errorf("received options %v, want %v as written", got.options, tc.option)
			}
			var want []string
			for _, operand := range tc.want {
				want = append(want, filepath.Join(dir, operand))
			}
			if !slices.Equal(got.operands, want) {
				t.Errorf("received operands %q, want %q", got.operands, want)
			}
			if slices.Contains(validator.recordedPaths(), tc.option.value) {
				t.Errorf("the option value %q was validated as a path", tc.option.value)
			}
		})
	}
}

func TestUrootCommandOperandsFindAndTarPaths(t *testing.T) {
	t.Parallel()
	outside := func(root string, name string) string { return filepath.Join(filepath.Dir(root), "outside", name) }
	for _, tc := range []struct {
		name string
		args func(root string) []string
	}{
		{"find starting path after options", func(root string) []string { return []string{"find", "-name", "x", outside(root, "d")} }},
		{"tar extraction directory", func(root string) []string { return []string{"tar", "-x", "-f", "a.tar", outside(root, "d")} }},
		{"tar create archive", func(root string) []string { return []string{"tar", "-c", "-f", outside(root, "a.tar"), "src"} }},
		{"tar list archive", func(root string) []string { return []string{"tar", "-t", "--file=" + outside(root, "a.tar")} }},
		{"tar combined archive", func(root string) []string { return []string{"tar", "-cf", outside(root, "a.tar"), "src"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, dir := operandTestRoot(t)
			_, _, called, err := runRecorded(t, root, dir, tc.args(root))
			if !errors.Is(err, errOperandTestDenied) || called {
				t.Fatalf("Run = (called %v, %v), want the denial before the command runs", called, err)
			}
		})
	}

	t.Run("tar list operands stay as written", func(t *testing.T) {
		t.Parallel()
		root, dir := operandTestRoot(t)
		validator, received, called, err := runRecorded(t, root, dir, []string{"tar", "-tf", "a.tar", "member.txt", "-x"})
		if err != nil || !called {
			t.Fatalf("Run = (called %v, %v)", called, err)
		}
		got := readReceived(t, "tar", received)
		if !slices.Equal(got.operands, []string{"member.txt", "-x"}) {
			t.Errorf("tar -t operands = %q, want them as written", got.operands)
		}
		if want := []string{"a.tar"}; !slices.Equal(validator.recordedPaths(), want) {
			t.Errorf("validated %q, want only the archive %q", validator.recordedPaths(), want)
		}
		if !slices.Contains(got.options, urootOption{name: "f", value: filepath.Join(dir, "a.tar")}) {
			t.Errorf("tar -f value was not resolved: %v", got.options)
		}
	})
}

func TestUrootCommandOperandsLoneDash(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args          []string
		want          func(dir string) []string
		validatedDash bool
	}{
		{[]string{"cat", "a", "-"}, func(dir string) []string { return []string{filepath.Join(dir, "a"), "-"} }, false},
		{[]string{"cat", "-", "a"}, func(dir string) []string { return []string{filepath.Join(dir, "a")} }, false},
		{[]string{"rm", "-", "a"}, func(dir string) []string { return []string{filepath.Join(dir, "a")} }, false},
		{[]string{"cp", "a", "-"}, func(dir string) []string { return []string{filepath.Join(dir, "a"), filepath.Join(dir, "-")} }, true},
		{[]string{"rm", "a", "-"}, func(dir string) []string { return []string{filepath.Join(dir, "a"), filepath.Join(dir, "-")} }, true},
		{[]string{"cat", "--", "-"}, func(_ string) []string { return []string{"-"} }, false},
		{[]string{"gzip", "-", "a"}, func(dir string) []string { return []string{filepath.Join(dir, "-"), filepath.Join(dir, "a")} }, true},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()
			root, dir := operandTestRoot(t)
			validator, received, called, err := runRecorded(t, root, dir, tc.args)
			if err != nil || !called {
				t.Fatalf("Run = (called %v, %v)", called, err)
			}
			got := readReceived(t, tc.args[0], received)
			if want := tc.want(dir); !slices.Equal(got.operands, want) {
				t.Errorf("received operands %q, want %q", got.operands, want)
			}
			if slices.Contains(validator.recordedPaths(), "-") != tc.validatedDash {
				t.Errorf("validated %q; want \"-\" validated = %v", validator.recordedPaths(), tc.validatedDash)
			}
		})
	}
}

func TestUrootCommandOperandsParseErrorHandsOnTheOption(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"rm", "-z", "a"}, []string{"rm", "--z"}},
		{[]string{"rm", "--zap", "a"}, []string{"rm", "--zap"}},
		{[]string{"touch", "-r", "ref", "f"}, []string{"touch", "--r"}},
		{[]string{"mkdir", "-m"}, []string{"mkdir", "--m"}},
		{[]string{"mkdir", "-m", "--", "d"}, []string{"mkdir", "--m"}},
		{[]string{"cat", "-h", "a"}, []string{"cat", "--h"}},
		{[]string{"gzip", "-z", "a"}, []string{"gzip", "-z"}},
		{[]string{"find", "d", "-zz"}, []string{"find", "-zz"}},
		{[]string{"rm", "--r=maybe", "a"}, []string{"rm", "--r=maybe"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()
			root, dir := operandTestRoot(t)
			validator, received, called, err := runRecorded(t, root, dir, tc.args)
			if err != nil || !called {
				t.Fatalf("Run = (called %v, %v), want the built-in to report the option", called, err)
			}
			if !slices.Equal(received, tc.want) {
				t.Errorf("received %q, want only the failing option %q", received, tc.want)
			}
			if calls := validator.recorded(); len(calls) != 0 {
				t.Errorf("an operand was validated after a parse error: %v", calls)
			}
		})
	}
}

func TestUrootCommandOperandsRebuildCheck(t *testing.T) {
	t.Parallel()
	flags := newRmCommand().SupportedFlags()
	want := urootArgs{options: []urootOption{{name: "r", value: "true", isBool: true}}, operands: []string{"/abs/a"}}
	if err := checkUrootRebuild(operandPreprocessArgsToGoArgs, flags, rebuildUrootArgs(operandPreprocessArgsToGoArgs, want), want); err != nil {
		t.Fatalf("checkUrootRebuild(rebuilt list) = %v, want nil", err)
	}
	for name, list := range map[string][]string{
		"operand read as an option": {"--r", "-x"},
		"missing operand":           {"--r", "---"},
		"other option":              {"--f", "---", "/abs/a"},
		"unencoded double dash":     {"--r", "--", "/abs/a"},
	} {
		if err := checkUrootRebuild(operandPreprocessArgsToGoArgs, flags, list, want); !errors.Is(err, errUrootOperandRebuild) {
			t.Errorf("checkUrootRebuild(%s %q) = %v, want errUrootOperandRebuild", name, list, err)
		}
	}
}
