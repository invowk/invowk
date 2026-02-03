// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"io"
	"os"
	"path/filepath"
)

// FileProcessor processes a single reader with file context.
// Parameters:
//   - r: the input stream to process
//   - filename: the original filename argument (or "-" for stdin)
//   - index: 0-based index of current file (0 for stdin)
//   - total: total number of files being processed (0 for stdin)
type FileProcessor func(r io.Reader, filename string, index, total int) error

// ProcessFilesOrStdin processes files from args or stdin if no files given.
// For each file, it resolves relative paths using workDir, opens the file,
// and invokes the processor. For stdin, the processor receives filename "-"
// with index=0 and total=0.
//
// Uses named return to aggregate close errors per project rules. If the processor
// succeeds but close fails, the close error is returned.
//
// Example usage (head command):
//
//	return ProcessFilesOrStdin(fs.Args(), hc.Stdin, hc.Dir, c.name,
//	    func(r io.Reader, filename string, index, total int) error {
//	        // Print header for multiple files
//	        if total > 1 {
//	            if index > 0 {
//	                fmt.Fprintln(hc.Stdout)
//	            }
//	            fmt.Fprintf(hc.Stdout, "==> %s <==\n", filename)
//	        }
//	        return c.processReader(hc.Stdout, r, numLines)
//	    })
func ProcessFilesOrStdin(
	args []string,
	stdin io.Reader,
	workDir string,
	cmdName string,
	processor FileProcessor,
) (err error) {
	if len(args) == 0 {
		return processor(stdin, "-", 0, 0)
	}

	total := len(args)
	for i, file := range args {
		if err := processFile(file, workDir, cmdName, func(f *os.File) error {
			return processor(f, file, i, total)
		}); err != nil {
			return err
		}
	}

	return nil
}

// processFile opens a file and calls the processor, handling path resolution
// and close error aggregation via named return.
func processFile(file, workDir, cmdName string, processor func(f *os.File) error) (err error) {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}

	f, err := os.Open(path)
	if err != nil {
		return wrapError(cmdName, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = wrapError(cmdName, closeErr)
		}
	}()

	if processErr := processor(f); processErr != nil {
		return processErr
	}

	return nil
}
