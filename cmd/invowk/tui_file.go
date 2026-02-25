// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIFileCommand creates the `invowk tui file` command.
func newTUIFileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file [path]",
		Short: "File picker",
		Long: `Browse and select files or directories.

If a path is provided, the picker starts in that directory.
Otherwise, it starts in the current directory.

Examples:
  # Pick any file from current directory
  invowk tui file

  # Start in specific directory
  invowk tui file /home/user/documents

  # Only show directories
  invowk tui file --directory

  # Show hidden files
  invowk tui file --hidden

  # Filter by extension
  invowk tui file --allowed ".go,.md,.txt"

  # Use in shell script
  FILE=$(invowk tui file --allowed ".json,.yaml")
  echo "Selected: $FILE"`,
		Args: cobra.MaximumNArgs(1),
		RunE: runTuiFile,
	}

	cmd.Flags().String("title", "", "title displayed above the file picker")
	cmd.Flags().String("description", "", "description displayed below the title")
	cmd.Flags().Bool("directory", false, "only show directories")
	cmd.Flags().Bool("file", true, "show files (default true)")
	cmd.Flags().Bool("hidden", false, "show hidden files")
	cmd.Flags().Int("height", 0, "height of the picker (0 for auto)")
	cmd.Flags().StringSlice("allowed", nil, "allowed file extensions (e.g., .go,.md)")

	return cmd
}

func runTuiFile(cmd *cobra.Command, args []string) error {
	fileTitle, _ := cmd.Flags().GetString("title")
	fileDescription, _ := cmd.Flags().GetString("description")
	fileDirectoryOnly, _ := cmd.Flags().GetBool("directory")
	fileShowFiles, _ := cmd.Flags().GetBool("file")
	fileHidden, _ := cmd.Flags().GetBool("hidden")
	fileHeight, _ := cmd.Flags().GetInt("height")
	fileAllowedExts, _ := cmd.Flags().GetStringSlice("allowed")

	startPath := "."
	if len(args) > 0 {
		startPath = args[0]
	}

	// Determine file/dir allowed settings
	allowFiles := !fileDirectoryOnly && fileShowFiles
	allowDirs := fileDirectoryOnly || !fileShowFiles

	var result string
	var err error

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		result, err = client.File(tuiserver.FileRequest{
			Title:       fileTitle,
			Description: fileDescription,
			Path:        startPath,
			ShowFiles:   allowFiles,
			ShowDirs:    allowDirs,
			ShowHidden:  fileHidden,
			Height:      fileHeight,
			AllowedExts: fileAllowedExts,
		})
	} else {
		// Render TUI directly
		result, err = tui.File(tui.FileOptions{
			Title:             fileTitle,
			Description:       fileDescription,
			CurrentDirectory:  startPath,
			FileAllowed:       allowFiles,
			DirAllowed:        allowDirs,
			ShowHidden:        fileHidden,
			Height:            tui.TerminalDimension(fileHeight),
			AllowedExtensions: fileAllowedExts,
		})
	}

	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout, result) // Terminal output; error non-critical
	return nil
}
