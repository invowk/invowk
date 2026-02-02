// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// DiscoverCLISurfaces inventories CLI command and flag surfaces.
func DiscoverCLISurfaces(root *cobra.Command) ([]UserFacingSurface, error) {
	if root == nil {
		return nil, errors.New("root command is nil")
	}

	var surfaces []UserFacingSurface
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd == nil || cmd.Hidden {
			return
		}

		cmdPath := cmd.CommandPath()
		if cmdPath == "" {
			cmdPath = cmd.Use
		}

		surfaces = append(surfaces, UserFacingSurface{
			ID:             fmt.Sprintf("cli:command:%s", cmdPath),
			Type:           SurfaceTypeCommand,
			Name:           cmdPath,
			SourceLocation: "cli",
		})

		for _, flag := range collectDefinedFlags(cmd) {
			flagName := fmt.Sprintf("%s --%s", cmdPath, flag.Name)
			surfaces = append(surfaces, UserFacingSurface{
				ID:             fmt.Sprintf("cli:flag:%s:%s", cmdPath, flag.Name),
				Type:           SurfaceTypeFlag,
				Name:           flagName,
				SourceLocation: "cli",
			})
		}

		children := cmd.Commands()
		sort.Slice(children, func(i, j int) bool {
			return children[i].CommandPath() < children[j].CommandPath()
		})
		for _, child := range children {
			walk(child)
		}
	}

	walk(root)
	return surfaces, nil
}

func collectDefinedFlags(cmd *cobra.Command) []*pflag.Flag {
	seen := make(map[string]struct{})
	flags := make([]*pflag.Flag, 0, 8)
	addFlag := func(flag *pflag.Flag) {
		if flag == nil || flag.Hidden {
			return
		}
		if _, ok := seen[flag.Name]; ok {
			return
		}
		seen[flag.Name] = struct{}{}
		flags = append(flags, flag)
	}

	cmd.LocalFlags().VisitAll(addFlag)
	cmd.PersistentFlags().VisitAll(addFlag)

	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Name < flags[j].Name
	})

	return flags
}
