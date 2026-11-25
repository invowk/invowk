// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"mvdan.cc/sh/v3/interp"
)

const (
	// EnvVarStateBinPath is set to the most recent host binary resolved by a virtual runtime.
	EnvVarStateBinPath    = "INVOWK_STATE_BIN_PATH"
	goosWindows           = "windows"
	virtualAnchorCache    = "@cache"
	virtualAnchorConfig   = "@config"
	virtualAnchorData     = "@data"
	virtualAnchorHome     = "@home"
	virtualAnchorState    = "@state"
	virtualAnchorTmp      = "@tmp"
	virtualAnchorWork     = "@work"
	virtualStrictBin      = "/bin"
	virtualStrictLocalBin = "/usr/local/bin"
	virtualStrictUsrBin   = "/usr/bin"
	// virtualPathMaxLinks is the number of links one path walk may follow: the
	// bound filepath.EvalSymlinks uses.
	virtualPathMaxLinks = 255
	virtualPathParent   = ".."
)

var (
	errVirtualHostBinaryDenied = errors.New("virtual host binary denied")
	errVirtualPathDenied       = errors.New("virtual path denied")
	// errVirtualPathParentOfMissing: a link target holds ".." after a component
	// that does not exist. Nothing below a missing directory can be inspected,
	// so the ".." could only be applied to the path's text.
	errVirtualPathParentOfMissing = errors.New("virtual path has \"..\" after a missing component")
	// errVirtualPathNoExistingAncestor: the path's volume root does not exist
	// (a missing drive or UNC share).
	errVirtualPathNoExistingAncestor = errors.New("virtual path has no existing ancestor")
	errVirtualPathTooManyLinks       = errors.New("virtual path crosses too many links")
	errVirtualPathLinkTarget         = errors.New("virtual path link target is relative to a drive")
)

type (
	virtualPathResolver struct {
		anchors      map[string]string
		paths        map[string]string
		allowedRoots []string
		access       invowkfile.VirtualFilesystemAccess
	}

	virtualPathValidator struct {
		resolver virtualPathResolver
	}

	// virtualPathFS is the filesystem a path walk reads. Production code uses
	// hostVirtualPathFS; the fake-filesystem test injects entries that only
	// Windows can materialise (a junction, an unreadable reparse point).
	//
	//goplint:ignore -- injected OS lookups take and return raw OS path strings.
	virtualPathFS struct {
		lstat    func(name string) (fs.FileInfo, error)
		readlink func(name string) (string, error)
		// canonical puts an existing path that holds no link in the platform's
		// canonical form (on Windows: on-disk case, long names, drive letter).
		canonical func(path string) (string, error)
	}

	// virtualPathWalk is the result of walkVirtualPath.
	//
	//goplint:ignore -- raw OS path strings produced by the path walk.
	virtualPathWalk struct {
		// prefix is the deepest existing ancestor, canonical and with no link.
		prefix string
		// suffix holds the missing components below prefix: plain names only.
		suffix []string
		// unresolvedLink is set when the walk ended or failed while it read
		// components that came from a link target. It is the own path of the
		// outermost such link on the request: the canonical form of the link's
		// directory, the link's name, and the request's components after it.
		// Only the full-access fallback reads it.
		unresolvedLink string
	}

	// virtualPathWalker is the state of one walkVirtualPath call.
	//
	//goplint:ignore -- raw OS path strings and path components of one walk.
	virtualPathWalker struct {
		fs virtualPathFS
		// prefix exists and holds no link.
		prefix string
		// queue holds the components still to read. Its last own entries are
		// the request's own; the entries before them came from link targets.
		queue []string
		own   int
		links int
		// inLink is true while the walk reads components that came from a link
		// target. linkDir, linkName and linkRest describe the outermost link
		// being followed on the request's own path: its directory, its name,
		// and the request's components after it.
		inLink   bool
		linkDir  string
		linkName string
		linkRest []string
	}

	virtualHostBinaryPolicy struct {
		allowed  []string
		mode     invowkfile.BinaryLookupMode
		workDir  string
		envPath  string
		pathext  string
		stateEnv map[string]string
	}
)

func newVirtualPathValidator(ctx *ExecutionContext) (virtualPathValidator, error) {
	resolver, err := newVirtualPathResolver(ctx)
	if err != nil {
		return virtualPathValidator{}, err
	}
	return virtualPathValidator{resolver: resolver}, nil
}

func newVirtualPathResolver(ctx *ExecutionContext) (virtualPathResolver, error) {
	filesystem := selectedVirtualFilesystem(ctx)
	return newVirtualPathResolverForFilesystem(
		virtualWorkdirFor(ctx.Invowkfile, ctx.Command, ctx.SelectedImpl, ctx.WorkDir),
		string(ctx.Invowkfile.GetScriptBasePath()),
		filesystem,
	)
}

func selectedVirtualFilesystem(ctx *ExecutionContext) invowkfile.VirtualFilesystemConfig {
	if ctx == nil || ctx.SelectedImpl == nil {
		return invowkfile.VirtualFilesystemConfig{}
	}
	platform := invowkfile.CurrentPlatform()
	if ctx.SelectedPlatform != "" {
		platform = ctx.SelectedPlatform
	}
	return ctx.SelectedImpl.VirtualFilesystemForPlatform(platform)
}

// newVirtualPathResolverForFilesystem builds the resolver of a virtual command.
// The workdir and the @work anchor are allowed roots only when
// workdir.isReadRoot holds; @work still reports the workdir either way.
func newVirtualPathResolverForFilesystem(
	workdir virtualWorkdir,
	scriptBasePath string,
	filesystem invowkfile.VirtualFilesystemConfig,
) (virtualPathResolver, error) {
	anchors := standardVirtualAnchors(workdir.path)
	paths, err := resolveVirtualFilesystemPaths(filesystem.Paths, scriptBasePath, anchors, filesystem.EffectiveAccess())
	if err != nil {
		return virtualPathResolver{}, err
	}
	roots := []string{scriptBasePath, anchors[virtualAnchorTmp]}
	rootAnchors := []string{virtualAnchorConfig, virtualAnchorData, virtualAnchorCache, virtualAnchorState}
	if workdir.isReadRoot() {
		roots = append(roots, workdir.path)
		rootAnchors = append(rootAnchors, virtualAnchorWork)
	}
	for _, name := range rootAnchors {
		if path := anchors[name]; path != "" {
			roots = append(roots, path)
		}
	}
	for _, path := range paths {
		roots = append(roots, path)
	}
	roots = append(roots, defaultTempRoots()...)
	return virtualPathResolver{
		anchors:      anchors,
		paths:        paths,
		allowedRoots: normalizedRoots(roots),
		access:       filesystem.EffectiveAccess(),
	}, nil
}

// newVirtualPathResolverForInteractiveConfig builds the resolver of an
// interactive subprocess from its flags. workDirModuleRoot is the parent's
// virtualWorkdir.moduleRoot ("" for a trusted workdir), so the child makes the
// same root decision as the parent.
func newVirtualPathResolverForInteractiveConfig(
	workDir, workDirModuleRoot, scriptBasePath string,
	filesystem invowkfile.VirtualFilesystemConfig,
) (virtualPathResolver, error) {
	workdir := virtualWorkdir{path: workDir, moduleRoot: workDirModuleRoot}
	resolver, err := newVirtualPathResolverForFilesystem(workdir, scriptBasePath, filesystem)
	if err != nil {
		return virtualPathResolver{}, err
	}
	return resolver, nil
}

func defaultTempRoots() []string {
	if runtime.GOOS == goosWindows {
		return nil
	}
	return []string{"/tmp"}
}

func standardVirtualAnchors(workDir string) map[string]string {
	return standardVirtualAnchorsForOS(runtime.GOOS, workDir, os.TempDir(), basedir.OS())
}

// standardVirtualAnchorsForOS builds the anchor map for goos. Every base comes
// from the shared resolver, so it is absolute: a relative XDG or AppData
// variable falls back to the home-based default, and an anchor whose base is
// unavailable (a relative or unset home) is "". An empty anchor is never an
// allow root and is an error when used (expandVirtualAnchorPath).
//
// The name is bound by the VirtualPathHarness correspondence table
// (formal/alloy/VirtualPathHarness.als); keep it.
func standardVirtualAnchorsForOS(goos, workDir, tempDir string, dirs basedir.Dirs) map[string]string {
	anchors := map[string]string{
		virtualAnchorHome: availableBase(dirs.Home()),
		virtualAnchorTmp:  tempDir,
		virtualAnchorWork: workDir,
	}
	config := availableBase(dirs.ConfigHome())
	data := availableBase(dirs.DataHome())
	cache := availableBase(dirs.CacheHome())
	state := availableBase(dirs.StateHome())
	switch goos {
	case goosWindows:
		anchors[virtualAnchorConfig] = joinAvailable(config, "invowk", "config")
		anchors[virtualAnchorData] = joinAvailable(data, "invowk", "data")
		anchors[virtualAnchorCache] = joinAvailable(cache, "invowk", "cache")
		anchors[virtualAnchorState] = joinAvailable(state, "invowk", "state")
	default:
		anchors[virtualAnchorConfig] = joinAvailable(config, "invowk")
		anchors[virtualAnchorData] = joinAvailable(data, "invowk")
		anchors[virtualAnchorCache] = joinAvailable(cache, "invowk")
		anchors[virtualAnchorState] = joinAvailable(state, "invowk")
	}
	return anchors
}

// availableBase returns the resolved base, or "" when it is unavailable.
//
//goplint:ignore -- anchor maps hold raw OS path strings; "" marks an unavailable anchor.
func availableBase(dir types.FilesystemPath, err error) string {
	if err != nil {
		return ""
	}
	return string(dir)
}

// joinAvailable joins elems under base, keeping an unavailable base "".
//
//goplint:ignore -- raw OS path strings for the anchor map.
func joinAvailable(base string, elems ...string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, elems...)...)
}

// normalizedRoots resolves the allowed roots with the strict resolver, the one
// a request is resolved with, so a root given through a link or junction is
// its target and compares with requests written through the same link. A root
// that cannot be resolved is dropped: a request below it meets the same error.
func normalizedRoots(paths []string) []string {
	roots := make([]string, 0, len(paths))
	for _, path := range paths {
		root, err := normalizeExistingOrParent(path, "")
		if err != nil || root == "" || slices.Contains(roots, root) {
			continue
		}
		roots = append(roots, root)
	}
	return roots
}

// resolveVirtualFilesystemPaths resolves the declared filesystem.paths of the
// selected platform. Their resolved form is what INVOWK_PATH_* and invowk.path
// report, and each is an allowed root. Under restricted access an entry that
// cannot be resolved is an error that names the entry, so the command fails
// before its script runs; under full access the entry keeps its fallback path
// (normalizeVirtualPathForAccess).
func resolveVirtualFilesystemPaths(
	filesystemPaths invowkfile.VirtualFilesystemPaths,
	scriptBasePath string,
	anchors map[string]string,
	access invowkfile.VirtualFilesystemAccess,
) (map[string]string, error) {
	if len(filesystemPaths) == 0 {
		return map[string]string{}, nil
	}
	paths := make(map[string]string, len(filesystemPaths))
	for name, rawPath := range filesystemPaths {
		expanded, err := expandVirtualAnchorPath(rawPath.String(), anchors)
		if err != nil {
			return nil, err
		}
		normalized, err := normalizeVirtualPathForAccess(expanded, scriptBasePath, access)
		if err != nil {
			return nil, fmt.Errorf("virtual filesystem path %q: %w", name.String(), err)
		}
		paths[name.String()] = normalized
	}
	return paths, nil
}

func (v virtualPathValidator) validate(cwd, path string) (string, error) {
	normalized, err := v.resolver.resolve(path, cwd)
	if err != nil {
		return "", err
	}
	if v.resolver.access == invowkfile.VirtualFilesystemAccessFull {
		return normalized, nil
	}
	for _, root := range v.resolver.allowedRoots {
		if pathWithin(root, normalized) {
			return normalized, nil
		}
	}
	return "", virtualPolicyDenyError(errVirtualPathDenied, normalized)
}

func (r virtualPathResolver) resolve(path, cwd string) (string, error) {
	expanded, err := r.expand(path)
	if err != nil {
		return "", err
	}
	return normalizeVirtualPathForAccess(expanded, cwd, r.access)
}

func (r virtualPathResolver) resolveBridgePath(path, cwd string) (string, error) {
	if strings.HasPrefix(path, "@") {
		return r.resolve(path, cwd)
	}
	name, suffix := splitVirtualAnchorPath(path)
	root, ok := r.paths[name]
	if !ok || root == "" {
		return "", fmt.Errorf("unknown virtual path %q", name)
	}
	if suffix != "" {
		root = filepath.Join(root, suffix)
	}
	return normalizeVirtualPathForAccess(root, cwd, r.access)
}

func (r virtualPathResolver) expand(path string) (string, error) {
	if strings.HasPrefix(path, "@") {
		return expandVirtualAnchorPath(path, r.anchors)
	}
	name, suffix := splitVirtualAnchorPath(path)
	root, ok := r.paths[name]
	if !ok || root == "" {
		return path, nil
	}
	if suffix == "" {
		return root, nil
	}
	return filepath.Join(root, suffix), nil
}

func expandVirtualAnchorPath(path string, anchors map[string]string) (string, error) {
	if !strings.HasPrefix(path, "@") {
		return path, nil
	}
	name, suffix := splitVirtualAnchorPath(path)
	root, ok := anchors[name]
	if !ok {
		return "", fmt.Errorf("unknown virtual path anchor %q", name)
	}
	if root == "" {
		return "", fmt.Errorf("virtual path anchor %q is unavailable: its base directory is relative or unset", name)
	}
	if suffix == "" {
		return root, nil
	}
	return filepath.Join(root, suffix), nil
}

func addVirtualRuntimeEnv(env map[string]string, resolver virtualPathResolver) {
	env[EnvVarStateBinPath] = ""
	for name, path := range resolver.anchors {
		key := "INVOWK_ANCHOR_" + strings.ToUpper(strings.TrimPrefix(name, "@"))
		env[key] = path
	}
	for name, path := range resolver.paths {
		env["INVOWK_PATH_"+name] = path
	}
}

func splitVirtualAnchorPath(path string) (name, suffix string) {
	index := strings.IndexAny(path, `/\`)
	if index == -1 {
		return path, ""
	}
	return path[:index], path[index+1:]
}

// normalizeExistingOrParent resolves a path physically, for every containment
// decision: the validator under restricted access, the allowed roots, and Lua
// require. It makes the path absolute (against cwd, or the process working
// directory when cwd is empty), walks it through every symbolic link and
// Windows junction to the deepest existing ancestor (walkVirtualPath), and
// returns that ancestor joined with the missing components. The result holds
// no link, no "." and no "..".
//
// It has no fallback: an empty path, a failed os.Getwd, and every failure of
// the walk (an error other than "does not exist", an unreadable link, too many
// links, a ".." after a missing component, a missing volume root) is an error
// that names the path, and the caller denies. Only full access, which makes no
// containment decision, may use another form (normalizeVirtualPathForAccess).
//
// The name is historical and is bound by the VirtualPathHarness correspondence
// table (formal/alloy/VirtualPathHarness.als); keep it.
func normalizeExistingOrParent(path, cwd string) (string, error) {
	cleaned, err := absoluteVirtualPath(path, cwd)
	if err != nil {
		return "", err
	}
	walk, err := walkVirtualPath(hostVirtualPathFS(), cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve virtual path %s: %w", cleaned, err)
	}
	return walk.path(), nil
}

// normalizeVirtualPathForAccess is normalizeExistingOrParent for the callers
// that only put a path in the form an operation runs on: resolve,
// resolveBridgePath and declared filesystem.paths. Under restricted access it
// is exactly the strict resolver. Full access has no allowed-root check, so
// there a path the resolver cannot express is handed to the operating system
// instead of being refused:
//   - when the walk ended or failed inside a link's target (a dangling link, a
//     link cycle, /dev/stderr on a pipe), the link's own path, which the
//     system follows when it opens it;
//   - when the walk failed anywhere else, the cleaned absolute path, for which
//     the system reports its own error.
//
// An empty path and a failed os.Getwd stay errors. No containment decision
// reads a fallback form: validate returns before the allowed-root check under
// full access, and the allowed roots and Lua require call the strict resolver.
//
//goplint:ignore -- filesystem boundary helper: raw OS path strings in and out.
func normalizeVirtualPathForAccess(path, cwd string, access invowkfile.VirtualFilesystemAccess) (string, error) {
	if access != invowkfile.VirtualFilesystemAccessFull {
		return normalizeExistingOrParent(path, cwd)
	}
	cleaned, err := absoluteVirtualPath(path, cwd)
	if err != nil {
		return "", err
	}
	walk, err := walkVirtualPath(hostVirtualPathFS(), cleaned)
	switch {
	case walk.unresolvedLink != "":
		return walk.unresolvedLink, nil
	case err != nil:
		// Not refused: full access makes no containment decision, and the
		// operating system reports its own error for this path.
		return cleaned, nil //nolint:nilerr // the fallback form is the documented result
	default:
		return walk.path(), nil
	}
}

// absoluteVirtualPath cleans path and makes it absolute against cwd, or
// against the process working directory when cwd is empty. A relative cwd is
// itself taken against the process working directory, so the walk never reads
// a relative path as one rooted at the volume root.
//
//goplint:ignore -- filesystem boundary helper: raw OS path strings in and out.
func absoluteVirtualPath(path, cwd string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path must not be empty")
	}
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}
	joined := filepath.Join(cwd, cleaned)
	if filepath.IsAbs(joined) {
		return joined, nil
	}
	absolute, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}
	return absolute, nil
}

// hostVirtualPathFS reads the host filesystem.
func hostVirtualPathFS() virtualPathFS {
	return virtualPathFS{lstat: os.Lstat, readlink: os.Readlink, canonical: filepath.EvalSymlinks}
}

// walkVirtualPath resolves a cleaned absolute path physically. It walks the
// components from the volume root and follows every link: a symbolic link, and
// on Windows a junction, which filepath.EvalSymlinks does not follow. It never
// inspects a component through an unresolved link, so "does not exist" is
// reliable on every platform.
//
// The walk ends at the first missing component. The result is then the
// deepest existing ancestor (prefix: canonical, no link) and the missing
// components (suffix: plain names). A ".." among the missing components is an
// error: it could only be applied to the path's text, and the name after it
// would never be inspected. Every error other than "does not exist" is
// returned; the caller decides whether that denies.
//
// The name is bound by the VirtualPathHarness correspondence table
// (formal/alloy/VirtualPathHarness.als); keep it.
//
//goplint:ignore -- filesystem boundary helper: raw OS path strings in and out.
func walkVirtualPath(vfs virtualPathFS, cleaned string) (virtualPathWalk, error) {
	volume := filepath.VolumeName(cleaned)
	walker := virtualPathWalker{fs: vfs, queue: splitVirtualPathComponents(cleaned[len(volume):])}
	walker.own = len(walker.queue)
	suffix, err := walker.walk(volume)
	result := virtualPathWalk{unresolvedLink: walker.unresolvedLink()}
	if err != nil {
		return result, err
	}
	prefix, err := vfs.canonical(walker.prefix)
	if err != nil {
		return result, fmt.Errorf("canonical form of %q: %w", walker.prefix, err)
	}
	result.prefix = prefix
	result.suffix = suffix
	return result, nil
}

// path returns the resolved path: the existing prefix joined with the missing
// names.
//
//goplint:ignore -- returns a raw OS path string.
func (w virtualPathWalk) path() string {
	return filepath.Join(append([]string{w.prefix}, w.suffix...)...)
}

// walk reads the queue until it is empty or a component is missing, and
// returns the missing components.
//
//goplint:ignore -- raw volume name in, raw path components out.
func (w *virtualPathWalker) walk(volume string) ([]string, error) {
	if err := w.enterVolume(volume); err != nil {
		return nil, err
	}
	for len(w.queue) > 0 {
		name := w.next()
		switch name {
		case ".":
			continue
		case virtualPathParent:
			// Only a link target can hold "..": the request was cleaned. The
			// prefix is physical, and a volume root is its own parent.
			w.prefix = filepath.Dir(w.prefix)
			continue
		}
		path := filepath.Join(w.prefix, name)
		info, err := w.fs.lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return missingVirtualPathSuffix(name, w.queue)
		}
		if err != nil {
			return nil, err
		}
		if !isVirtualPathLink(info.Mode()) {
			w.prefix = path
			continue
		}
		if err := w.follow(path, name); err != nil {
			return nil, err
		}
	}
	w.inLink = false
	return nil, nil
}

// next takes the first component off the queue. Taking one of the request's
// own components means every link followed so far was resolved.
//
//goplint:ignore -- returns one raw path component.
func (w *virtualPathWalker) next() string {
	name := w.queue[0]
	fromLink := len(w.queue) > w.own
	w.queue = w.queue[1:]
	if !fromLink {
		w.own--
		w.inLink = false
	}
	return name
}

// enterVolume puts the prefix at the root of volume. A root that does not
// exist is a missing drive or UNC share: the path has no existing ancestor.
//
//goplint:ignore -- raw OS volume name.
func (w *virtualPathWalker) enterVolume(volume string) error {
	root := volume + string(filepath.Separator)
	if _, err := w.fs.lstat(root); err != nil {
		if isVirtualPathMissingVolume(err) {
			return fmt.Errorf("%w: %s", errVirtualPathNoExistingAncestor, root)
		}
		return err
	}
	w.prefix = root
	return nil
}

// follow reads the link at path and puts its target's components at the front
// of the queue. The target is never cleaned as text: its components are read
// one by one like any others, so a ".." in it is applied to a physical prefix.
//
//goplint:ignore -- raw OS path of the link and its raw name.
func (w *virtualPathWalker) follow(path, name string) error {
	// os.Readlink reports ENOENT for a reparse point that is not a link, so a
	// not-exist error here is never a missing component.
	target, err := w.fs.readlink(path)
	if err != nil {
		return fmt.Errorf("read link target: %w", err)
	}
	w.links++
	if w.links > virtualPathMaxLinks {
		return fmt.Errorf("%w: %s", errVirtualPathTooManyLinks, path)
	}
	if !w.inLink {
		w.inLink = true
		w.linkDir = w.prefix
		w.linkName = name
		w.linkRest = slices.Clone(w.queue)
	}
	volume := filepath.VolumeName(target)
	rest := target[len(volume):]
	rooted := rest != "" && os.IsPathSeparator(rest[0])
	switch {
	case rooted || filepath.IsAbs(target):
		if volume == "" {
			// A rooted target with no volume (Windows) stays on the volume of
			// the link's directory.
			volume = filepath.VolumeName(w.prefix)
		}
		if err := w.enterVolume(volume); err != nil {
			return err
		}
	case volume != "":
		return fmt.Errorf("%w: %s -> %s", errVirtualPathLinkTarget, path, target)
	}
	w.queue = append(splitVirtualPathComponents(rest), w.queue...)
	return nil
}

// unresolvedLink returns the own path of the outermost link the walk was
// following when it ended or failed, or "" when it was following none.
//
//goplint:ignore -- returns a raw OS path string; "" means no unresolved link.
func (w *virtualPathWalker) unresolvedLink() string {
	if !w.inLink {
		return ""
	}
	dir, err := w.fs.canonical(w.linkDir)
	if err != nil {
		return ""
	}
	return filepath.Join(append([]string{dir, w.linkName}, w.linkRest...)...)
}

// missingVirtualPathSuffix returns the missing component and the components
// after it as plain names.
//
//goplint:ignore -- raw path components of one walk.
func missingVirtualPathSuffix(missing string, rest []string) ([]string, error) {
	suffix := []string{missing}
	for _, name := range rest {
		switch name {
		case ".":
		case virtualPathParent:
			return nil, errVirtualPathParentOfMissing
		default:
			suffix = append(suffix, name)
		}
	}
	return suffix, nil
}

// isVirtualPathLink reports whether an entry is a link the walk follows: a
// symbolic link, or an irregular entry that is not a directory. On Windows
// os.Lstat reports ModeIrregular, and neither ModeSymlink nor ModeDir, for a
// name-surrogate reparse point such as a junction, and also for every other
// reparse tag except SYMLINK, AF_UNIX and DEDUP (an AppExecLink alias, for
// example). The walk treats all of them as links: one whose target os.Readlink
// cannot read fails the walk, so it is refused under restricted access.
func isVirtualPathLink(mode fs.FileMode) bool {
	return mode&fs.ModeSymlink != 0 || (mode&fs.ModeIrregular != 0 && mode&fs.ModeDir == 0)
}

// splitVirtualPathComponents splits the part of a path after its volume name.
//
//goplint:ignore -- raw OS path string split into its components.
func splitVirtualPathComponents(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool {
		return r < utf8.RuneSelf && os.IsPathSeparator(uint8(r))
	})
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || rel == "" || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func (v virtualPathValidator) openHandler(next interp.OpenHandlerFunc) interp.OpenHandlerFunc {
	return func(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
		hc := interp.HandlerCtx(ctx)
		normalized, err := v.validate(hc.Dir, path)
		if err != nil {
			return nil, &os.PathError{Op: "open", Path: path, Err: err}
		}
		return next(ctx, normalized, flag, perm)
	}
}

func (v virtualPathValidator) readDirHandler(next interp.ReadDirHandlerFunc2) interp.ReadDirHandlerFunc2 {
	return func(ctx context.Context, path string) ([]fs.DirEntry, error) {
		hc := interp.HandlerCtx(ctx)
		normalized, err := v.validate(hc.Dir, path)
		if err != nil {
			return nil, &os.PathError{Op: "readdir", Path: path, Err: err}
		}
		return next(ctx, normalized)
	}
}

// statHandler validates the path of every stat the shell makes. A stat that
// follows symbolic links (cd, and every file test but -L and -h) validates it
// with validate; one that does not (-L, -h, and the access emulation on
// non-Unix platforms, which accessHandler hands the resolved path) validates
// it with validateNoFollow, so the test sees the link itself.
func (v virtualPathValidator) statHandler(next interp.StatHandlerFunc) interp.StatHandlerFunc {
	return func(ctx context.Context, path string, followSymlinks bool) (fs.FileInfo, error) {
		hc := interp.HandlerCtx(ctx)
		check := v.validate
		if !followSymlinks {
			check = v.validateNoFollow
		}
		normalized, err := check(hc.Dir, path)
		if err != nil {
			return nil, &os.PathError{Op: "stat", Path: path, Err: err}
		}
		return next(ctx, normalized, followSymlinks)
	}
}

func selectedRuntimeConfig(ctx *ExecutionContext) *invowkfile.RuntimeConfig {
	if ctx == nil || ctx.SelectedImpl == nil {
		return nil
	}
	return invowkfile.FindRuntimeConfig(ctx.SelectedImpl.Runtimes, ctx.SelectedRuntime)
}

func hostBinaryPolicy(ctx *ExecutionContext, env map[string]string) *virtualHostBinaryPolicy {
	cfg := selectedRuntimeConfig(ctx)
	return &virtualHostBinaryPolicy{
		allowed:  allowedBinaryStrings(cfg),
		mode:     binaryLookupMode(cfg),
		workDir:  ctx.EffectiveWorkDir(),
		envPath:  env["PATH"],
		pathext:  env["PATHEXT"],
		stateEnv: env,
	}
}

func allowedBinaryStrings(cfg *invowkfile.RuntimeConfig) []string {
	if cfg == nil {
		return nil
	}
	allowed := make([]string, 0, len(cfg.AllowedBinaries))
	for _, binary := range cfg.AllowedBinaries {
		allowed = append(allowed, binary.String())
	}
	return allowed
}

func binaryLookupMode(cfg *invowkfile.RuntimeConfig) invowkfile.BinaryLookupMode {
	if cfg == nil || cfg.BinaryLookupMode == "" {
		return invowkfile.BinaryLookupModeHost
	}
	return cfg.BinaryLookupMode
}

func (p *virtualHostBinaryPolicy) resolve(name string) (string, error) {
	if len(p.allowed) == 0 {
		return "", virtualPolicyDenyError(errVirtualHostBinaryDenied, name)
	}
	path, err := p.lookup(name)
	if err != nil {
		return "", err
	}
	if p.allows(name, path) {
		if p.stateEnv != nil {
			p.stateEnv[EnvVarStateBinPath] = path
		}
		return path, nil
	}
	return "", virtualPolicyDenyError(errVirtualHostBinaryDenied, name)
}

//goplint:ignore -- virtual policy diagnostics include raw denied path/binary text for actionable errors.
func virtualPolicyDenyError(err error, value string) error {
	return fmt.Errorf("%w: %s", err, value)
}

func (p *virtualHostBinaryPolicy) lookup(name string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		info, err := os.Stat(name)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("%q is a directory", name)
		}
		return name, nil
	}
	for _, dir := range p.lookupDirs() {
		for _, candidate := range candidateExecutablePaths(dir, name, p.pathext) {
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() && isExecutable(info) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%q: executable file not found in virtual binary lookup path", name)
}

func (p *virtualHostBinaryPolicy) lookupDirs() []string {
	if p.mode == invowkfile.BinaryLookupModeStrict {
		if runtime.GOOS == goosWindows {
			return []string{`C:\Windows\System32`}
		}
		return []string{virtualStrictLocalBin, virtualStrictUsrBin, virtualStrictBin}
	}
	if p.envPath == "" {
		return nil
	}
	dirs := filepath.SplitList(p.envPath)
	for i, dir := range dirs {
		if dir == "" {
			dirs[i] = p.workDir
		}
	}
	return dirs
}

func candidateExecutablePaths(dir, name, pathext string) []string {
	if runtime.GOOS != goosWindows || filepath.Ext(name) != "" {
		return []string{filepath.Join(dir, name)}
	}
	exts := filepath.SplitList(pathext)
	if len(exts) == 0 {
		exts = []string{".com", ".exe", ".bat", ".cmd"}
	}
	candidates := make([]string, 0, len(exts)+1)
	candidates = append(candidates, filepath.Join(dir, name))
	for _, ext := range exts {
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		candidates = append(candidates, filepath.Join(dir, name+ext))
	}
	return candidates
}

func isExecutable(info os.FileInfo) bool {
	if runtime.GOOS == goosWindows {
		return true
	}
	return info.Mode()&0o111 != 0
}

func (p *virtualHostBinaryPolicy) allows(name, resolved string) bool {
	for _, allowed := range p.allowed {
		switch {
		case allowed == "*":
			return true
		case filepath.IsAbs(allowed):
			if sameCleanPath(allowed, resolved) {
				return true
			}
		case allowed == name || allowed == filepath.Base(resolved):
			return true
		}
	}
	return false
}

func sameCleanPath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
