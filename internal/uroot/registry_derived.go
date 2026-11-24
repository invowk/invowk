// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/u-root/u-root/pkg/upath"
)

const (
	cpSourceMissingErrMsg             = "virtual path denied: cp source does not exist"
	cpIntoSourceErrMsg                = "virtual path denied: cp destination is inside a source tree"
	cpLinkWithOtherDestinationsErrMsg = "virtual path denied: cp -P would create a link next to other destinations"
	tarArchiveNotRegularErrMsg        = "virtual path denied: tar archive is not a regular file"

	// derivedPathFileWrite: the built-in creates or truncates a regular file
	// at the path, following a link there.
	derivedPathFileWrite derivedPathUse = 0
	// derivedPathDirCreate: the built-in creates a directory at the path and
	// any missing parents (MkdirAll), following links on the way.
	derivedPathDirCreate derivedPathUse = 1
	// derivedPathLinkCreate: cp -P creates a symbolic link at the path.
	derivedPathLinkCreate derivedPathUse = 2
	// derivedPathModeChange: tar -x changes the mode of the path, following a
	// link there.
	derivedPathModeChange derivedPathUse = 3
	// derivedPathSourceLink: cp without -P follows the source link at the
	// path and reads its target.
	derivedPathSourceLink derivedPathUse = 4
	// derivedPathSourceOperand: a cp source operand after the first, which
	// must exist when the command starts.
	derivedPathSourceOperand derivedPathUse = 5
)

var (
	// errCpSourceMissing refuses a cp source operand after the first that
	// does not exist when the command starts: an earlier source of the same
	// command could create it, so what it reads cannot be predicted.
	errCpSourceMissing = errors.New(cpSourceMissingErrMsg)
	// errCpIntoSource refuses a cp -r destination at or below a source
	// operand of the same command: entries written there could be walked and
	// copied again, at paths that were not checked.
	errCpIntoSource = errors.New(cpIntoSourceErrMsg)
	// errCpLinkWithOtherDestinations refuses a cp -P command that would
	// create a link and has any other destination: a later destination could
	// resolve through that link, which did not exist when it was checked.
	errCpLinkWithOtherDestinations = errors.New(cpLinkWithOtherDestinationsErrMsg)
	// errTarArchiveNotRegular refuses a tar -x archive that exists and is not
	// a regular file: it is read twice, and a second read of a named pipe or
	// a device would not give the entries that were checked.
	errTarArchiveNotRegular = errors.New(tarArchiveNotRegularErrMsg)
)

type (
	// derivedPathUse is what a built-in does with a path it derives from its
	// resolved operands.
	derivedPathUse int

	// derivedPath is one path a built-in derives from its resolved operands,
	// as the built-in itself builds it, with its use.
	//
	//goplint:ignore -- raw OS path that u-root builds from the resolved operands.
	derivedPath struct {
		path string
		use  derivedPathUse
		// sourceInfo is the cp source entry a destination copies, as u-root's
		// Copy stats it; nil when that stat fails or for other uses.
		sourceInfo fs.FileInfo
	}

	// derivedPathVisit receives each derived path in the built-in's order; an
	// error ends the enumeration and is returned unchanged.
	derivedPathVisit func(derivedPath) error

	// cpDerivedChecker is the restricted-access visitor of cp's derived
	// paths: the allowed-root check of every path and the prediction rules
	// that keep what cp derives at run time equal to what was checked.
	//
	//goplint:ignore -- raw OS paths of one cp check.
	cpDerivedChecker struct {
		hc        *HandlerContext
		recursive bool
		// sources holds the os.Lstat info of every source operand that exists.
		sources []fs.FileInfo
		// insideDirs caches, per resolved directory, whether it or one of its
		// existing ancestors is a source operand.
		insideDirs map[string]bool
		// firstLink is the resolved path of the first link the command would
		// create; destinations counts every destination checked so far.
		firstLink    string
		destinations int
	}
)

// checkUrootDerivedPaths checks, under restricted access, every path cp and
// tar -x derive from their resolved operands before the built-in runs: each
// path they will create, write, create directories at or change the mode of,
// and each source link cp follows, is resolved with hc.ResolvePath, the full
// resolution plus the allowed-root check. One refusal refuses the whole
// command. Like checkGzipOutputPaths, the checked paths are not handed on:
// the built-in derives them again from the same resolved operands. The
// prediction rules of cpDerivedChecker and the archive rule of
// tarDerivedPaths refuse the shapes whose paths the run itself could change.
//
// Under full access (hc.FullAccess) it checks nothing. The zero value checks,
// so a validating context that does not set the flag fails closed.
//
// The paths are checked before the built-in uses them. Only the script's own
// concurrent work (its background jobs, other stages of a pipeline, allowed
// host binaries they start) can change the tree in between; that window is
// accepted, as for every other built-in's operand check.
func checkUrootDerivedPaths(ctx context.Context, hc *HandlerContext, name string, resolved urootArgs) error {
	if hc == nil || hc.FullAccess {
		return nil
	}
	switch name {
	case "cp":
		return cpDerivedPaths(ctx, resolved, newCpDerivedChecker(hc, resolved).visit)
	case "tar":
		return tarDerivedPaths(ctx, resolved, func(p derivedPath) error {
			_, err := hc.ResolvePath(p.path)
			return err
		})
	default:
		return nil
	}
}

// cpDerivedPaths yields, in u-root cp's order (u-root v0.16.0,
// pkg/core/cp/cp.go RunContext, Copy and CopyTree), every path cp will
// create or write, and without -P every source link it follows. It first
// yields each source operand after the first (derivedPathSourceOperand).
// Then, for each source, the destination is the last operand or, when that
// is a directory, the directory joined with the source's base name. Without
// -r a source that is not a directory yields its destination as a file
// write. With -r a filepath.Walk of the source, which does not descend into
// links, yields the destination joined with every entry's relative path,
// with the use Copy gives the entry's type.
//
// It never ends a source earlier than u-root does: only a walk error ends
// one, as in CopyTree. An entry that Copy fails on (a stat error, a type it
// does not support) yields its destination as a file write and the walk goes
// on, so more is checked than u-root may touch, which fails closed. The
// pre-callback's skips are not evaluated: a skipped entry is yielded too.
func cpDerivedPaths(ctx context.Context, resolved urootArgs, visit derivedPathVisit) error {
	operands := resolved.operands
	if len(operands) < 2 {
		return nil
	}
	recursive := resolved.boolOption("r", "R", "recursive", "RECURSIVE")
	noFollow := resolved.boolOption("P", "no-dereference")
	sources, to := operands[:len(operands)-1], operands[len(operands)-1]
	toInfo, toExists := cpStat(to, false)
	toDir := toExists && toInfo.IsDir()
	if len(sources) > 1 && !toDir {
		return nil
	}
	for _, source := range sources[1:] {
		if err := visit(derivedPath{path: source, use: derivedPathSourceOperand}); err != nil {
			return err
		}
	}
	for _, source := range sources {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("cp: %w", ctxErr)
		}
		dst := to
		if toDir {
			dst = filepath.Join(dst, filepath.Base(source))
		}
		var err error
		if recursive {
			err = cpTreeDerivedPaths(ctx, source, dst, noFollow, visit)
		} else {
			err = cpFileDerivedPath(source, dst, noFollow, visit)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// cpFileDerivedPath yields the destination of cp without -r: nothing for a
// source that cannot be stat'ed (u-root fails on it) or is a directory (the
// pre-callback skips it), otherwise the destination with the use Copy gives
// the source's type.
//
//goplint:ignore -- raw OS paths of the resolved operand and its destination.
func cpFileDerivedPath(source, dst string, noFollow bool, visit derivedPathVisit) error {
	info, ok := cpStat(source, noFollow)
	if !ok || info.IsDir() {
		return nil
	}
	return visit(derivedPath{path: dst, use: cpUseFor(info, noFollow), sourceInfo: info})
}

// cpTreeDerivedPaths yields the destinations of CopyTree(src, dst). A walk
// error ends this source, as u-root's walk function returns it; an error
// from visit or a cancelled ctx ends the enumeration and is returned.
//
//goplint:ignore -- raw OS paths of the resolved operand and its destination.
func cpTreeDerivedPaths(ctx context.Context, src, dst string, noFollow bool, visit derivedPathVisit) error {
	var stop error
	// The walk's own error is not returned: a walk error ends only this
	// source, and stop carries every error that ends the enumeration.
	_ = filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipAll
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			stop = fmt.Errorf("cp: %w", ctxErr)
			return filepath.SkipAll
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return filepath.SkipAll
		}
		if visitErr := cpEntryDerivedPaths(path, filepath.Join(dst, rel), info, noFollow, visit); visitErr != nil {
			stop = visitErr
			return filepath.SkipAll
		}
		return nil
	})
	return stop
}

// cpEntryDerivedPaths yields what Copy(path, dst) touches for one walk entry
// whose os.Lstat info is lstatInfo. Without -P an entry that is neither a
// directory nor a regular file (a symbolic link, or on Windows a junction or
// another reparse point) is a source link that Copy follows; it is yielded
// first, whatever its target is. Then the destination is yielded with the use
// Copy gives the entry's type, or as a file write when Copy's stat fails.
//
//goplint:ignore -- raw OS paths of one walk entry and its destination.
func cpEntryDerivedPaths(path, dst string, lstatInfo fs.FileInfo, noFollow bool, visit derivedPathVisit) error {
	info := lstatInfo
	if !noFollow {
		if !lstatInfo.IsDir() && !lstatInfo.Mode().IsRegular() {
			if err := visit(derivedPath{path: path, use: derivedPathSourceLink}); err != nil {
				return err
			}
		}
		followed, ok := cpStat(path, false)
		if !ok {
			return visit(derivedPath{path: dst, use: derivedPathFileWrite})
		}
		info = followed
	}
	return visit(derivedPath{path: dst, use: cpUseFor(info, noFollow), sourceInfo: info})
}

// cpStat stats path as u-root's Options.stat does (os.Lstat under -P,
// os.Stat otherwise) and reports whether that succeeded. Every caller treats
// a failure as u-root does, whatever its cause, so the error is not kept.
//
//goplint:ignore -- raw OS path of a cp operand or source entry.
func cpStat(path string, noFollow bool) (fs.FileInfo, bool) {
	var (
		info fs.FileInfo
		err  error
	)
	if noFollow {
		info, err = os.Lstat(path)
	} else {
		info, err = os.Stat(path)
	}
	return info, err == nil
}

// pathMissing reports whether os.Stat of path fails because it does not
// exist, as os.IsNotExist decides it in u-root's ExtractDir.
//
//goplint:ignore -- raw OS path of the extraction directory.
func pathMissing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// cpUseFor returns what u-root's copyFile does at the destination for a
// source with info: a directory is created, a regular file written, a link
// created under -P; any other type is not supported and is yielded as a file
// write, which fails closed.
func cpUseFor(info fs.FileInfo, noFollow bool) derivedPathUse {
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return derivedPathDirCreate
	case mode.IsRegular():
		return derivedPathFileWrite
	case noFollow && mode&fs.ModeSymlink != 0:
		return derivedPathLinkCreate
	default:
		return derivedPathFileWrite
	}
}

// tarDerivedPaths yields, as u-root tar -x does (u-root v0.16.0,
// pkg/core/tar/tar.go and pkg/tarutil/tar.go ExtractDir and
// createFileInRoot), every path it will create, write or change the mode of.
// It runs only for -x without -c or -t, with one operand and an archive;
// otherwise u-root fails before extracting and nothing is yielded.
//
// The archive is the last -f/-file value, already resolved. One that exists
// and is not a regular file is refused (errTarArchiveNotRegular): it is read
// here and again by u-root. A missing archive or an extraction directory that
// exists and is not a directory yields nothing, as u-root fails on it. A
// missing extraction directory is yielded as a directory creation. For each
// entry the path is upath.SafeFilepathJoin(dir, name), the join u-root calls,
// and an entry it refuses is skipped, as u-root skips it. A regular-file
// entry yields a file write and a mode change, a directory entry a directory
// creation and a mode change; any other type, or a read error, ends the
// enumeration, because u-root stops there.
func tarDerivedPaths(ctx context.Context, resolved urootArgs, visit derivedPathVisit) error {
	if !resolved.boolOption("x", "extract") || resolved.boolOption("c", "create") || resolved.boolOption("t", "list") {
		return nil
	}
	archive := resolved.stringOption("", "f", "file")
	if archive == "" || len(resolved.operands) != 1 {
		return nil
	}
	archiveInfo, ok := cpStat(archive, false)
	if !ok {
		return nil
	}
	if !archiveInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", errTarArchiveNotRegular, archive)
	}
	dir := resolved.operands[0]
	dirInfo, dirExists := cpStat(dir, false)
	switch {
	case !dirExists && pathMissing(dir):
		if err := visit(derivedPath{path: dir, use: derivedPathDirCreate}); err != nil {
			return err
		}
	case !dirExists || !dirInfo.IsDir():
		return nil
	}
	return tarEntryDerivedPaths(ctx, archive, dir, visit)
}

// tarEntryDerivedPaths reads the archive once and yields each entry's paths.
//
//goplint:ignore -- raw OS paths of the resolved archive and extraction directory.
func tarEntryDerivedPaths(ctx context.Context, archive, dir string, visit derivedPathVisit) error {
	f, err := os.Open(archive)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }() // read-only; a close error changes nothing
	tr := tar.NewReader(f)
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("tar: %w", ctxErr)
		}
		hdr, err := tr.Next()
		if err != nil {
			return nil
		}
		path, err := upath.SafeFilepathJoin(dir, hdr.Name)
		if err != nil {
			continue
		}
		// u-root switches on these type bits: a regular file and a directory
		// are extracted, every other type stops the extraction.
		entryType := hdr.FileInfo().Mode() & fs.ModeType
		use := derivedPathFileWrite
		if entryType == fs.ModeDir {
			use = derivedPathDirCreate
		} else if entryType != 0 {
			return nil
		}
		if err := visit(derivedPath{path: path, use: use}); err != nil {
			return err
		}
		if err := visit(derivedPath{path: path, use: derivedPathModeChange}); err != nil {
			return err
		}
	}
}

// newCpDerivedChecker records the os.Lstat info of every source operand of
// resolved that exists.
func newCpDerivedChecker(hc *HandlerContext, resolved urootArgs) *cpDerivedChecker {
	checker := &cpDerivedChecker{
		hc:         hc,
		recursive:  resolved.boolOption("r", "R", "recursive", "RECURSIVE"),
		insideDirs: make(map[string]bool),
	}
	if len(resolved.operands) < 2 {
		return checker
	}
	for _, source := range resolved.operands[:len(resolved.operands)-1] {
		if info, err := os.Lstat(source); err == nil {
			checker.sources = append(checker.sources, info)
		}
	}
	return checker
}

// visit checks one path cp derives.
//
//   - A source operand after the first must exist (errCpSourceMissing). A
//     missing first source yields nothing and keeps u-root's own error.
//   - A followed source link is resolved and checked.
//   - A destination is resolved and checked. With -r it must not be at or
//     below a source operand of the command (errCpIntoSource), compared by
//     file identity of its existing part, unless the destination as u-root
//     builds it is already the source entry it copies, which u-root skips.
//   - A link the command creates must be its only destination
//     (errCpLinkWithOtherDestinations). No names are compared: which names a
//     volume treats as one entry cannot be told from their spelling.
func (c *cpDerivedChecker) visit(p derivedPath) error {
	switch p.use {
	case derivedPathSourceOperand:
		if _, err := os.Lstat(p.path); err != nil {
			return fmt.Errorf("%w: %s", errCpSourceMissing, p.path)
		}
		return nil
	case derivedPathSourceLink, derivedPathModeChange:
		_, err := c.hc.ResolvePath(p.path)
		return err
	case derivedPathFileWrite, derivedPathDirCreate, derivedPathLinkCreate:
		return c.checkDestination(p)
	default:
		_, err := c.hc.ResolvePath(p.path)
		return err
	}
}

func (c *cpDerivedChecker) checkDestination(p derivedPath) error {
	resolved, err := c.hc.ResolvePath(p.path)
	if err != nil {
		return err
	}
	if c.recursive && c.insideSource(resolved) && !sameSourceEntry(p) {
		return fmt.Errorf("%w: %s", errCpIntoSource, resolved)
	}
	c.destinations++
	if p.use == derivedPathLinkCreate && c.firstLink == "" {
		c.firstLink = resolved
	}
	if c.firstLink != "" && c.destinations > 1 {
		return fmt.Errorf("%w: %s", errCpLinkWithOtherDestinations, c.firstLink)
	}
	return nil
}

// insideSource reports whether path, a resolved destination, or one of its
// existing ancestors is the same file as a source operand. The existing part
// of a resolved path holds no link, so its ancestors are its real parents;
// its missing components are new entries, which are no existing source.
//
//goplint:ignore -- raw OS path of a resolved destination.
func (c *cpDerivedChecker) insideSource(path string) bool {
	if len(c.sources) == 0 {
		return false
	}
	if info, err := os.Lstat(path); err == nil && c.isSource(info) {
		return true
	}
	parent := filepath.Dir(path)
	if parent == path {
		return false
	}
	if inside, ok := c.insideDirs[parent]; ok {
		return inside
	}
	inside := c.insideSource(parent)
	c.insideDirs[parent] = inside
	return inside
}

func (c *cpDerivedChecker) isSource(info fs.FileInfo) bool {
	for _, source := range c.sources {
		if os.SameFile(source, info) {
			return true
		}
	}
	return false
}

// sameSourceEntry reports whether the destination, as u-root builds it and
// read without following its last component, is the source entry it copies:
// u-root's same-file check then skips it, or, under -P for a link, creating
// the link fails on the existing entry, so nothing is written.
func sameSourceEntry(p derivedPath) bool {
	if p.sourceInfo == nil {
		return false
	}
	info, err := os.Lstat(p.path)
	return err == nil && os.SameFile(info, p.sourceInfo)
}
