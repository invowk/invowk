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
)

var (
	errVirtualHostBinaryDenied = errors.New("virtual host binary denied")
	errVirtualPathDenied       = errors.New("virtual path denied")
)

type (
	virtualPathValidator struct {
		roots []string
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

func newVirtualPathValidator(ctx *ExecutionContext) virtualPathValidator {
	roots := []string{ctx.EffectiveWorkDir(), string(ctx.Invowkfile.GetScriptBasePath()), os.TempDir()}
	roots = append(roots, defaultTempRoots()...)
	roots = append(roots, platformConfigDirs()...)
	return virtualPathValidator{roots: normalizedRoots(roots)}
}

func defaultTempRoots() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []string{"/tmp"}
}

func platformConfigDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		var roots []string
		for _, env := range []string{"APPDATA", "LOCALAPPDATA"} {
			if value := os.Getenv(env); value != "" {
				roots = append(roots, value)
			}
		}
		return roots
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support"),
			filepath.Join(home, "Library", "Caches"),
		}
	default:
		roots := []string{
			firstNonEmpty(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config")),
			firstNonEmpty(os.Getenv("XDG_CACHE_HOME"), filepath.Join(home, ".cache")),
			firstNonEmpty(os.Getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share")),
			firstNonEmpty(os.Getenv("XDG_STATE_HOME"), filepath.Join(home, ".local", "state")),
		}
		return roots
	}
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

func (v virtualPathValidator) validate(cwd, path string) (string, error) {
	normalized, err := normalizeExistingOrParent(path, cwd)
	if err != nil {
		return "", err
	}
	for _, root := range v.roots {
		if pathWithin(root, normalized) {
			return normalized, nil
		}
	}
	return "", fmt.Errorf("%w: %s", errVirtualPathDenied, normalized)
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
		if runtime.GOOS == "windows" {
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
	if runtime.GOOS != "windows" || filepath.Ext(name) != "" {
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
	if runtime.GOOS == "windows" {
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
