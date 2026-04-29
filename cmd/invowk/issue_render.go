// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"github.com/charmbracelet/glamour"
	"github.com/invowk/invowk/internal/issue"
)

//goplint:ignore -- CLI rendering helper passes through Glamour style names and rendered text.
func renderIssueCatalogEntry(catalogEntry *issue.Issue, stylePath string) (string, error) {
	if catalogEntry == nil {
		return "", nil
	}
	return catalogEntry.RenderWith(glamour.Render, stylePath)
}
