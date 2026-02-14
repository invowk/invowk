// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIPagerCommand creates the `invowk tui pager` command.
func newTUIPagerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pager [file]",
		Short: "Scroll through content",
		Long: `Display content in a scrollable pager.

Content can be provided via:
- A file path as an argument
- Piped via stdin

Examples:
  # View a file
  invowk tui pager README.md

  # Pipe content
  cat long-file.txt | invowk tui pager

  # With line numbers
  invowk tui pager --line-numbers myfile.go

  # Soft wrap long lines
  invowk tui pager --soft-wrap document.txt

  # Use with command output
  git log | invowk tui pager --title "Git History"`,
		Args: cobra.MaximumNArgs(1),
		RunE: runTuiPager,
	}

	cmd.Flags().String("title", "", "title displayed above the pager")
	cmd.Flags().Bool("line-numbers", false, "show line numbers")
	cmd.Flags().Bool("soft-wrap", false, "soft wrap long lines")

	return cmd
}

func runTuiPager(cmd *cobra.Command, args []string) error {
	pagerTitle, _ := cmd.Flags().GetString("title")
	pagerLineNum, _ := cmd.Flags().GetBool("line-numbers")
	pagerSoftWrap, _ := cmd.Flags().GetBool("soft-wrap")

	var content string

	if len(args) > 0 {
		// Read from file
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		content = string(data)
	} else {
		// Check if we have stdin input
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Read from stdin
			var sb strings.Builder
			reader := bufio.NewReader(os.Stdin)
			for {
				line, err := reader.ReadString('\n')
				sb.WriteString(line)
				if err != nil {
					if err == io.EOF {
						break
					}
					return fmt.Errorf("error reading stdin: %w", err)
				}
			}
			content = sb.String()
		} else {
			return fmt.Errorf("no content provided; provide a file path or pipe content via stdin")
		}
	}

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		return client.Pager(tuiserver.PagerRequest{
			Content:     content,
			ShowLineNum: pagerLineNum,
			SoftWrap:    pagerSoftWrap,
		})
	}

	// Render TUI directly
	return tui.Pager(tui.PagerOptions{
		Title:           pagerTitle,
		Content:         content,
		ShowLineNumbers: pagerLineNum,
		SoftWrap:        pagerSoftWrap,
	})
}
