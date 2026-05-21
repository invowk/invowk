// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const moduleCopyHashLength = 12

type (
	provisionedModuleDestinationPath string

	//goplint:validate-all
	//
	// provisionedModuleCopy records how one discovered host module is copied
	// into the provisioned layer while preserving its original command namespace.
	provisionedModuleCopy struct {
		SourcePath       types.FilesystemPath
		DestinationPath  provisionedModuleDestinationPath
		CommandNamespace invowkmod.ModuleNamespace
	}
)

func (p provisionedModuleDestinationPath) Validate() error {
	value := string(p)
	if value == "" || pathpkg.IsAbs(value) || strings.ContainsRune(value, '\x00') || pathpkg.Clean(value) != value {
		return fmt.Errorf("invalid provisioned module destination path %q", value)
	}
	if _, err := invowkmod.ParseModuleName(pathpkg.Base(value)); err != nil {
		return fmt.Errorf("provisioned module destination path: %w", err)
	}
	return nil
}

func (p provisionedModuleDestinationPath) String() string { return string(p) }

func (c provisionedModuleCopy) Validate() error {
	return errors.Join(
		c.SourcePath.Validate(),
		c.DestinationPath.Validate(),
		c.CommandNamespace.Validate(),
	)
}

func discoverProvisionedModuleCopies(paths []types.FilesystemPath, entries ModuleEntries) []provisionedModuleCopy {
	copies := make([]provisionedModuleCopy, 0)
	seen := make(map[string]struct{})

	appendModuleCopy := func(modulePath string, namespace invowkmod.ModuleNamespace) {
		sourcePath := types.FilesystemPath(modulePath) //goplint:ignore -- discovered from filesystem walk and validated by DiscoverModules.
		if namespace == "" {
			namespace = commandNamespaceFromModulePath(sourcePath)
		}
		moduleCopy := provisionedModuleCopy{
			SourcePath:       sourcePath,
			DestinationPath:  destinationPathForModule(sourcePath, namespace),
			CommandNamespace: namespace,
		}
		key := string(moduleCopy.SourcePath) + "\x00" + string(moduleCopy.CommandNamespace)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		copies = append(copies, moduleCopy)
	}

	for _, modulePath := range DiscoverModules(paths) {
		appendModuleCopy(modulePath, "")
	}
	for _, entry := range entries {
		if entry.Path == "" {
			continue
		}
		for _, modulePath := range DiscoverModules([]types.FilesystemPath{entry.Path}) {
			appendModuleCopy(modulePath, entry.CommandNamespace)
		}
	}

	slices.SortFunc(copies, func(a, b provisionedModuleCopy) int {
		if cmp := strings.Compare(string(a.CommandNamespace), string(b.CommandNamespace)); cmp != 0 {
			return cmp
		}
		return strings.Compare(string(a.SourcePath), string(b.SourcePath))
	})
	return copies
}

func commandNamespaceFromModulePath(modulePath types.FilesystemPath) invowkmod.ModuleNamespace {
	return invowkmod.ModuleNamespace(invowkmod.CommandSourceIDFromModulePath(modulePath))
}

func destinationPathForModule(modulePath types.FilesystemPath, namespace invowkmod.ModuleNamespace) provisionedModuleDestinationPath {
	baseName := filepath.Base(string(modulePath))
	sum := sha256.Sum256([]byte(string(modulePath) + "\x00" + string(namespace)))
	shortHash := hex.EncodeToString(sum[:])[:moduleCopyHashLength]
	destinationPath := provisionedModuleDestinationPath(pathpkg.Join("p"+shortHash, baseName))
	if err := destinationPath.Validate(); err != nil {
		return ""
	}
	return destinationPath
}
