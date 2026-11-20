// SPDX-License-Identifier: MPL-2.0

// Package fstree materialises a bounded filesystem tree described by an abstract
// spec (typically translated from an Alloy golden instance) as a real directory
// tree under a test temporary directory, so that formal-verification replays
// exercise os.ReadFile, os.Lstat, filepath.EvalSymlinks, and filepath.WalkDir
// exactly as production does. It maps every spec atom back to its real path,
// probes symlink and junction capability (keeping each probe's error), and
// applies a per-platform policy to an instance that needs a capability the
// host lacks: it fails where the capability is required and skips elsewhere.
package fstree

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const (
	// KindDir is a directory.
	KindDir Kind = iota
	// KindFile is a regular file.
	KindFile
	// KindSymlink is a symbolic link.
	KindSymlink
	// KindJunction is a Windows directory junction (an NTFS mount point).
	KindJunction
)

// probeOnce probes the host's temporary filesystem once per test process.
var probeOnce = sync.OnceValue(func() Caps {
	base, err := os.MkdirTemp("", "fstree-probe-*")
	if err != nil {
		return Caps{SymlinkErr: err, JunctionErr: err}
	}
	defer func() { _ = os.RemoveAll(base) }()
	symlinkErr := probeSymlink(base)
	junctionErr := probeJunction(base)
	return Caps{
		Symlink: symlinkErr == nil, SymlinkErr: symlinkErr,
		Junction: junctionErr == nil, JunctionErr: junctionErr,
	}
})

type (
	// Fataler is the part of *testing.T and *rapid.T a fixture writer needs.
	Fataler interface {
		Helper()
		Fatalf(format string, args ...any)
	}

	// Kind is the filesystem kind of a node.
	Kind int

	// Node is one node of the tree. Parent is the atom of its parent directory
	// (the spec's RootAtom for top-level nodes). Target is the atom a link
	// resolves to; a Dangling link points at a fixed missing name instead.
	Node struct {
		Atom     string
		Parent   string
		Name     string
		Kind     Kind
		Target   string
		Dangling bool
		// Content is written into a regular file; it defaults to the atom.
		Content string
		// ModuleRoot marks a directory as a module root: the helper writes a
		// fixed invowkmod.cue (module id = the name before ".invowkmod") and
		// invowkfile.cue into it.
		ModuleRoot bool
	}

	// Spec is a tree to materialise. RootAtom names the host root directory.
	Spec struct {
		RootAtom string
		Nodes    []Node
	}

	// Caps records which filesystem capabilities the host offers, and why a
	// probe failed.
	Caps struct {
		Symlink     bool
		Junction    bool
		SymlinkErr  error
		JunctionErr error
	}

	// Options tunes how a tree is materialised. The zero value is the
	// default every replay uses.
	Options struct {
		// RelativeLinkTargets writes each symlink's target relative to the
		// link's parent directory instead of as an absolute path. Junctions
		// stay absolute (NTFS mount points hold absolute targets). Links are
		// created after their targets either way.
		RelativeLinkTargets bool
	}

	// Built is a materialised tree. Path maps every atom to its real path.
	Built struct {
		Root string
		Path map[string]string
	}

	// Cache memoises one materialisation per distinct tree across the instances
	// of a golden replay and counts the instances skipped per missing capability
	// that the host is not required to offer.
	Cache struct {
		caps  Caps
		host  Host
		opts  Options
		built map[string]*Built
		skips map[Capability]int
	}
)

// Probe reports the capabilities of the host's temporary filesystem.
func Probe(t *testing.T) Caps {
	t.Helper()
	return probeOnce()
}

// TempRoot returns filepath.EvalSymlinks(t.TempDir()): a temporary directory
// with out-of-model ancestor links (macOS /var -> /private/var) resolved.
func TempRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	return root
}

// NewCache probes the host once and returns an empty materialisation cache.
func NewCache(t *testing.T) *Cache {
	t.Helper()
	return newCache(Probe(t), CurrentHost())
}

// NewCacheWithOptions is NewCache whose trees are materialised with opts.
func NewCacheWithOptions(t *testing.T, opts Options) *Cache {
	t.Helper()
	c := NewCache(t)
	c.opts = opts
	return c
}

// newCache is NewCache with the probe result and the platform injected, so
// the policy test can drive Get on any GOOS.
func newCache(caps Caps, host Host) *Cache {
	return &Cache{caps: caps, host: host, built: map[string]*Built{}, skips: map[Capability]int{}}
}

// Get returns the tree for spec, materialising it on first use of key. When
// the host lacks a needed capability, it fails t if the capability is required
// on this platform (Capability.Required), and otherwise returns nil and counts
// the skip.
func (c *Cache) Get(t *testing.T, key string, spec Spec) *Built {
	t.Helper()
	if b, ok := c.built[key]; ok {
		return b
	}
	if skip := skipReason(spec, c.caps); skip != "" {
		c.missing(t, skip)
		return nil
	}
	b, _ := MaterializeWithOptions(t, spec, c.caps, c.opts)
	c.built[key] = b
	return b
}

// LogSkips reports the skipped-instance count per missing capability.
func (c *Cache) LogSkips(t *testing.T) {
	t.Helper()
	for capability, n := range c.skips {
		t.Logf("skipped %d instances needing capability %q (not required on %s)", n, string(capability), c.host.GOOS)
	}
}

// missing applies the policy to an instance that needs the missing
// capability: it fails t when the capability is required here, and otherwise
// counts the skip.
func (c *Cache) missing(t Fataler, skip Capability) {
	t.Helper()
	if skip.Required(c.host) {
		_, err := c.caps.Has(skip)
		t.Fatalf("%s is required on %s: %v", skip, c.host.GOOS, err)
		return
	}
	c.skips[skip]++
}

// Materialize builds the tree under filepath.EvalSymlinks(t.TempDir()), which
// removes out-of-model ancestor links such as macOS's /var -> /private/var. It
// returns a non-empty skip reason (the missing capability) and a nil tree when
// the spec needs a capability the host lacks; callers apply the policy (Cache.Get,
// Require).
func Materialize(t *testing.T, spec Spec, caps Caps) (built *Built, skip Capability) {
	t.Helper()
	return MaterializeWithOptions(t, spec, caps, Options{})
}

// MaterializeWithOptions is Materialize with opts applied.
func MaterializeWithOptions(t *testing.T, spec Spec, caps Caps, opts Options) (built *Built, skip Capability) {
	t.Helper()
	if skip = skipReason(spec, caps); skip != "" {
		return nil, skip
	}
	root := TempRoot(t)
	built = &Built{Root: root, Path: map[string]string{spec.RootAtom: root}}
	byAtom := make(map[string]Node, len(spec.Nodes))
	for _, n := range spec.Nodes {
		byAtom[n.Atom] = n
	}
	// Directories first (MkdirAll creates missing parents), then files, then
	// links, whose targets may be any other node.
	for _, n := range spec.Nodes {
		if n.Kind != KindDir {
			continue
		}
		p := built.pathOf(byAtom, n)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		built.Path[n.Atom] = p
		if n.ModuleRoot {
			WriteModuleFiles(t, p)
		}
	}
	for _, n := range spec.Nodes {
		if n.Kind != KindFile {
			continue
		}
		p := built.pathOf(byAtom, n)
		if err := os.WriteFile(p, []byte(cmp.Or(n.Content, n.Atom)), 0o644); err != nil {
			t.Fatalf("write file %s: %v", p, err)
		}
		built.Path[n.Atom] = p
	}
	for _, n := range linkCreationOrder(spec, byAtom) {
		built.createLink(t, byAtom, n, opts)
	}
	return built, ""
}

// linkCreationOrder returns the spec's links so that a link whose target is
// another link comes after that link (Windows picks a symlink's file or
// directory type from its target when it is created). Links on a cycle keep
// their spec order once nothing else can be placed.
func linkCreationOrder(spec Spec, byAtom map[string]Node) []Node {
	isLink := func(n Node) bool { return n.Kind == KindSymlink || n.Kind == KindJunction }
	var pending []Node
	for _, n := range spec.Nodes {
		if isLink(n) {
			pending = append(pending, n)
		}
	}
	created := map[string]bool{}
	order := make([]Node, 0, len(pending))
	for len(pending) > 0 {
		var rest []Node
		for _, n := range pending {
			target, ok := byAtom[n.Target]
			if !n.Dangling && ok && isLink(target) && !created[target.Atom] && target.Atom != n.Atom {
				rest = append(rest, n)
				continue
			}
			created[n.Atom] = true
			order = append(order, n)
		}
		if len(rest) == len(pending) { // a cycle: place the first and retry
			created[rest[0].Atom] = true
			order = append(order, rest[0])
			rest = rest[1:]
		}
		pending = rest
	}
	return order
}

// Within reports whether path lies inside (or equals) root, lexically. It is
// the replays' independent oracle; do not replace it with the production
// pathWithin it checks.
func Within(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// pathOf returns a node's lexical path, walking parent pointers, so it can name
// a node that is not created yet (a link target).
func (b *Built) pathOf(byAtom map[string]Node, n Node) string {
	if parent, ok := b.Path[n.Parent]; ok {
		return filepath.Join(parent, n.Name)
	}
	return filepath.Join(b.pathOf(byAtom, byAtom[n.Parent]), n.Name)
}

func (b *Built) createLink(t *testing.T, byAtom map[string]Node, n Node, opts Options) {
	t.Helper()
	linkPath := b.pathOf(byAtom, n)
	target := filepath.Join(b.Root, "does-not-exist-"+n.Atom)
	if !n.Dangling && n.Target != "" {
		target = b.pathOf(byAtom, byAtom[n.Target])
	}
	if opts.RelativeLinkTargets && n.Kind == KindSymlink {
		rel, err := filepath.Rel(filepath.Dir(linkPath), target)
		if err != nil {
			t.Fatalf("relative target of %s -> %s: %v", linkPath, target, err)
		}
		target = rel
	}
	var err error
	if n.Kind == KindJunction {
		err = createJunction(target, linkPath)
	} else {
		err = createSymlink(target, linkPath)
	}
	if err != nil {
		t.Fatalf("link %s -> %s: %v", linkPath, target, err)
	}
	b.Path[n.Atom] = linkPath
}

// WriteModuleFiles writes the fixed invowkmod.cue and invowkfile.cue into a
// module root dir; the module id is the directory name before ".invowkmod".
// t is a *testing.T or a *rapid.T, so a property check fails its own case.
func WriteModuleFiles(t Fataler, dir string) {
	t.Helper()
	files := map[string]string{
		"invowkmod.cue":  "module: \"" + strings.TrimSuffix(filepath.Base(dir), ".invowkmod") + "\"\nversion: \"1.0.0\"\n",
		"invowkfile.cue": "cmds: {}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func skipReason(spec Spec, caps Caps) Capability {
	for _, n := range spec.Nodes {
		if n.Kind == KindSymlink && !caps.Symlink {
			return CapSymlink
		}
		if n.Kind == KindJunction && !caps.Junction {
			return CapJunction
		}
	}
	return ""
}

func probeSymlink(base string) error {
	target := filepath.Join(base, "probe-target")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		return fmt.Errorf("write probe target: %w", err)
	}
	return createSymlink(target, filepath.Join(base, "probe-link"))
}
