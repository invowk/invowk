// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"archive/tar"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	urootTar "github.com/u-root/u-root/pkg/core/tar"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

type (
	//goplint:ignore -- raw CLI archive path and operand strings.
	tarCreateOptions struct {
		archivePath string
		noRecursion bool
		verbose     bool
		operands    []string
	}

	// tarCommand wraps the u-root tar implementation.
	tarCommand struct {
		baseWrapper
	}
)

// newTarCommand creates a new tar command wrapper.
func newTarCommand() *tarCommand {
	return &tarCommand{
		name: "tar",
		flags: []FlagInfo{
			{Name: "create", ShortName: "c", Description: "create a new archive"},
			{Name: "extract", ShortName: "x", Description: "extract files from an archive"},
			{Name: "list", ShortName: "t", Description: "list the contents of an archive"},
			{Name: "file", ShortName: "f", Description: "use archive file", TakesValue: true},
			{Name: "verbose", ShortName: "v", Description: "verbosely list files processed"},
			{Name: "no-recursion", Description: "do not recurse into directories"},
		},
	}
}

// Run executes the tar command.
//
// Create (-c) archives are written directly with archive/tar because u-root
// v0.16.0's tarutil.CreateTar fails with absolute operands when ChangeDirectory
// is empty, leaks opened file handles, and logs -v to os.Stdout.
//
//goplint:ignore -- Command interface uses raw argv strings.
func (c *tarCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 1 {
		if opts, ok := parseTarCreateOptions(args[1:]); ok {
			if err := createTarArchive(ctx, GetHandlerContext(ctx), opts); err != nil {
				return wrapError(c.name, err)
			}
			return nil
		}
	}
	return c.runUpstream(ctx, urootTar.New(), args)
}

//goplint:ignore -- raw CLI archive path and operand strings.
func parseTarCreateOptions(raw []string) (tarCreateOptions, bool) {
	var tail []string
	if idx := leadingDashDash(raw); idx >= 0 {
		raw, tail = raw[:idx], raw[idx+1:]
	}
	fsFlags := flag.NewFlagSet("tar", flag.ContinueOnError)
	fsFlags.SetOutput(io.Discard)
	var create, extract, list, noRecursion, verbose bool
	var file string
	fsFlags.BoolVar(&create, "create", false, "")
	fsFlags.BoolVar(&create, "c", false, "")
	fsFlags.BoolVar(&extract, "extract", false, "")
	fsFlags.BoolVar(&extract, "x", false, "")
	fsFlags.BoolVar(&list, "list", false, "")
	fsFlags.BoolVar(&list, "t", false, "")
	fsFlags.BoolVar(&noRecursion, "no-recursion", false, "")
	fsFlags.BoolVar(&verbose, "verbose", false, "")
	fsFlags.BoolVar(&verbose, "v", false, "")
	fsFlags.StringVar(&file, "file", "", "")
	fsFlags.StringVar(&file, "f", "", "")
	if err := fsFlags.Parse(unixflag.ArgsToGoArgs(raw)); err != nil {
		return tarCreateOptions{}, false
	}
	ops := make([]string, 0, len(fsFlags.Args())+len(tail))
	ops = append(ops, fsFlags.Args()...)
	ops = append(ops, tail...)
	if !create || extract || list || file == "" || len(ops) == 0 {
		return tarCreateOptions{}, false
	}
	return tarCreateOptions{
		archivePath: file,
		noRecursion: noRecursion,
		verbose:     verbose,
		operands:    ops,
	}, true
}

func createTarArchive(ctx context.Context, hc *HandlerContext, opts tarCreateOptions) (retErr error) {
	baseDir := resolveTarBaseDir(hc.Dir)
	archivePath := hc.joinDir(opts.archivePath)
	outFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive %s: %w", archivePath, err)
	}
	outInfo, _ := outFile.Stat()
	tw := tar.NewWriter(outFile)
	defer func() {
		retErr = closeTarArchive(tw, outFile, archivePath, retErr)
	}()

	for _, op := range opts.operands {
		if err := tarCtxErr(ctx); err != nil {
			return err
		}
		absOp := resolveTarOperandAbsPath(baseDir, op)
		if err := addTarOperand(ctx, tw, baseDir, absOp, opts, outInfo, hc.Stdout); err != nil {
			return err
		}
	}
	return nil
}

//goplint:ignore -- raw archive path string.
func closeTarArchive(tw *tar.Writer, outFile *os.File, archivePath string, retErr error) error {
	if closeErr := tw.Close(); closeErr != nil && retErr == nil {
		retErr = fmt.Errorf("close tar writer %s: %w", archivePath, closeErr)
	}
	if closeErr := outFile.Close(); closeErr != nil && retErr == nil {
		retErr = fmt.Errorf("close archive %s: %w", archivePath, closeErr)
	}
	return retErr
}

func tarCtxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("tar create canceled: %w", err)
	}
	return nil
}

//goplint:ignore -- raw OS operand and directory paths.
func resolveTarOperandAbsPath(baseDir, op string) string {
	if !filepath.IsAbs(op) {
		if baseDir != "" {
			return filepath.Join(baseDir, op)
		}
		return op
	}
	dir := resolveTarBaseDir(filepath.Dir(op))
	return filepath.Join(dir, filepath.Base(op))
}

//goplint:ignore -- raw OS directory path string.
func resolveTarBaseDir(dir string) string {
	if dir == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

//goplint:ignore -- raw OS directory and operand path strings.
func addTarOperand(
	ctx context.Context,
	tw *tar.Writer,
	baseDir, absOp string,
	opts tarCreateOptions,
	outInfo fs.FileInfo,
	stdout io.Writer,
) error {
	if opts.noRecursion {
		info, err := os.Lstat(absOp)
		if err != nil {
			return fmt.Errorf("lstat %s: %w", absOp, err)
		}
		return writeTarEntry(tw, absOp, tarMemberName(baseDir, absOp), info, outInfo, opts.verbose, stdout)
	}
	if err := walkTarTree(ctx, tw, baseDir, absOp, opts, outInfo, stdout); err != nil {
		return fmt.Errorf("walk %s: %w", absOp, err)
	}
	return nil
}

//goplint:ignore -- raw OS directory and operand path strings.
func walkTarTree(
	ctx context.Context,
	tw *tar.Writer,
	baseDir, absOp string,
	opts tarCreateOptions,
	outInfo fs.FileInfo,
	stdout io.Writer,
) error {
	return filepath.WalkDir(absOp, func(path string, _ fs.DirEntry, walkErr error) error { //nolint:wrapcheck // caller adds operand context.
		if walkErr != nil {
			return walkErr
		}
		if err := tarCtxErr(ctx); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("lstat %s: %w", path, err)
		}
		name := tarMemberName(baseDir, path)
		if name == "." {
			return nil
		}
		return writeTarEntry(tw, path, name, info, outInfo, opts.verbose, stdout)
	})
}

//goplint:ignore -- raw OS paths and tar member name string.
func tarMemberName(baseDir, absPath string) string {
	cleanPath := filepath.Clean(absPath)
	if baseDir != "" {
		if rel, err := filepath.Rel(baseDir, cleanPath); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	vol := filepath.VolumeName(cleanPath)
	trimmed := strings.TrimLeft(filepath.ToSlash(cleanPath[len(vol):]), "/")
	if trimmed == "" {
		return "."
	}
	return trimmed
}

//goplint:ignore -- raw OS path and tar member name strings.
func writeTarEntry(tw *tar.Writer, path, name string, info, outInfo fs.FileInfo, verbose bool, stdout io.Writer) error {
	if outInfo != nil && os.SameFile(info, outInfo) {
		return nil
	}
	linkTarget := ""
	if info.Mode()&os.ModeSymlink != 0 {
		var err error
		linkTarget, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("read link %s: %w", path, err)
		}
	} else if !info.Mode().IsRegular() && !info.IsDir() {
		// Reject junctions (ModeIrregular) and non-regular special entries
		// without following or reading targets.
		return fmt.Errorf("unsupported file type for %s", path)
	}
	hdr, err := tar.FileInfoHeader(info, linkTarget)
	if err != nil {
		return fmt.Errorf("build tar header for %s: %w", path, err)
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %s: %w", name, err)
	}
	if verbose && stdout != nil {
		fmt.Fprintln(stdout, name)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return copyTarRegularFile(tw, path)
}

//goplint:ignore -- raw OS path string.
func copyTarRegularFile(tw *tar.Writer, path string) (retErr error) {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("close %s: %w", path, closeErr)
		}
	}()
	if _, err = io.Copy(tw, f); err != nil {
		return fmt.Errorf("copy %s: %w", path, err)
	}
	return nil
}
