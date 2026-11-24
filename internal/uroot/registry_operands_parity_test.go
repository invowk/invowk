// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

var (
	// usageFlagLine matches an option line of Go's flag.PrintDefaults output:
	// "  -name" for a boolean, "  -name type" for an option with a value.
	usageFlagLine = regexp.MustCompile(`^ {2}-(\S+)(?: (\S+))?`)

	// operandParityCorpus is the table of TestUrootCommandOperandsMatchFlagParse
	// and the seed corpus of FuzzUrootCommandOperands.
	operandParityCorpus = [][]string{
		{"cat", "a", "-x"},
		{"cat", "-u", "a", "-", "--", "b"},
		{"cat", "--", "-x"},
		{"cat", "--", "-"},
		{"cat", "-", "a"},
		{"cat", "-u", "--", "-u"},
		{"rm", "-rf", "a", "-x"},
		{"rm", "--", "-x"},
		{"rm", "-", "a", "-"},
		{"rm", "-r", "-R", "--f", "a"},
		{"rm", "-z", "a"},
		{"mv", "a", "b", "-x"},
		{"mv", "-n", "a"},
		{"cp", "-rf", "a", "-"},
		{"cp", "--recursive", "--no-dereference", "a", "b"},
		{"cp", "-P", "--", "-a", "-b"},
		{"mkdir", "-m", "755", "-p", "d"},
		{"mkdir", "-m", "-x", "d"},
		{"mkdir", "--m=700", "d", "-e"},
		{"mkdir", "-pm", "755", "d"},
		{"mkdir", "-m"},
		{"mkdir", "-m", "--", "d"},
		{"touch", "-d", "2024-01-02T03:04:05Z", "f", "-g"},
		{"touch", "-c", "-r", "ref", "f"},
		{urootCommandShasum, "-a", "256", "f", "-x"},
		{urootCommandShasum, "--algorithm=512", "--", "-x"},
		{urootCommandBase64, "-d", "a", "-"},
		{"ls", "-la", "d", "-x"},
		{"ls", "-h"},
		{"gzip", "-k", "-S", ".z", "f", "-x"},
		{"gzip", "-d", "-k", "f.gz"},
		{"gzip", "-c", "f"},
		{"gzip", "-t", "f.gz"},
		{"gzip", "-", "f"},
		{"gzip", "-S", "x/y", "f"},
		{"gzip", "-S=x/y", "-d", "f"},
		{"gzip", "--S=x/y", "f"},
		{"gzip", "-k9", "f"},
		{"gzip", "--", "-f"},
		{"find", "-name", "x", "d"},
		{"find", "d", "-type", "f", "-l"},
		{"find", "-name=-x", "d"},
		{"find", "d", "-zz"},
		{"tar", "-c", "-f", "a.tar", "src", "-x"},
		{"tar", "-cf", "a.tar", "src"},
		{"tar", "-tf", "a.tar", "member.txt", "-x"},
		{"tar", "--file=a.tar", "-x", "dir"},
		{"tar", "-x", "-f", "a.tar", "--", "-d"},
		{"tar", "-t", "--file", "a.tar", "--create"},
		{"tar", "-cfarchive.tar", "src"},
		{"find", "---0"},
		{"find", "d", "-="},
		{"rm", "----", "a"},
		{"rm", "-r", "-=", "a"},
		{"gzip", "-k", "---", "f"},
	}
)

type (
	// parityValidator maps every path to root/<hex of the path>, so that a
	// resolved operand is absolute and tells which request produced it, and
	// records each call.
	parityValidator struct {
		root  string
		mu    sync.Mutex
		calls []operandTestCall
	}

	// referenceReading is a built-in's arguments as its parser reads them:
	// the final value of every option that was set, and the operands.
	referenceReading struct {
		values   map[string]string
		operands []string
	}
)

func (v *parityValidator) mapped(path string) string {
	return filepath.Join(v.root, "r"+hex.EncodeToString([]byte(path)))
}

func (v *parityValidator) hook(follow bool) func(cwd, path string) (string, error) {
	return func(_, path string) (string, error) {
		v.mu.Lock()
		v.calls = append(v.calls, operandTestCall{path: path, follow: follow})
		v.mu.Unlock()
		return v.mapped(path), nil
	}
}

func (v *parityValidator) recorded() []operandTestCall {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.calls)
}

// referenceRead reads raw as the built-in name does, independently of the
// collector: the built-in's own preprocessing and a fresh flag set with typed
// options from its FlagInfo. For the ArgsToGoArgs commands a raw "--" before
// the first argument that does not start with "-" splits the list: the part
// before it is parsed, the part after it is operands as written.
func referenceRead(name string, raw []string, splitDashDash bool) (referenceReading, error) {
	preprocess := urootOperandRules[name]
	var tail []string
	hasTail := false
	if preprocess == operandPreprocessArgsToGoArgs && splitDashDash {
		for i, arg := range raw {
			if arg == "--" {
				raw, tail, hasTail = raw[:i], raw[i+1:], true
				break
			}
			if !strings.HasPrefix(arg, "-") {
				break
			}
		}
	}
	var list []string
	switch preprocess {
	case operandPreprocessArgsToGoArgs:
		list = unixflag.ArgsToGoArgs(raw)
	case operandPreprocessFindReorder:
		list = referenceFindReorder(raw)
	case operandPreprocessNone:
		list = slices.Clone(raw)
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, info := range validatedWrappers()[name].SupportedFlags() {
		for _, flagName := range []string{info.Name, info.ShortName} {
			if flagName == "" || fs.Lookup(flagName) != nil {
				continue
			}
			if info.TakesValue {
				fs.String(flagName, "", "")
			} else {
				fs.Bool(flagName, false, "")
			}
		}
	}
	if err := fs.Parse(list); err != nil {
		return referenceReading{}, fmt.Errorf("reference parse: %w", err)
	}
	reading := referenceReading{values: map[string]string{}, operands: slices.Clone(fs.Args())}
	fs.Visit(func(f *flag.Flag) { reading.values[f.Name] = f.Value.String() })
	if hasTail {
		reading.operands = append(reading.operands, tail...)
	}
	return reading, nil
}

// referenceFindReorder restates upstream find's reorderArgs (u-root v0.16.0).
func referenceFindReorder(args []string) []string {
	var front, rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "-name" || arg == "-type" || arg == "-mode") && i+1 < len(args):
			front = append([]string{arg, args[i+1]}, front...)
			i++
		case strings.HasPrefix(arg, "-"):
			front = append([]string{arg}, front...)
		default:
			rest = append(rest, arg)
		}
	}
	return append(front, rest...)
}

// referenceAsWritten reports whether operand stays as written.
func referenceAsWritten(name string, reading referenceReading, operand string) bool {
	switch name {
	case "cat":
		return operand == "-"
	case "tar":
		isSet := func(names ...string) bool {
			for _, n := range names {
				if reading.values[n] == "true" {
					return true
				}
			}
			return false
		}
		return isSet("t", "list") && !isSet("c", "create") && !isSet("x", "extract")
	default:
		return false
	}
}

// referenceFollows reports whether the operand at index, of count operands,
// takes the follow hook: false for every rm operand and every mv source (all
// but the last of two or more), true otherwise (design D3).
func referenceFollows(name string, index, count int) bool {
	switch name {
	case "rm":
		return false
	case "mv":
		return count > 1 && index == count-1
	default:
		return true
	}
}

// checkOperandParity checks the three properties of design D4 "Parity tests"
// for one argument list.
func checkOperandParity(t *testing.T, name string, raw []string) {
	t.Helper()
	validator := &parityValidator{root: filepath.Join(os.TempDir(), "parity-root")}
	r, wrapper := newRecordingRegistry(name)
	hc := &HandlerContext{
		Stdin:                strings.NewReader(""),
		Stdout:               io.Discard,
		Stderr:               io.Discard,
		Dir:                  validator.root,
		ValidatePath:         validator.hook(true),
		ValidateNoFollowPath: validator.hook(false),
	}
	args := append([]string{name}, raw...)
	runErr := r.Run(WithHandlerContext(t.Context(), hc), name, args)
	received, called := wrapper.received()
	calls := validator.recorded()

	reference, refErr := referenceRead(name, raw, true)
	if refErr != nil {
		if runErr != nil || !called || len(received) != 2 || len(calls) != 0 {
			t.Fatalf("%q: the parser rejects it, but Run = %v, received %q, hook calls %v; want only the failing option handed on", args, runErr, received, calls)
		}
		return
	}

	// Property 1: every operand of the reference reading reached a hook,
	// except the two as-written cases, and the hook its position selects
	// (design D3: rm operands and mv sources the no-follow hook).
	called1 := map[operandTestCall]int{}
	for _, call := range calls {
		called1[call]++
	}
	for index, operand := range reference.operands {
		if referenceAsWritten(name, reference, operand) {
			continue
		}
		want := operandTestCall{path: operand, follow: referenceFollows(name, index, len(reference.operands))}
		if called1[want] == 0 {
			t.Fatalf("%q: operand %q reached no hook with follow=%v (calls %v)", args, operand, want.follow, calls)
		}
		called1[want]--
	}

	suffix, hasSuffix := reference.values["S"]
	if name == "gzip" && hasSuffix && strings.ContainsAny(suffix, gzipSuffixSeparators()) {
		// Property 3, second half: a -S value with a separator stops the
		// command before it runs.
		if runErr == nil || called {
			t.Fatalf("%q: a -S suffix with a separator did not stop the command (Run = %v, called %v)", args, runErr, called)
		}
		return
	}
	if runErr != nil || !called {
		t.Fatalf("%q: Run = %v, called %v; want the command run", args, runErr, called)
	}

	// Property 2: the received list, read by the built-in's own parser,
	// gives the same option values and exactly the hook results.
	got, err := referenceRead(name, received[1:], false)
	if err != nil {
		t.Fatalf("%q: the built-in cannot parse what it received (%q): %v", args, received, err)
	}
	wantValues := maps.Clone(reference.values)
	if name == "tar" {
		for _, option := range []string{"f", "file"} {
			if value, ok := wantValues[option]; ok {
				wantValues[option] = validator.mapped(value)
			}
		}
	}
	if !maps.Equal(got.values, wantValues) {
		t.Fatalf("%q: received option values %v, want %v", args, got.values, wantValues)
	}
	var wantOperands []string
	for _, operand := range reference.operands {
		if referenceAsWritten(name, reference, operand) {
			wantOperands = append(wantOperands, operand)
			continue
		}
		wantOperands = append(wantOperands, validator.mapped(operand))
	}
	if !slices.Equal(got.operands, wantOperands) {
		t.Fatalf("%q: received operands %q, want %q", args, got.operands, wantOperands)
	}

	// Property 3: gzip's output path of each resolved operand reached the
	// follow hook.
	if name == "gzip" && reference.values["c"] != "true" && reference.values["t"] != "true" {
		if !hasSuffix {
			suffix = gzipDefaultSuffix
		}
		for _, operand := range wantOperands {
			output := operand + suffix
			if reference.values["d"] == "true" {
				output = strings.TrimSuffix(operand, suffix)
			}
			if !slices.Contains(calls, operandTestCall{path: output, follow: true}) {
				t.Fatalf("%q: gzip's output path %q reached no follow hook (calls %v)", args, output, calls)
			}
		}
	}
}

func gzipSuffixSeparators() string {
	if filepath.Separator == '\\' {
		return `/\`
	}
	return "/"
}

func TestUrootCommandOperandsMatchFlagParse(t *testing.T) {
	t.Parallel()
	for _, args := range operandParityCorpus {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			checkOperandParity(t, args[0], args[1:])
		})
	}
}

func FuzzUrootCommandOperands(f *testing.F) {
	for _, args := range operandParityCorpus {
		f.Add(args[0], strings.Join(args[1:], "\x00"))
	}
	names := slices.Sorted(maps.Keys(urootOperandRules))
	f.Fuzz(func(t *testing.T, name, joined string) {
		if _, ok := urootOperandRules[name]; !ok {
			name = names[len(name)%len(names)]
		}
		var raw []string
		if joined != "" {
			raw = strings.Split(joined, "\x00")
		}
		checkOperandParity(t, name, raw)
	})
}

func TestUrootFlagInfoMatchesUpstream(t *testing.T) {
	t.Parallel()
	for name, cmd := range validatedWrappers() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			ctx := WithHandlerContext(t.Context(), &HandlerContext{
				Stdin:  strings.NewReader(""),
				Stdout: io.Discard,
				Stderr: &stderr,
				Dir:    t.TempDir(),
			})
			if err := cmd.Run(ctx, []string{name, "--invowk-flag-probe"}); err == nil {
				t.Fatalf("%s accepted an undefined option", name)
			}
			upstream := map[string]bool{}
			scanner := bufio.NewScanner(&stderr)
			for scanner.Scan() {
				if m := usageFlagLine.FindStringSubmatch(scanner.Text()); m != nil {
					upstream[m[1]] = m[2] != ""
				}
			}
			if len(upstream) == 0 {
				t.Fatalf("%s printed no option list:\n%s", name, stderr.String())
			}
			declared := map[string]bool{}
			for _, info := range cmd.SupportedFlags() {
				for _, flagName := range []string{info.Name, info.ShortName} {
					if flagName != "" {
						declared[flagName] = info.TakesValue
					}
				}
			}
			if !maps.Equal(declared, upstream) {
				t.Errorf("%s FlagInfo (name: takes a value) = %v, upstream defines %v", name, declared, upstream)
			}
		})
	}
}

func TestUrootFindOperandsMatchUpstream(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]string{
		{"d"},
		{"-name", "x", "d"},
		{"d", "-name", "x"},
		{"d", "-type", "f", "-l"},
		{"-type", "d", "d"},
		{"-mode", "420", "d"},
	} {
		t.Run(strings.Join(raw, " "), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "d", "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "d", "x"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			validator := &operandTestValidator{root: root}
			var stdout, stderr bytes.Buffer
			hc := validator.handler(root, &stdout, &stderr)
			args := append([]string{"find"}, raw...)
			if err := BuildDefaultRegistry().Run(WithHandlerContext(t.Context(), hc), "find", args); err != nil {
				t.Fatalf("find %q: %v (stderr %s)", raw, err, stderr.String())
			}
			paths := validator.recordedPaths()
			if len(paths) != 1 || paths[0] != "d" {
				t.Fatalf("find %q validated %q, want only its starting path d", raw, paths)
			}
			start := filepath.Join(root, "d")
			for line := range strings.Lines(stdout.String()) {
				if line = strings.TrimSpace(line); line != "" && !strings.Contains(line, start) {
					t.Errorf("find %q walked %q, outside its starting path %s", raw, line, start)
				}
			}
		})
	}
}
