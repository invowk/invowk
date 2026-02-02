// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"invowk-cli/pkg/invkmod"
)

// DiscoverModuleSurfaces inventories module definition surfaces.
func DiscoverModuleSurfaces(opts Options) ([]UserFacingSurface, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	modulesRoot := filepath.Join(opts.RepoRoot, "modules")
	if !dirExists(modulesRoot) {
		return nil, nil
	}

	var surfaces []UserFacingSurface
	err := filepath.WalkDir(modulesRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if path == modulesRoot {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".invkmod") {
			return nil
		}

		metadata, err := invkmod.ParseModuleMetadataOnly(path)
		if err != nil {
			return fmt.Errorf("parse module metadata %s: %w", path, err)
		}

		moduleID := metadata.Module
		if moduleID == "" {
			moduleID = strings.TrimSuffix(d.Name(), ".invkmod")
		}

		surfaces = append(surfaces, UserFacingSurface{
			ID:             fmt.Sprintf("module:%s", moduleID),
			Type:           SurfaceTypeModule,
			Name:           moduleID,
			SourceLocation: path,
		})

		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("walk modules: %w", err)
	}

	sort.Slice(surfaces, func(i, j int) bool {
		if surfaces[i].Name != surfaces[j].Name {
			return surfaces[i].Name < surfaces[j].Name
		}
		return surfaces[i].ID < surfaces[j].ID
	})

	return surfaces, nil
}
