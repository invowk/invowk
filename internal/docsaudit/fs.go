// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var defaultSkipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"coverage":     {},
	".idea":        {},
	".vscode":      {},
}

func findRepoRoot(start string) (string, error) {
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		start = cwd
	}

	absStart, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}

	info, err := os.Stat(absStart)
	if err != nil {
		return "", fmt.Errorf("stat start path: %w", err)
	}
	if !info.IsDir() {
		absStart = filepath.Dir(absStart)
	}

	cur := absStart
	for {
		gitPath := filepath.Join(cur, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return cur, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", gitPath, err)
		}

		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	return "", fmt.Errorf("repository root not found from %s", absStart)
}

func listFiles(root string, dirs, exts []string) ([]string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	if len(dirs) == 0 {
		dirs = []string{rootAbs}
	}

	extSet := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extSet[ext] = struct{}{}
	}

	var files []string
	for _, dir := range dirs {
		walkRoot := dir
		if !filepath.IsAbs(dir) {
			walkRoot = filepath.Join(rootAbs, dir)
		}

		err = filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if _, ok := defaultSkipDirs[d.Name()]; ok {
					return filepath.SkipDir
				}
				return nil
			}

			if len(extSet) == 0 {
				files = append(files, path)
				return nil
			}

			ext := strings.ToLower(filepath.Ext(d.Name()))
			if _, ok := extSet[ext]; ok {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", walkRoot, err)
		}
	}

	return files, nil
}

func readFileLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return strings.Split(string(data), "\n"), nil
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}

	return nil
}
