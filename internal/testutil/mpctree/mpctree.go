// SPDX-License-Identifier: MPL-2.0

// Package mpctree translates a ModulePathContainment Alloy golden instance into
// an fstree.Spec plus the operation metadata the replay needs, so the golden
// replays in internal/app/moduleops, pkg/invowkfile, and internal/runtime share
// one translation of the shared golden file.
package mpctree

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/testutil/alloygolden"
	"github.com/invowk/invowk/internal/testutil/fstree"
)

// Constants named as the model's sigs: operation kinds and reference shapes,
// plus the exact directory name the vendor scan skips.
const (
	VendoredModulesDir = "invowk_modules"

	ScriptRead       OpKind = "ScriptRead"
	EnvRead          OpKind = "EnvRead"
	ContainerfileRef OpKind = "ContainerfileRef"
	WorkdirCwd       OpKind = "WorkdirCwd"
	VendorCopy       OpKind = "VendorCopy"
	Hash             OpKind = "Hash"
	Unpack           OpKind = "Unpack"

	RelInside    RefShape = "RelInside"
	ParentEscape RefShape = "ParentEscape"
	AbsPath      RefShape = "AbsPath"
)

type (
	// OpKind is a ModulePathContainment operation kind.
	OpKind string
	// RefShape is the syntactic form of an operation's reference.
	RefShape string

	// Case is one translated instance: the tree to materialise, the operation, and
	// the layer decisions the model recorded for it.
	Case struct {
		Spec       fstree.Spec
		OwnerAtom  string // module root dir atom
		OpKind     OpKind
		RefAtom    string // node the op references
		RefShape   RefShape
		Validates  bool
		LexInside  bool
		PhysInside bool
		Accepts    bool
		Contained  bool
		TouchAtom  string // node physically touched, or "" (none)
	}

	// Reporter collects replay disagreements, printing the first few in full.
	Reporter struct {
		t     *testing.T
		count int
	}
)

// FromInstance builds a Case from one Alloy instance.
func FromInstance(inst alloygolden.Instance) Case {
	rootAtom := inst.Atoms("Root")[0]
	moduleAtom := inst.Atoms("Module")[0]
	mroot := inst.Target("mroot", moduleAtom)
	vendorName := inst.Target("vendorName", inst.Atoms("FS")[0])

	nodes := make([]fstree.Node, 0, 8)
	label := func(atom string) string { return inst.Target("label", atom) }
	// stable, valid file/dir names derived from the label atom
	nameOf := func(atom string) string {
		l := label(atom)
		if l == vendorName {
			return VendoredModulesDir
		}
		return "n" + alloygolden.ID(l)
	}

	moduleID := "m" + alloygolden.ID(moduleAtom)
	for _, d := range inst.Atoms("Dir") {
		if d == rootAtom {
			continue
		}
		n := fstree.Node{Atom: d, Parent: inst.Target("parent", d), Kind: fstree.KindDir, Name: nameOf(d)}
		if d == mroot {
			n.ModuleRoot = true
			n.Name = moduleID + ".invowkmod"
		}
		nodes = append(nodes, n)
	}
	for _, f := range inst.Atoms("File") {
		nodes = append(nodes, fstree.Node{Atom: f, Parent: inst.Target("parent", f), Kind: fstree.KindFile, Name: nameOf(f), Content: "content-" + alloygolden.ID(f)})
	}
	for _, l := range inst.Atoms("Link") {
		kind := fstree.KindSymlink
		if inst.In("JunKind", inst.Target("kind", l)) {
			kind = fstree.KindJunction
		}
		target := inst.Target("tgt", l)
		nodes = append(nodes, fstree.Node{
			Atom: l, Parent: inst.Target("parent", l), Kind: kind,
			Name: nameOf(l), Target: target, Dangling: target == "",
		})
	}

	c := Case{
		Spec:      fstree.Spec{RootAtom: rootAtom, Nodes: nodes},
		OwnerAtom: mroot,
		OpKind:    OpKindOf(inst),
	}
	opAtom := inst.Atoms("Op")[0]
	c.RefShape = RefShape(Which(inst, inst.Target("refShape", opAtom), string(RelInside), string(ParentEscape), string(AbsPath)))
	c.RefAtom = inst.Target("ref", opAtom)
	dec := inst.Atoms("Decisions")[0]
	c.Validates = slices.Contains(inst.Targets("dValidates", dec), opAtom)
	c.LexInside = slices.Contains(inst.Targets("dLexContains", dec), opAtom)
	c.PhysInside = slices.Contains(inst.Targets("dPhysContains", dec), opAtom)
	c.Accepts = slices.Contains(inst.Targets("dAccepts", dec), opAtom)
	c.Contained = slices.Contains(inst.Targets("dContained", dec), opAtom)
	c.TouchAtom = touchTarget(inst, dec, opAtom)
	return c
}

// OpKindOf returns the instance's operation kind without translating the rest
// of it, so a replay can filter instances cheaply.
func OpKindOf(inst alloygolden.Instance) OpKind {
	k := inst.Target("kind", inst.Atoms("Op")[0])
	return OpKind(Which(inst, k, string(ScriptRead), string(EnvRead), string(ContainerfileRef),
		string(WorkdirCwd), string(VendorCopy), string(Hash), string(Unpack)))
}

// Which returns the first of the named sigs that contains atom, or "".
func Which(inst alloygolden.Instance, atom string, sigs ...string) string {
	for _, sig := range sigs {
		if inst.In(sig, atom) {
			return sig
		}
	}
	return ""
}

// touchTarget reads the arity-3 dTouch relation (Decisions -> Op -> Node).
func touchTarget(inst alloygolden.Instance, dec, opAtom string) string {
	for _, tuple := range inst.Rel["dTouch"] {
		if len(tuple) == 3 && tuple[0] == dec && tuple[1] == opAtom {
			return tuple[2]
		}
	}
	return ""
}

// RelPath returns the module-relative path string for the op reference, as the
// value type would see it: a clean relative path for RelInside, a `..` path for
// ParentEscape, and the absolute path for AbsPath.
func (c Case) RelPath(built *fstree.Built) string {
	refReal := built.Path[c.RefAtom]
	moduleRoot := built.Path[c.OwnerAtom]
	if c.RefShape == AbsPath {
		return refReal
	}
	rel, err := filepath.Rel(moduleRoot, refReal)
	if err != nil {
		return refReal
	}
	return filepath.ToSlash(rel)
}

// StructuralKey identifies the materialised tree (independent of the
// operation), so replays memoise one materialisation per distinct tree.
func (c Case) StructuralKey() string { return fmt.Sprintf("%v", c.Spec) }

// NewReporter returns a Reporter for t.
func NewReporter(t *testing.T) *Reporter {
	t.Helper()
	return &Reporter{t: t}
}

// Errorf records one disagreement.
func (r *Reporter) Errorf(format string, args ...any) {
	r.t.Helper()
	r.count++
	if r.count <= 8 {
		r.t.Errorf(format, args...)
	}
}

// Finish fails the test when any disagreement was recorded.
func (r *Reporter) Finish(model string, instances int) {
	r.t.Helper()
	if r.count > 0 {
		r.t.Fatalf("%d disagreements with %s over %d instances", r.count, model, instances)
	}
}
