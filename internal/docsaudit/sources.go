// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SourceCatalog captures discovered documentation sources and their files.
type SourceCatalog struct {
	Sources      []DocumentationSource
	Files        []string
	FileToSource map[string]DocumentationSource
}

// DiscoverSources finds documentation sources and their files.
func DiscoverSources(ctx context.Context, opts Options) (*SourceCatalog, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	sources, err := resolveDocSources(opts)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, 128)
	fileToSource := make(map[string]DocumentationSource)
	for _, source := range sources {
		sourceFiles, err := listSourceFiles(opts.RepoRoot, source)
		if err != nil {
			return nil, err
		}
		for _, file := range sourceFiles {
			if _, exists := fileToSource[file]; exists {
				continue
			}
			fileToSource[file] = source
			files = append(files, file)
		}
	}

	sort.Strings(files)

	return &SourceCatalog{
		Sources:      sources,
		Files:        files,
		FileToSource: fileToSource,
	}, nil
}

func resolveDocSources(opts Options) ([]DocumentationSource, error) {
	if len(opts.DocsSources) > 0 {
		return resolveExplicitSources(opts.RepoRoot, opts.DocsSources)
	}

	return resolveDefaultSources(opts)
}

func resolveExplicitSources(repoRoot string, paths []string) ([]DocumentationSource, error) {
	var sources []DocumentationSource
	for _, raw := range paths {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		location := raw
		if !filepath.IsAbs(location) {
			location = filepath.Join(repoRoot, raw)
		}

		info, err := os.Stat(location)
		if err != nil {
			return nil, fmt.Errorf("stat doc source %s: %w", location, err)
		}

		kind := DocKindGuide
		if info.IsDir() {
			kind = DocKindGuide
		} else if isReadmeFile(info.Name()) {
			kind = DocKindReadme
		}

		sources = append(sources, DocumentationSource{
			ID:       sourceIDFromPath(repoRoot, location),
			Kind:     kind,
			Location: location,
		})
	}

	return sources, nil
}

func resolveDefaultSources(opts Options) ([]DocumentationSource, error) {
	var sources []DocumentationSource
	repoRoot := opts.RepoRoot

	readmePath, err := findReadme(repoRoot)
	if err != nil {
		return nil, err
	}
	if readmePath != "" {
		sources = append(sources, DocumentationSource{
			ID:       sourceIDFromPath(repoRoot, readmePath),
			Kind:     DocKindReadme,
			Location: readmePath,
		})
	}

	if websiteDocs := filepath.Join(repoRoot, "website", "docs"); dirExists(websiteDocs) {
		sources = append(sources, DocumentationSource{
			ID:       sourceIDFromPath(repoRoot, websiteDocs),
			Kind:     DocKindWebsite,
			Location: websiteDocs,
		})
	}

	for _, dir := range []string{"docs", "guides"} {
		path := filepath.Join(repoRoot, dir)
		if dirExists(path) {
			sources = append(sources, DocumentationSource{
				ID:       sourceIDFromPath(repoRoot, path),
				Kind:     DocKindGuide,
				Location: path,
			})
		}
	}

	appendSampleSource := func(path string) {
		if path == "" || !dirExists(path) {
			return
		}
		sources = append(sources, DocumentationSource{
			ID:       sourceIDFromPath(repoRoot, path),
			Kind:     DocKindSample,
			Location: path,
		})
	}

	appendSampleSource(opts.CanonicalExamplesPath)
	appendSampleSource(filepath.Join(repoRoot, "examples"))
	appendSampleSource(filepath.Join(repoRoot, "modules"))

	return dedupeSources(sources), nil
}

func findReadme(repoRoot string) (string, error) {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return "", fmt.Errorf("read repo root: %w", err)
	}

	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if isReadmeFile(name) {
			candidates = append(candidates, filepath.Join(repoRoot, name))
		}
	}

	if len(candidates) == 0 {
		return "", nil
	}

	sort.Strings(candidates)
	for _, candidate := range candidates {
		if strings.EqualFold(filepath.Base(candidate), "README.md") {
			return candidate, nil
		}
	}

	return candidates[0], nil
}

func isReadmeFile(name string) bool {
	base := strings.TrimSpace(name)
	if base == "" {
		return false
	}

	if strings.EqualFold(base, "README") {
		return true
	}

	if strings.HasPrefix(strings.ToUpper(base), "README.") {
		return true
	}

	return false
}

func listSourceFiles(repoRoot string, source DocumentationSource) ([]string, error) {
	if source.Location == "" {
		return nil, nil
	}

	info, err := os.Stat(source.Location)
	if err != nil {
		return nil, fmt.Errorf("stat source %s: %w", source.Location, err)
	}
	if !info.IsDir() {
		return []string{source.Location}, nil
	}

	exts := extensionsForKind(source.Kind)
	files, err := ListFiles(repoRoot, []string{source.Location}, exts)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func extensionsForKind(kind DocumentationKind) []string {
	switch kind {
	case DocKindSample:
		return []string{".md", ".mdx", ".cue", ".txt"}
	case DocKindWebsite, DocKindGuide:
		return []string{".md", ".mdx"}
	default:
		return []string{".md", ".mdx"}
	}
}

func sourceIDFromPath(repoRoot, location string) string {
	if repoRoot == "" || location == "" {
		return location
	}

	rel, err := filepath.Rel(repoRoot, location)
	if err != nil || strings.HasPrefix(rel, "..") {
		return location
	}

	return filepath.ToSlash(rel)
}

func dedupeSources(sources []DocumentationSource) []DocumentationSource {
	seen := make(map[string]struct{})
	unique := make([]DocumentationSource, 0, len(sources))
	for _, source := range sources {
		key := string(source.Kind) + "|" + source.Location
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, source)
	}
	return unique
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
