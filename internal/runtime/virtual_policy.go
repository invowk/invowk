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

	"github.com/invowk/invowk/pkg/invowkfile"

	"mvdan.cc/sh/v3/interp"
)

const (
	// EnvVarStateBinPath is set to the most recent host binary resolved by a virtual runtime.
	EnvVarStateBinPath = "INVOWK_STATE_BIN_PATH"
	goosWindows        = "windows"
)

var (
	errVirtualHostBinaryDenied = errors.New("virtual host binary denied")
	errVirtualPathDenied       = errors.New("virtual path denied")
)

type (
	virtualPathResolver struct {
		anchors      map[string]string
		paths        map[string]string
		allowedRoots []string
	}

	virtualPathValidator struct {
		resolver virtualPathResolver
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
	allowedPaths := invowkfile.AllowedPaths(nil)
	if ctx != nil && ctx.SelectedImpl != nil {
		allowedPaths = ctx.SelectedImpl.AllowedPaths
	}
	return newVirtualPathResolverForAllowedPaths(
		ctx.EffectiveWorkDir(),
		string(ctx.Invowkfile.GetScriptBasePath()),
		allowedPaths,
		invowkfile.CurrentPlatform(),
	)
}

func newVirtualPathResolverForPaths(workDir, scriptBasePath string) virtualPathResolver {
	resolver, err := newVirtualPathResolverForAllowedPaths(workDir, scriptBasePath, nil, "")
	if err != nil {
		return virtualPathResolver{}
	}
	return resolver
}

func newVirtualPathResolverForAllowedPaths(
	workDir string,
	scriptBasePath string,
	allowedPaths invowkfile.AllowedPaths,
	platform invowkfile.PlatformType,
) (virtualPathResolver, error) {
	anchors := standardVirtualAnchors(workDir)
	paths, err := resolveAllowedPaths(allowedPaths, platform, scriptBasePath, anchors)
	if err != nil {
		return virtualPathResolver{}, err
	}
	roots := []string{workDir, scriptBasePath, anchors["@tmp"]}
	for _, name := range []string{"@config", "@data", "@cache", "@state", "@work"} {
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
	}, nil
}

func newVirtualPathResolverForEnv(workDir, scriptBasePath string, env map[string]string) virtualPathResolver {
	resolver := newVirtualPathResolverForPaths(workDir, scriptBasePath)
	if len(env) == 0 {
		return resolver
	}
	resolver.paths = make(map[string]string)
	for key, value := range env {
		if !strings.HasPrefix(key, "INVOWK_PATH_") || strings.TrimSpace(value) == "" {
			continue
		}
		name := strings.TrimPrefix(key, "INVOWK_PATH_")
		resolver.paths[name] = value
		resolver.allowedRoots = append(resolver.allowedRoots, normalizedRoots([]string{value})...)
	}
	return resolver
}

func defaultTempRoots() []string {
	if runtime.GOOS == goosWindows {
		return nil
	}
	return []string{"/tmp"}
}

func standardVirtualAnchors(workDir string) map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return standardVirtualAnchorsForOS(runtime.GOOS, workDir, home, os.TempDir(), os.Getenv)
}

func standardVirtualAnchorsForOS(
	goos string,
	workDir string,
	home string,
	tempDir string,
	getenv func(string) string,
) map[string]string {
	get := func(name string) string {
		if getenv == nil {
			return ""
		}
		return getenv(name)
	}
	anchors := map[string]string{
		"@home": home,
		"@tmp":  tempDir,
		"@work": workDir,
	}
	switch goos {
	case goosWindows:
		roaming := firstNonEmpty(get("APPDATA"), filepath.Join(home, "AppData", "Roaming"))
		local := firstNonEmpty(get("LOCALAPPDATA"), filepath.Join(home, "AppData", "Local"))
		anchors["@config"] = filepath.Join(roaming, "invowk", "config")
		anchors["@data"] = filepath.Join(local, "invowk", "data")
		anchors["@cache"] = filepath.Join(local, "invowk", "cache")
		anchors["@state"] = filepath.Join(local, "invowk", "state")
	case "darwin":
		anchors["@config"] = filepath.Join(home, "Library", "Application Support", "invowk")
		anchors["@data"] = filepath.Join(home, "Library", "Application Support", "invowk")
		anchors["@cache"] = filepath.Join(home, "Library", "Caches", "invowk")
		anchors["@state"] = filepath.Join(home, "Library", "Logs", "invowk")
	default:
		anchors["@config"] = filepath.Join(firstNonEmpty(get("XDG_CONFIG_HOME"), filepath.Join(home, ".config")), "invowk")
		anchors["@data"] = filepath.Join(firstNonEmpty(get("XDG_DATA_HOME"), filepath.Join(home, ".local", "share")), "invowk")
		anchors["@cache"] = filepath.Join(firstNonEmpty(get("XDG_CACHE_HOME"), filepath.Join(home, ".cache")), "invowk")
		anchors["@state"] = filepath.Join(firstNonEmpty(get("XDG_STATE_HOME"), filepath.Join(home, ".local", "state")), "invowk")
	}
	return anchors
}

func firstNonEmpty(first, second string) string {
	if first != "" {
		return first
	}
	return second
}

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

func resolveAllowedPaths(
	allowedPaths invowkfile.AllowedPaths,
	platform invowkfile.PlatformType,
	scriptBasePath string,
	anchors map[string]string,
) (map[string]string, error) {
	if len(allowedPaths) == 0 {
		return map[string]string{}, nil
	}
	paths := make(map[string]string, len(allowedPaths))
	for name := range allowedPaths {
		rawPath, ok, err := allowedPaths.PathForPlatform(name, platform)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("allowed_paths[%q] has no %q mapping", name, platform)
		}
		expanded, err := expandVirtualAnchorPath(rawPath, anchors)
		if err != nil {
			return nil, err
		}
		normalized, err := normalizeExistingOrParent(expanded, scriptBasePath)
		if err != nil {
			return nil, err
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
	for _, root := range v.resolver.allowedRoots {
		if pathWithin(root, normalized) {
			return normalized, nil
		}
	}
	return "", fmt.Errorf("%w: %s", errVirtualPathDenied, normalized)
}

func (r virtualPathResolver) resolve(path, cwd string) (string, error) {
	expanded, err := r.expand(path)
	if err != nil {
		return "", err
	}
	return normalizeExistingOrParent(expanded, cwd)
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
	return normalizeExistingOrParent(root, cwd)
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
	if !ok || root == "" {
		return "", fmt.Errorf("unknown virtual path anchor %q", name)
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

func normalizeExistingOrParent(path, cwd string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path must not be empty")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("get working directory: %w", err)
			}
		}
		cleaned = filepath.Join(cwd, cleaned)
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved, nil
	}
	parent := filepath.Dir(cleaned)
	if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
		return filepath.Join(resolvedParent, filepath.Base(cleaned)), nil
	}
	return cleaned, nil
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

func (v virtualPathValidator) statHandler(next interp.StatHandlerFunc) interp.StatHandlerFunc {
	return func(ctx context.Context, path string, followSymlinks bool) (fs.FileInfo, error) {
		hc := interp.HandlerCtx(ctx)
		normalized, err := v.validate(hc.Dir, path)
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
		return "", fmt.Errorf("%w: %s", errVirtualHostBinaryDenied, name)
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
	return "", fmt.Errorf("%w: %s", errVirtualHostBinaryDenied, name)
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
		return []string{"/usr/local/bin", "/usr/bin", "/bin"}
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
