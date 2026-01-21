// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	pagerTitle    string
	pagerLineNum  bool
	pagerSoftWrap bool

	// tuiPagerCmd provides content scrolling.
	tuiPagerCmd = &cobra.Command{
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
)

func init() {
	tuiCmd.AddCommand(tuiPagerCmd)

	tuiPagerCmd.Flags().StringVar(&pagerTitle, "title", "", "title displayed above the pager")
	tuiPagerCmd.Flags().BoolVar(&pagerLineNum, "line-numbers", false, "show line numbers")
	tuiPagerCmd.Flags().BoolVar(&pagerSoftWrap, "soft-wrap", false, "soft wrap long lines")
}

func runTuiPager(cmd *cobra.Command, args []string) error {
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
