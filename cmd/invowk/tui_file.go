// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
)

var (
	fileTitle         string
	fileDescription   string
	fileDirectoryOnly bool
	fileShowFiles     bool
	fileHidden        bool
	fileHeight        int
	fileAllowedExts   []string
)

// tuiFileCmd provides a file picker.
var tuiFileCmd = &cobra.Command{
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

func init() {
	tuiCmd.AddCommand(tuiFileCmd)

	tuiFileCmd.Flags().StringVar(&fileTitle, "title", "", "title displayed above the file picker")
	tuiFileCmd.Flags().StringVar(&fileDescription, "description", "", "description displayed below the title")
	tuiFileCmd.Flags().BoolVar(&fileDirectoryOnly, "directory", false, "only show directories")
	tuiFileCmd.Flags().BoolVar(&fileShowFiles, "file", true, "show files (default true)")
	tuiFileCmd.Flags().BoolVar(&fileHidden, "hidden", false, "show hidden files")
	tuiFileCmd.Flags().IntVar(&fileHeight, "height", 0, "height of the picker (0 for auto)")
	tuiFileCmd.Flags().StringSliceVar(&fileAllowedExts, "allowed", nil, "allowed file extensions (e.g., .go,.md)")
}

func runTuiFile(cmd *cobra.Command, args []string) error {
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
			Height:            fileHeight,
			AllowedExtensions: fileAllowedExts,
		})
	}

	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, result)
	return nil
}
