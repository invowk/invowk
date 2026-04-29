// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/invowk/invowk/internal/issue"
)

type (
	//goplint:constant-only
	//
	// issueMarkdown is a CLI rendering DTO for catalog markdown.
	issueMarkdown string

	//goplint:constant-only
	//
	// formattedDisplayError is a CLI rendering DTO for actionable errors.
	formattedDisplayError string
)

func (m issueMarkdown) String() string { return string(m) }

func (m issueMarkdown) Validate() error { return nil }

func (e formattedDisplayError) String() string { return string(e) }

func (e formattedDisplayError) Validate() error { return nil }

//goplint:ignore -- CLI rendering helper passes through Glamour style names and rendered text.
func renderIssueCatalogEntry(catalogEntry *issue.Issue, stylePath string) (string, error) {
	if catalogEntry == nil {
		return "", nil
	}
	return glamour.Render(issueCatalogMarkdown(catalogEntry).String(), stylePath)
}

func issueCatalogMarkdown(catalogEntry *issue.Issue) issueMarkdown {
	var md strings.Builder
	md.WriteString(catalogEntry.MarkdownMsg().String())

	docLinks := catalogEntry.DocLinks()
	extLinks := catalogEntry.ExtLinks()
	if len(docLinks) > 0 || len(extLinks) > 0 {
		md.WriteString("\n\n")
		md.WriteString("## See also:\n")
		for _, link := range docLinks {
			md.WriteString("- [" + link.String() + "]\n")
		}
		for _, link := range extLinks {
			md.WriteString("- [" + link.String() + "]\n")
		}
	}

	return issueMarkdown(md.String()) //goplint:ignore -- markdown is assembled from validated issue catalog data.
}

func formatActionableError(err *issue.ActionableError, verbose bool) formattedDisplayError {
	var msg strings.Builder

	msg.WriteString(err.Error())

	for _, suggestion := range err.Suggestions() {
		msg.WriteString("\n\n  • ")
		msg.WriteString(suggestion)
	}

	if verbose && err.Cause() != nil {
		msg.WriteString("\n\nError chain:")
		cause := err.Cause()
		depth := 1
		for cause != nil {
			fmt.Fprintf(&msg, "\n  %d. %s", depth, cause.Error())
			cause = errors.Unwrap(cause)
			depth++
		}
	}

	return formattedDisplayError(msg.String()) //goplint:ignore -- display text is assembled from structured actionable error data.
}
