// SPDX-License-Identifier: MPL-2.0

// Package alloygolden loads golden vectors exported from Alloy models by
// `scripts/formal.py golden`: every instance of a model's `golden` command at
// its declared scope. Tests replay each instance against the real Go code and
// compare the result with the model's decision recorded in the instance.
package alloygolden

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// shortStride keeps every Nth instance under `go test -short`. Mutation
// profiles run with -short, so each mutant pays for a stratified sample
// instead of the full enumeration; `make test` replays every instance.
const shortStride = 16

type (
	// Vectors is one exported golden file.
	Vectors struct {
		Model       string     `json:"model"`
		Command     string     `json:"command"`
		Fingerprint string     `json:"fingerprint"`
		Count       int        `json:"count"`
		Instances   []Instance `json:"instances"`
	}

	// Instance is one Alloy solution: sig atoms and relation tuples.
	Instance struct {
		Sig map[string][]string   `json:"sig"`
		Rel map[string][][]string `json:"rel"`
	}
)

// Load reads the golden file for model, checks that it was generated from the
// current formal/alloy/<model>.als (so a model edit without regeneration fails
// plain `go test`), and returns its instances. Under -short it returns every
// shortStride-th instance.
func Load(t *testing.T, path, model string) []Instance {
	t.Helper()
	file, err := os.Open(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("open golden vectors: %v", err)
	}
	defer func() { _ = file.Close() }()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip golden vectors: %v", err)
	}
	var vectors Vectors
	if err := json.NewDecoder(reader).Decode(&vectors); err != nil {
		t.Fatalf("decode golden vectors: %v", err)
	}
	if vectors.Model != model || vectors.Command != "golden" {
		t.Fatalf("golden vectors are for %s.%s, want %s.golden", vectors.Model, vectors.Command, model)
	}
	if vectors.Count == 0 || vectors.Count != len(vectors.Instances) {
		t.Fatalf("golden vectors count = %d, instances = %d (truncated file?)", vectors.Count, len(vectors.Instances))
	}
	source := modelSourceDigest(t, model)
	if !strings.Contains(vectors.Fingerprint, "source="+source+";") {
		t.Fatalf("%s is stale: formal/alloy/%s.als changed since it was generated; run `make formal-golden`", path, model)
	}
	if !testing.Short() {
		return vectors.Instances
	}
	sample := make([]Instance, 0, len(vectors.Instances)/shortStride+1)
	for i := 0; i < len(vectors.Instances); i += shortStride {
		sample = append(sample, vectors.Instances[i])
	}
	return sample
}

func modelSourceDigest(t *testing.T, model string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			break
		} else if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("stat go.mod: %v", statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root (go.mod) not found")
		}
		dir = parent
	}
	data, err := os.ReadFile(filepath.Join(dir, "formal", "alloy", model+".als"))
	if err != nil {
		t.Fatalf("read model source: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Atoms returns the atoms of a sig.
func (inst Instance) Atoms(sig string) []string { return inst.Sig[sig] }

// One returns the single atom of a `one sig`, failing the test otherwise.
func (inst Instance) One(t *testing.T, sig string) string {
	t.Helper()
	atoms := inst.Sig[sig]
	if len(atoms) != 1 {
		t.Fatalf("sig %s has %d atoms, want exactly one", sig, len(atoms))
	}
	return atoms[0]
}

// In reports whether atom belongs to sig (including subset sigs).
func (inst Instance) In(sig, atom string) bool { return slices.Contains(inst.Sig[sig], atom) }

// Targets returns the atoms related to atom by a binary relation.
func (inst Instance) Targets(rel, atom string) []string {
	var out []string
	for _, tuple := range inst.Rel[rel] {
		if len(tuple) == 2 && tuple[0] == atom {
			out = append(out, tuple[1])
		}
	}
	return out
}

// Target returns the single atom related to atom by rel, or "".
func (inst Instance) Target(rel, atom string) string {
	if targets := inst.Targets(rel, atom); len(targets) > 0 {
		return targets[0]
	}
	return ""
}

// ID turns an atom label such as "Mod$1" into a lowercase identifier "mod1".
func ID(atom string) string { return strings.ToLower(strings.ReplaceAll(atom, "$", "")) }

// GitURL maps an atom to a valid, unique Git URL.
func GitURL(atom string) string { return fmt.Sprintf("https://example.com/%s.git", ID(atom)) }

// ModuleID maps an atom to a valid, unique RDNS module ID.
func ModuleID(atom string) string { return "io.example." + ID(atom) }
