// SPDX-License-Identifier: MPL-2.0

package docsaudit

import "testing"

func TestDiscoverConfigSurfaces(t *testing.T) {
	surfaces, err := DiscoverConfigSurfaces()
	if err != nil {
		t.Fatalf("DiscoverConfigSurfaces: %v", err)
	}

	nameSet := make(map[string]struct{})
	for _, surface := range surfaces {
		nameSet[surface.Name] = struct{}{}
	}

	for _, expected := range []string{
		"default_runtime",
		"search_paths",
		"virtual_shell.enable_uroot_utils",
		"container.auto_provision.enabled",
		"ui.color_scheme",
	} {
		if _, ok := nameSet[expected]; !ok {
			t.Fatalf("missing config surface %q", expected)
		}
	}
}
