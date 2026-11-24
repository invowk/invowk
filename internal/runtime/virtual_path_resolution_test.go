// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	luart "github.com/invowk/golua/runtime"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// These tests cover every consumer of the physical path resolution
// (normalizeExistingOrParent and walkVirtualPath): the validator and resolve,
// Lua invowk.path (resolveBridgePath), Lua require, the allowed roots, and
// declared filesystem.paths, under restricted and under full access. They
// build the validator directly over materialised roots wherever a verdict is
// asserted, because t.TempDir lies below the resolver's own temporary roots.

// pathTestValidator builds a validator whose only allowed roots are roots.
func pathTestValidator(access invowkfile.VirtualFilesystemAccess, roots ...string) virtualPathValidator {
	return virtualPathValidator{resolver: virtualPathResolver{
		allowedRoots: normalizedRoots(roots),
		access:       access,
	}}
}

// pathTestJoin joins a missing suffix below dir.
func pathTestJoin(dir string, names ...string) string {
	return filepath.Join(append([]string{dir}, names...)...)
}

// rawLinkTarget writes a link target exactly as given, with the platform
// separator: filepath.Join would clean a ".." away, and the walk must not
// depend on a cleaned target.
func rawLinkTarget(parts ...string) string {
	return strings.Join(parts, string(filepath.Separator))
}

func TestVirtualPathInRootLinksAccepted(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	root := mustMkdir(t, filepath.Join(fstree.TempRoot(t), "root"))
	in := mustMkdir(t, filepath.Join(root, "in"))
	fstree.MustSymlink(t, "in", filepath.Join(root, "rel"))
	fstree.MustSymlink(t, in, filepath.Join(root, "abs"))
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, root)

	for _, link := range []string{"rel", "abs"} {
		for _, missing := range [][]string{nil, {"n1"}, {"n1", "n2"}} {
			request := pathTestJoin(filepath.Join(root, link), missing...)
			want := pathTestJoin(in, missing...)
			if got, err := validator.validate(root, request); err != nil || got != want {
				t.Errorf("validate(%s) = (%q, %v), want %q", request, got, err, want)
			}
			if got, err := validator.resolver.resolve(request, root); err != nil || got != want {
				t.Errorf("resolve(%s) = (%q, %v), want %q", request, got, err, want)
			}
		}
	}

	// A dangling link whose missing target lies inside the root is accepted in
	// its resolved form, and a write through the validated path lands inside.
	for name, target := range map[string]string{
		"dangling-rel": filepath.Join("in", "created-rel.txt"),
		"dangling-abs": filepath.Join(in, "created-abs.txt"),
	} {
		fstree.MustSymlink(t, target, filepath.Join(root, name))
		want := filepath.Join(in, filepath.Base(target))
		got, err := validator.validate(root, filepath.Join(root, name))
		if err != nil || got != want {
			t.Fatalf("validate(%s) = (%q, %v), want %q", name, got, err, want)
		}
		if err := os.WriteFile(got, []byte("inside"), 0o600); err != nil {
			t.Fatalf("write through the validated path: %v", err)
		}
		if _, err := os.Stat(want); err != nil {
			t.Errorf("the dangling link's target was not created inside the root: %v", err)
		}
	}
}

func TestVirtualPathUnresolvableDenied(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	root := mustMkdir(t, filepath.Join(fstree.TempRoot(t), "root"))
	fstree.MustSymlink(t, "loop", filepath.Join(root, "loop"))
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, root)

	request := filepath.Join(root, "loop", "x")
	_, err := validator.validate(root, request)
	if !errors.Is(err, errVirtualPathTooManyLinks) || errors.Is(err, errVirtualPathDenied) {
		t.Fatalf("validate(link cycle) error = %v, want errVirtualPathTooManyLinks", err)
	}
	if !strings.Contains(err.Error(), request) {
		t.Errorf("the error does not name the path %s: %v", request, err)
	}
	if _, resolveErr := validator.resolver.resolve(request, root); !errors.Is(resolveErr, errVirtualPathTooManyLinks) {
		t.Errorf("resolve(link cycle) error = %v, want errVirtualPathTooManyLinks", resolveErr)
	}

	// Linux and macOS report ENOTDIR below a plain file; it is not "missing".
	// Windows reports not-exist there, and the request is judged by where the
	// file is (design D2).
	file := filepath.Join(root, "file")
	if writeErr := os.WriteFile(file, []byte("x"), 0o600); writeErr != nil {
		t.Fatalf("write file: %v", writeErr)
	}
	_, err = validator.validate(root, filepath.Join(file, "below"))
	if goruntime.GOOS == goosWindows {
		if err != nil {
			t.Errorf("validate(below a file) on Windows = %v, want the path accepted inside the root", err)
		}
		return
	}
	if !errors.Is(err, syscall.ENOTDIR) || errors.Is(err, errVirtualPathDenied) {
		t.Errorf("validate(below a file) error = %v, want ENOTDIR", err)
	}
}

func TestVirtualPathWithoutLinkKeepsText(t *testing.T) {
	t.Parallel()
	root := fstree.TempRoot(t)
	existing := mustMkdir(t, filepath.Join(root, "a", "b"))
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, root)
	for _, missing := range [][]string{nil, {"n1"}, {"n1", "n2", "n3"}} {
		request := pathTestJoin(existing, missing...)
		got, err := validator.validate(root, request)
		if err != nil || got != request {
			t.Errorf("validate(%s) = (%q, %v), want the cleaned input", request, got, err)
		}
		if got, err := normalizeExistingOrParent(request+string(filepath.Separator)+".", ""); err != nil || got != request {
			t.Errorf("normalizeExistingOrParent(%s/.) = (%q, %v), want %q", request, got, err, request)
		}
	}
	// A relative path is read from the given working directory.
	if got, err := normalizeExistingOrParent(filepath.Join("b", "n1"), filepath.Join(root, "a")); err != nil || got != filepath.Join(existing, "n1") {
		t.Errorf("normalizeExistingOrParent(relative) = (%q, %v), want %q", got, err, filepath.Join(existing, "n1"))
	}
	if _, err := normalizeExistingOrParent("  ", root); err == nil {
		t.Error("normalizeExistingOrParent accepted an empty path")
	}
}

func TestVirtualPathUnresolvableRootIsNotAllowed(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	module := mustMkdir(t, filepath.Join(base, "module"))
	cycle := filepath.Join(base, "cycle")
	fstree.MustSymlink(t, "cycle", cycle)

	// A workdir that cannot be resolved does not fail the construction; it is
	// only not an allowed root.
	resolver, err := newVirtualPathResolverForFilesystem(cycle, module, invowkfile.VirtualFilesystemConfig{
		Access: invowkfile.VirtualFilesystemAccessRestricted,
	})
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	if slices.ContainsFunc(resolver.allowedRoots, func(root string) bool { return strings.Contains(root, "cycle") }) {
		t.Fatalf("the unresolvable workdir is an allowed root: %v", resolver.allowedRoots)
	}
	if roots := normalizedRoots([]string{cycle, module}); !slices.Equal(roots, []string{module}) {
		t.Fatalf("normalizedRoots = %v, want only %s", roots, module)
	}
	// A request below it meets the same resolution error, not a path denial.
	_, err = virtualPathValidator{resolver: resolver}.validate(module, filepath.Join(cycle, "x"))
	if !errors.Is(err, errVirtualPathTooManyLinks) || errors.Is(err, errVirtualPathDenied) {
		t.Errorf("validate(below the unresolvable root) error = %v, want errVirtualPathTooManyLinks", err)
	}
}

func TestVirtualPathRootThroughSymlink(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	realDir := mustMkdir(t, filepath.Join(base, "real"))
	alias := filepath.Join(base, "alias")
	fstree.MustSymlink(t, realDir, alias)
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, alias)
	if !slices.Equal(validator.resolver.allowedRoots, []string{realDir}) {
		t.Fatalf("allowed roots = %v, want the link's target %s", validator.resolver.allowedRoots, realDir)
	}
	// Two missing components below the root, written through the link: the
	// pre-change code compared this request unresolved and denied it.
	got, err := validator.validate(alias, filepath.Join(alias, "new1", "new2"))
	if want := filepath.Join(realDir, "new1", "new2"); err != nil || got != want {
		t.Errorf("validate(alias/new1/new2) = (%q, %v), want %q", got, err, want)
	}
	if _, deniedErr := validator.validate(alias, filepath.Join(base, "elsewhere", "new1", "new2")); !errors.Is(deniedErr, errVirtualPathDenied) {
		t.Errorf("validate(outside the aliased root) error = %v, want errVirtualPathDenied", deniedErr)
	}

	// The always-allowed temporary root, as production lists it. On macOS /tmp
	// is a link to /private/tmp, so this is the real case of the shape above:
	// /tmp/<unique>/new1/new2 was wrongly denied before.
	tempRoots := defaultTempRoots()
	if len(tempRoots) == 0 {
		return // Windows has no fixed temporary root.
	}
	temp := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, tempRoots...)
	unique := fmt.Sprintf("invowk-path-test-%d-%s", os.Getpid(), filepath.Base(base))
	request := filepath.Join(tempRoots[0], unique, "new1", "new2")
	resolvedTemp, err := filepath.EvalSymlinks(tempRoots[0])
	if err != nil {
		t.Fatalf("resolve %s: %v", tempRoots[0], err)
	}
	got, err = temp.validate(base, request)
	if want := filepath.Join(resolvedTemp, unique, "new1", "new2"); err != nil || got != want {
		t.Errorf("validate(%s) = (%q, %v), want %q", request, got, err, want)
	}
	if goruntime.GOOS == "darwin" && !strings.HasPrefix(got, "/private/tmp/") {
		t.Errorf("validate(%s) = %q on macOS, want a path under /private/tmp", request, got)
	}
}

func TestVirtualPathBridgeResolvesDeclaredLink(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	work := mustMkdir(t, filepath.Join(base, "work"))
	module := mustMkdir(t, filepath.Join(base, "module"))
	realDir := mustMkdir(t, filepath.Join(base, "real"))
	link := filepath.Join(base, "link")
	fstree.MustSymlink(t, realDir, link)

	resolver, err := newVirtualPathResolverForFilesystem(work, module, invowkfile.VirtualFilesystemConfig{
		Access: invowkfile.VirtualFilesystemAccessRestricted,
		Paths:  invowkfile.VirtualFilesystemPaths{"DATA": invowkfile.VirtualFilesystemPath(link)},
	})
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	for request, want := range map[string]string{
		"DATA":       realDir,
		"DATA/n1/n2": filepath.Join(realDir, "n1", "n2"),
	} {
		if got, err := resolver.resolveBridgePath(request, work); err != nil || got != want {
			t.Errorf("resolveBridgePath(%s) = (%q, %v), want %q", request, got, err, want)
		}
	}
	env := map[string]string{}
	addVirtualRuntimeEnv(env, resolver)
	if env["INVOWK_PATH_DATA"] != realDir {
		t.Errorf("INVOWK_PATH_DATA = %q, want the resolved form %q", env["INVOWK_PATH_DATA"], realDir)
	}
}

func TestVirtualPathDeclaredPathBehindLink(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	work := mustMkdir(t, filepath.Join(base, "work"))
	module := mustMkdir(t, filepath.Join(base, "module"))
	realDir := mustMkdir(t, filepath.Join(base, "real"))
	link := filepath.Join(base, "link")
	fstree.MustSymlink(t, realDir, link)
	cycle := filepath.Join(base, "cycle")
	fstree.MustSymlink(t, "cycle", cycle)

	resolver, err := newVirtualPathResolverForFilesystem(work, module, invowkfile.VirtualFilesystemConfig{
		Access: invowkfile.VirtualFilesystemAccessRestricted,
		Paths:  invowkfile.VirtualFilesystemPaths{"DATA": invowkfile.VirtualFilesystemPath(link)},
	})
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	if !slices.Contains(resolver.allowedRoots, realDir) {
		t.Fatalf("the declared path's target %s is not an allowed root: %v", realDir, resolver.allowedRoots)
	}
	// Only the declared path as a root, so the temporary roots that enclose
	// t.TempDir do not decide the verdict.
	validator := virtualPathValidator{resolver: virtualPathResolver{
		paths:        resolver.paths,
		allowedRoots: normalizedRoots([]string{resolver.paths["DATA"]}),
		access:       invowkfile.VirtualFilesystemAccessRestricted,
	}}
	for _, request := range []string{filepath.Join(link, "n1", "n2"), "DATA/n1/n2"} {
		got, validateErr := validator.validate(work, request)
		if want := filepath.Join(realDir, "n1", "n2"); validateErr != nil || got != want {
			t.Errorf("validate(%s) = (%q, %v), want %q", request, got, validateErr, want)
		}
	}
	if _, deniedErr := validator.validate(work, filepath.Join(work, "n1", "n2")); !errors.Is(deniedErr, errVirtualPathDenied) {
		t.Errorf("validate(outside the declared path) error = %v, want errVirtualPathDenied", deniedErr)
	}

	// A declared path that cannot be resolved fails the construction under
	// restricted access, and the error names the entry.
	_, err = newVirtualPathResolverForFilesystem(work, module, invowkfile.VirtualFilesystemConfig{
		Access: invowkfile.VirtualFilesystemAccessRestricted,
		Paths:  invowkfile.VirtualFilesystemPaths{"LOOP": invowkfile.VirtualFilesystemPath(cycle)},
	})
	if !errors.Is(err, errVirtualPathTooManyLinks) || !strings.Contains(err.Error(), `"LOOP"`) {
		t.Errorf("declared link cycle: error = %v, want errVirtualPathTooManyLinks naming \"LOOP\"", err)
	}
}

func TestLuaRequireRefusesLinkOutOfScriptBase(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	module := mustMkdir(t, filepath.Join(base, "module"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	realPkg := mustMkdir(t, filepath.Join(module, "real"))
	for _, file := range []string{
		filepath.Join(outside, "evil.lua"),
		filepath.Join(outside, "mod.lua"),
		filepath.Join(realPkg, "ok.lua"),
	} {
		if err := os.WriteFile(file, []byte("return {}"), 0o600); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	fstree.MustSymlink(t, filepath.Join(outside, "evil.lua"), filepath.Join(module, "evil.lua"))
	fstree.MustSymlink(t, outside, filepath.Join(module, "pkg"))
	fstree.MustSymlink(t, "real", filepath.Join(module, "alias"))
	escaping := []string{"evil", "pkg.mod"}
	if ok, _ := fstree.Probe(t).Has(fstree.CapJunction); ok {
		// Windows: a junction out of the script base is refused like a symlink.
		fstree.MustJunction(t, outside, filepath.Join(module, "jpkg"))
		escaping = append(escaping, "jpkg.mod")
	}
	bridge := &luaRequireBridge{
		scriptBasePath: module,
		loaded:         map[string]luart.Value{},
		loading:        map[string]bool{},
	}

	for _, name := range escaping {
		path, source, err := bridge.resolveModule(name)
		if err == nil || !strings.Contains(err.Error(), "escapes script base path") {
			t.Errorf("resolveModule(%s) = (%q, %q, %v), want the escape error", name, path, source, err)
		}
	}
	path, source, err := bridge.resolveModule("alias.ok")
	if want := filepath.Join(realPkg, "ok.lua"); err != nil || path != want || string(source) != "return {}" {
		t.Errorf("resolveModule(alias.ok) = (%q, %q, %v), want %q", path, source, err, want)
	}
}

func TestVirtualPathFullAccessFallback(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	root := mustMkdir(t, filepath.Join(base, "root"))
	work := mustMkdir(t, filepath.Join(base, "work"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	dangling := filepath.Join(root, "dangling")
	fstree.MustSymlink(t, filepath.Join("..", "outside", "created.txt"), dangling)
	loop := filepath.Join(root, "loop")
	fstree.MustSymlink(t, "loop", loop)
	climb := filepath.Join(root, "climb")
	fstree.MustSymlink(t, rawLinkTarget("nope", "..", "x"), climb)
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// No allowed root at all: full access makes no containment decision.
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessFull)

	// A dangling link validates to the link's own path, and the operating
	// system follows it: the write creates the link's target.
	got, err := validator.validate(work, dangling)
	if err != nil || got != dangling {
		t.Fatalf("validate(dangling link) = (%q, %v), want the link's own path %q", got, err, dangling)
	}
	if writeErr := os.WriteFile(got, []byte("via link"), 0o600); writeErr != nil {
		t.Fatalf("write through the dangling link: %v", writeErr)
	}
	if content, readErr := os.ReadFile(filepath.Join(outside, "created.txt")); readErr != nil || string(content) != "via link" {
		t.Errorf("the dangling link's target = (%q, %v), want it created", content, readErr)
	}

	// Paths the resolver cannot express are not refused: a link cycle and a
	// ".." after a missing component keep the link's own path, and a file used
	// as a directory keeps the cleaned input.
	unresolvable := map[string]string{
		filepath.Join(loop, "x"): filepath.Join(loop, "x"),
		climb:                    climb,
	}
	if goruntime.GOOS != goosWindows {
		unresolvable[filepath.Join(file, "below")] = filepath.Join(file, "below")
	}
	for request, want := range unresolvable {
		if unresolved, validateErr := validator.validate(work, request); validateErr != nil || unresolved != want {
			t.Errorf("validate(%s) = (%q, %v), want %q", request, unresolved, validateErr, want)
		}
	}
	// An empty path is still an error.
	if _, emptyErr := validator.validate(work, ""); emptyErr == nil {
		t.Error("validate accepted an empty path under full access")
	}

	// Declared paths: a dangling link and a link cycle keep their own path,
	// and neither fails the construction.
	dangling2 := filepath.Join(root, "dangling2")
	fstree.MustSymlink(t, filepath.Join("..", "outside", "missing2"), dangling2)
	resolver, err := newVirtualPathResolverForFilesystem(work, work, invowkfile.VirtualFilesystemConfig{
		Access: invowkfile.VirtualFilesystemAccessFull,
		Paths: invowkfile.VirtualFilesystemPaths{
			"OUT":  invowkfile.VirtualFilesystemPath(dangling2),
			"LOOP": invowkfile.VirtualFilesystemPath(loop),
		},
	})
	if err != nil {
		t.Fatalf("build the full-access resolver with unresolvable declared paths: %v", err)
	}
	if resolver.paths["LOOP"] != loop || resolver.paths["OUT"] != dangling2 {
		t.Errorf("declared paths = %v, want LOOP=%q and OUT=%q (their own paths)", resolver.paths, loop, dangling2)
	}
	for request, want := range map[string]string{
		"OUT":      dangling2,
		"OUT/more": filepath.Join(dangling2, "more"),
	} {
		if got, err := resolver.resolveBridgePath(request, work); err != nil || got != want {
			t.Errorf("resolveBridgePath(%s) = (%q, %v), want %q", request, got, err, want)
		}
	}
}
