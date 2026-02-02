// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"invowk-cli/internal/config"
)

// DiscoverConfigSurfaces inventories configuration field surfaces.
func DiscoverConfigSurfaces() ([]UserFacingSurface, error) {
	paths := collectConfigPaths(reflect.TypeFor[config.Config](), "")
	if len(paths) == 0 {
		return nil, fmt.Errorf("no config fields discovered")
	}

	sort.Strings(paths)
	surfaces := make([]UserFacingSurface, 0, len(paths))
	for _, path := range paths {
		surfaces = append(surfaces, UserFacingSurface{
			ID:             fmt.Sprintf("config:%s", path),
			Type:           SurfaceTypeConfigField,
			Name:           path,
			SourceLocation: "config",
		})
	}

	return surfaces, nil
}

func collectConfigPaths(t reflect.Type, prefix string) []string {
	if t == nil {
		return nil
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		if prefix == "" {
			return nil
		}
		return []string{prefix}
	}

	var paths []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}

		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct {
			paths = append(paths, collectConfigPaths(fieldType, path)...)
			continue
		}

		paths = append(paths, path)
	}

	return paths
}
