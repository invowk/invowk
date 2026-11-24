// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

type upstreamCase struct {
	name  string
	setup func(t *testing.T, base string)
	args  []string
	// refused is the prediction refusal the restricted visitor must give;
	// nil means the prediction must equal what u-root touches.
	refused error
}

// derivedResolvedArgs reads and resolves args as urootCommandOperands does.
func derivedResolvedArgs(t *testing.T, hc *HandlerContext, args []string) urootArgs {
	t.Helper()
	name := args[0]
	cmd, ok := BuildDefaultRegistry().Lookup(name)
	if !ok {
		t.Fatalf("no built-in %s", name)
	}
	parsed, _, err := readUrootArgs(urootOperandRules[name], cmd.SupportedFlags(), args[1:], true)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveUrootArgs(hc, name, parsed)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// predictDerivedPaths returns what the enumerator of args[0] yields: every
// destination, and the link creations among them.
func predictDerivedPaths(t *testing.T, hc *HandlerContext, args []string) (destinations, links []string) {
	t.Helper()
	resolved := derivedResolvedArgs(t, hc, args)
	visit := func(p derivedPath) error {
		switch p.use {
		case derivedPathFileWrite, derivedPathDirCreate:
			destinations = append(destinations, filepath.Clean(p.path))
		case derivedPathLinkCreate:
			destinations = append(destinations, filepath.Clean(p.path))
			links = append(links, filepath.Clean(p.path))
		case derivedPathModeChange, derivedPathSourceLink, derivedPathSourceOperand:
		}
		return nil
	}
	var err error
	switch args[0] {
	case "cp":
		err = cpDerivedPaths(t.Context(), resolved, visit)
	case "tar":
		err = tarDerivedPaths(t.Context(), resolved, visit)
	default:
		t.Fatalf("no enumerator for %s", args[0])
	}
	if err != nil {
		t.Fatalf("enumerate %q: %v", args, err)
	}
	return destinations, links
}

// closeUnderNewAncestors adds every ancestor of a predicted path, below base,
// that did not exist before the run: MkdirAll creates them.
func closeUnderNewAncestors(base string, predicted []string, before map[string]string) map[string]bool {
	closed := make(map[string]bool)
	for _, path := range predicted {
		closed[path] = true
		for dir := filepath.Dir(path); dir != base && strings.HasPrefix(dir, base); dir = filepath.Dir(dir) {
			if _, existed := before[dir]; existed {
				break
			}
			closed[dir] = true
		}
	}
	return closed
}

// touchedPaths lists the entries present after the run and not before, and
// the regular files whose content changed.
func touchedPaths(before, after map[string]string) (touched map[string]bool, newLinks []string) {
	touched = make(map[string]bool)
	for path, value := range after {
		old, existed := before[path]
		if !existed {
			touched[path] = true
			if strings.HasPrefix(value, "L") {
				newLinks = append(newLinks, path)
			}
			continue
		}
		if old != value && strings.HasPrefix(value, "-") {
			touched[path] = true
		}
	}
	return touched, newLinks
}

func assertSameSet(t *testing.T, what string, predicted, touched map[string]bool) {
	t.Helper()
	if maps.Equal(predicted, touched) {
		return
	}
	for _, path := range slices.Sorted(maps.Keys(touched)) {
		if !predicted[path] {
			t.Errorf("%s: touched but not predicted: %s", what, path)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(predicted)) {
		if !touched[path] {
			t.Errorf("%s: predicted but not touched: %s", what, path)
		}
	}
}

func upstreamTree(t *testing.T, files map[string]string, dirs ...string) func(t *testing.T, base string) {
	t.Helper()
	return func(t *testing.T, base string) {
		t.Helper()
		for name, content := range files {
			writeTestFile(t, filepath.Join(base, filepath.FromSlash(name)), content)
		}
		for _, dir := range dirs {
			mkdirTest(t, filepath.Join(base, filepath.FromSlash(dir)))
		}
	}
}

func upstreamTarSetup(withExtractionDir bool) func(t *testing.T, base string) {
	return func(t *testing.T, base string) {
		t.Helper()
		writeTarArchive(t, filepath.Join(base, "a.tar"),
			tarDir("top", 0o755),
			tarFile("top/f", "f", 0o644),
			tarDir("top/sub/deep", 0o750),
			tarFile("top/sub/deep/g", "g", 0o600),
			tarFile("h", "h", 0o644),
			tarDir("empty", 0o700),
		)
		if withExtractionDir {
			mkdirTest(t, filepath.Join(base, "ex"))
		}
	}
}

// TestUrootDerivedPathsMatchUpstream compares, on trees whose destinations
// hold no link, the paths the enumerators predict with the paths u-root
// creates or writes when it runs: a prediction that misses a path would leave
// that path unchecked.
func TestUrootDerivedPathsMatchUpstream(t *testing.T) {
	t.Parallel()
	cases := []upstreamCase{
		{name: "cp file into directory", setup: upstreamTree(t, map[string]string{"f": "f"}, "d"), args: []string{"cp", "f", "d"}},
		{name: "cp two files into directory", setup: upstreamTree(t, map[string]string{"f": "f", "g": "g"}, "d"), args: []string{"cp", "f", "g", "d"}},
		{name: "cp file to new path", setup: upstreamTree(t, map[string]string{"f": "f"}), args: []string{"cp", "f", "new"}},
		{
			name: "cp -r into existing directory", setup: upstreamTree(t, map[string]string{"src/x": "x", "src/sub/y": "y"}, "dst"),
			args: []string{"cp", "-r", "src", "dst"},
		},
		{
			name: "cp -r to missing directory", setup: upstreamTree(t, map[string]string{"src/x": "x", "src/sub/y": "y"}),
			args: []string{"cp", "-r", "src", "dst"},
		},
		{
			name: "cp -r nested directories",
			setup: upstreamTree(t, map[string]string{"src/a/b/c/f": "f", "src/z": "z", "src/a/g": "g"},
				"src/a/e", "src/a/b/empty", "dst"),
			args: []string{"cp", "-R", "src", "dst"},
		},
		{
			name: "cp -r same-named trees with overlapping entries",
			setup: upstreamTree(t, map[string]string{
				"a/s/x": "ax", "a/s/sub/p": "ap", "b/s/x": "bx", "b/s/y": "by", "b/s/sub/q": "bq",
			}, "d"),
			args: []string{"cp", "-r", "a/s", "b/s", "d"},
		},
		{
			name:  "cp -r distinct trees",
			setup: upstreamTree(t, map[string]string{"a/x": "ax", "b/y/z": "byz"}, "d"),
			args:  []string{"cp", "--recursive", "a", "b", "d"},
		},
		{name: "tar -x into existing directory", setup: upstreamTarSetup(true), args: []string{"tar", "-x", "-f", "a.tar", "ex"}},
		{name: "tar -x into missing directory", setup: upstreamTarSetup(false), args: []string{"tar", "-x", "-f", "a.tar", "ex"}},
		{
			name:  "cp -r later source an earlier source creates",
			setup: upstreamTree(t, map[string]string{"a/s/x/f": "f"}, "d"),
			args:  []string{"cp", "-r", "a/s", "d/s/x", "d"}, refused: errCpSourceMissing,
		},
		{
			name:  "cp -r directory into itself",
			setup: upstreamTree(t, map[string]string{"d/x": "x"}),
			args:  []string{"cp", "-r", "d", "d"}, refused: errCpIntoSource,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			tc.setup(t, base)
			restricted := &HandlerContext{Dir: base, ValidatePath: (&derivedTestValidator{}).follow}
			checkErr := checkUrootDerivedPaths(t.Context(), restricted, tc.args[0], derivedResolvedArgs(t, restricted, tc.args))
			if tc.refused != nil {
				if !errors.Is(checkErr, tc.refused) {
					t.Fatalf("restricted check = %v, want %v", checkErr, tc.refused)
				}
				return
			}
			if checkErr != nil {
				t.Fatalf("restricted check refused a predictable command: %v", checkErr)
			}
			compareWithUpstream(t, base, tc.args, false)
		})
	}
}

// TestUrootDerivedPathsMatchUpstreamLinks compares, under full access, the
// links cp -P -r is predicted to create with the links it creates. D5 rule 3
// refuses these commands under restricted access by the predicted link
// creations, so a missed link would fail open.
func TestUrootDerivedPathsMatchUpstreamLinks(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	cases := []upstreamCase{
		{
			name: "file link",
			setup: func(t *testing.T, base string) {
				t.Helper()
				upstreamTree(t, map[string]string{"src/x": "x"}, "dst")(t, base)
				symlinkTest(t, "x", filepath.Join(base, "src", "l"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"},
		},
		{
			name: "directory link",
			setup: func(t *testing.T, base string) {
				t.Helper()
				upstreamTree(t, map[string]string{"src/sub/y": "y"}, "dst")(t, base)
				symlinkTest(t, filepath.Join(base, "src", "sub"), filepath.Join(base, "src", "dl"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"},
		},
		{
			name: "dangling link",
			setup: func(t *testing.T, base string) {
				t.Helper()
				upstreamTree(t, map[string]string{"src/x": "x"}, "dst")(t, base)
				symlinkTest(t, filepath.Join(base, "nothing"), filepath.Join(base, "src", "dangling"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"},
		},
		{
			name: "nested link",
			setup: func(t *testing.T, base string) {
				t.Helper()
				upstreamTree(t, map[string]string{"src/a/b/f": "f"})(t, base)
				symlinkTest(t, "f", filepath.Join(base, "src", "a", "b", "l"))
			},
			args: []string{"cp", "--no-dereference", "-r", "src", "dst"},
		},
		{
			name: "two sources with links",
			setup: func(t *testing.T, base string) {
				t.Helper()
				upstreamTree(t, map[string]string{"a/s/x": "x", "b/s/y": "y"}, "d")(t, base)
				symlinkTest(t, "x", filepath.Join(base, "a", "s", "la"))
				symlinkTest(t, "y", filepath.Join(base, "b", "s", "sub", "lb"))
			},
			args: []string{"cp", "-P", "-r", "a/s", "b/s", "d"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			tc.setup(t, base)
			compareWithUpstream(t, base, tc.args, true)
		})
	}
}

// compareWithUpstream predicts the destinations of args, runs args under
// full access, and compares the prediction with what the run touched; with
// links it also compares the predicted link creations with the new links.
func compareWithUpstream(t *testing.T, base string, args []string, links bool) {
	t.Helper()
	validator := &derivedTestValidator{}
	predictHC := &HandlerContext{Dir: base, ValidatePath: validator.follow, ValidateNoFollowPath: validator.noFollow}
	destinations, predictedLinks := predictDerivedPaths(t, predictHC, args)
	before := derivedSnapshot(t, base)
	expectNoError(t, runDerivedWith(t, validator, base, true, args...))
	after := derivedSnapshot(t, base)
	touched, newLinks := touchedPaths(before, after)
	assertSameSet(t, "destinations", closeUnderNewAncestors(base, destinations, before), touched)
	if !links {
		return
	}
	if len(predictedLinks) == 0 {
		t.Fatal("the case creates no link")
	}
	slices.Sort(predictedLinks)
	slices.Sort(newLinks)
	if !slices.Equal(predictedLinks, newLinks) {
		t.Fatalf("predicted link creations %q, created links %q", predictedLinks, newLinks)
	}
	for _, link := range newLinks {
		if info, err := os.Lstat(link); err != nil || info.Mode()&fs.ModeSymlink == 0 {
			t.Fatalf("%s is not a link: %v", link, err)
		}
	}
}
