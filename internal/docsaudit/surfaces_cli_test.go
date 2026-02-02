// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestDiscoverCLISurfaces(t *testing.T) {
	root := &cobra.Command{Use: "invowk"}
	root.PersistentFlags().Bool("verbose", false, "verbose output")

	docsCmd := &cobra.Command{Use: "docs"}
	auditCmd := &cobra.Command{Use: "audit"}
	auditCmd.Flags().StringP("out", "o", "", "output path")
	docsCmd.AddCommand(auditCmd)
	root.AddCommand(docsCmd)

	surfaces, err := DiscoverCLISurfaces(root)
	if err != nil {
		t.Fatalf("DiscoverCLISurfaces: %v", err)
	}

	nameSet := make(map[string]struct{})
	for _, surface := range surfaces {
		nameSet[surface.Name] = struct{}{}
	}

	for _, expected := range []string{"invowk", "invowk docs", "invowk docs audit", "invowk --verbose", "invowk docs audit --out"} {
		if _, ok := nameSet[expected]; !ok {
			t.Fatalf("missing surface %q", expected)
		}
	}
}
