// SPDX-License-Identifier: MPL-2.0

package fstree

import (
	"os"
	"path/filepath"
	"testing"
)

// linkSpec: root/{d/{f, chain -> l1}, l1 -> d/f, dangling}, with the
// link-to-link declared before its target link.
func linkSpec() Spec {
	return Spec{RootAtom: "root", Nodes: []Node{
		{Atom: "d", Parent: "root", Name: "d", Kind: KindDir},
		{Atom: "f", Parent: "d", Name: "f", Kind: KindFile, Content: "F"},
		{Atom: "chain", Parent: "d", Name: "chain", Kind: KindSymlink, Target: "l1"},
		{Atom: "l1", Parent: "root", Name: "l1", Kind: KindSymlink, Target: "f"},
		{Atom: "dangling", Parent: "root", Name: "dangling", Kind: KindSymlink, Dangling: true},
	}}
}

// TestMaterializeLinkTargets: by default every symlink target is absolute;
// with RelativeLinkTargets every target is relative to the link's directory
// and resolves to the same node.
func TestMaterializeLinkTargets(t *testing.T) {
	t.Parallel()
	Require(t, CapSymlink)
	for _, relative := range []bool{false, true} {
		b, skip := MaterializeWithOptions(t, linkSpec(), Probe(t), Options{RelativeLinkTargets: relative})
		if skip != "" {
			t.Fatalf("skip %q after Require", skip)
		}
		for _, atom := range []string{"chain", "l1", "dangling"} {
			target, err := os.Readlink(b.Path[atom])
			if err != nil {
				t.Fatalf("readlink %s: %v", atom, err)
			}
			if filepath.IsAbs(target) == relative {
				t.Errorf("relative=%v: %s target %q", relative, atom, target)
			}
		}
		for _, atom := range []string{"chain", "l1"} {
			data, err := os.ReadFile(b.Path[atom])
			if err != nil || string(data) != "F" {
				t.Errorf("relative=%v: read through %s = %q, %v; want the file", relative, atom, data, err)
			}
		}
		if _, err := os.Stat(b.Path["dangling"]); !os.IsNotExist(err) {
			t.Errorf("relative=%v: dangling link resolves (%v)", relative, err)
		}
	}
}

// TestLinkCreationOrder: a link to a link comes after its target; a cycle and
// a self link are still all placed.
func TestLinkCreationOrder(t *testing.T) {
	t.Parallel()
	spec := Spec{RootAtom: "root", Nodes: []Node{
		{Atom: "a", Parent: "root", Name: "a", Kind: KindSymlink, Target: "b"},
		{Atom: "b", Parent: "root", Name: "b", Kind: KindSymlink, Target: "c"},
		{Atom: "c", Parent: "root", Name: "c", Kind: KindFile},
		{Atom: "x", Parent: "root", Name: "x", Kind: KindSymlink, Target: "y"},
		{Atom: "y", Parent: "root", Name: "y", Kind: KindSymlink, Target: "x"},
		{Atom: "s", Parent: "root", Name: "s", Kind: KindSymlink, Target: "s"},
	}}
	byAtom := map[string]Node{}
	for _, n := range spec.Nodes {
		byAtom[n.Atom] = n
	}
	order := linkCreationOrder(spec, byAtom)
	pos := map[string]int{}
	for i, n := range order {
		pos[n.Atom] = i
	}
	if len(order) != 5 {
		t.Fatalf("order %v, want the five links", order)
	}
	if pos["a"] < pos["b"] {
		t.Errorf("a (-> b) placed before b: %v", pos)
	}
}
