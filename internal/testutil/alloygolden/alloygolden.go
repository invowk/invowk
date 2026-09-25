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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	// shortStride keeps every Nth instance under `go test -short`. Mutation
	// profiles run with -short, so each mutant pays for a stratified sample
	// instead of the full enumeration; `make test` replays every instance.
	shortStride = 16

	// goldenFormat is the only vector format Load accepts (scripts/formal.py
	// GOLDEN_FORMAT).
	goldenFormat = "2"
)

var (
	errGoldenFormat = errors.New("unsupported golden vector format")
	errGoldenShape  = errors.New("malformed golden vectors")

	formatRE = regexp.MustCompile(`^format=(\d+);`)
)

type (
	// Instance is one Alloy solution: sig atoms and relation tuples. Every
	// sig and relation of the model has a key, with an empty slice when it
	// has no atoms or tuples.
	Instance struct {
		Sig map[string][]string
		Rel map[string][][]string
	}

	// vectors is one exported golden file in format 2: a sorted atom
	// dictionary and one column per sig and relation. A column stores its
	// distinct values once, as atom-index lists (relations flattened
	// row-major), and Index selects each instance's value.
	vectors struct {
		Model       string            `json:"model"`
		Command     string            `json:"command"`
		Fingerprint string            `json:"fingerprint"`
		Count       int               `json:"count"`
		Atoms       []string          `json:"atoms"`
		Sig         map[string]column `json:"sig"`
		Rel         map[string]column `json:"rel"`
	}

	column struct {
		Arity  int     `json:"arity"`
		Values [][]int `json:"values"`
		Index  []int   `json:"index"`
	}

	// decodedColumn holds a column's distinct values as tuples of atom labels.
	decodedColumn struct {
		values [][][]string
		index  []int
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
	stride := 1
	if testing.Short() {
		stride = shortStride
	}
	vecs, instances, err := decode(file, stride)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if vecs.Model != model || vecs.Command != "golden" {
		t.Fatalf("golden vectors are for %s.%s, want %s.golden", vecs.Model, vecs.Command, model)
	}
	source := modelSourceDigest(t, model)
	if !strings.Contains(vecs.Fingerprint, "source="+source+";") {
		t.Fatalf("%s is stale: formal/alloy/%s.als changed since it was generated; run `make formal-golden`", path, model)
	}
	return instances
}

// decode reads format-2 vectors and returns every stride-th instance.
func decode(r io.Reader, stride int) (vectors, []Instance, error) {
	reader, err := gzip.NewReader(r)
	if err != nil {
		return vectors{}, nil, fmt.Errorf("gzip golden vectors: %w", err)
	}
	var vecs vectors
	if err = json.NewDecoder(reader).Decode(&vecs); err != nil {
		return vectors{}, nil, fmt.Errorf("decode golden vectors: %w", err)
	}
	if m := formatRE.FindStringSubmatch(vecs.Fingerprint); len(m) < 2 || m[1] != goldenFormat {
		return vectors{}, nil, fmt.Errorf("%w (fingerprint %q, want format=%s); run `make formal-golden`",
			errGoldenFormat, vecs.Fingerprint, goldenFormat)
	}
	if vecs.Count == 0 {
		return vectors{}, nil, fmt.Errorf("%w: no instances (truncated file?)", errGoldenShape)
	}
	sigs, err := decodeColumns(vecs, vecs.Sig, false)
	if err != nil {
		return vectors{}, nil, err
	}
	rels, err := decodeColumns(vecs, vecs.Rel, true)
	if err != nil {
		return vectors{}, nil, err
	}
	instances := make([]Instance, 0, (vecs.Count+stride-1)/stride)
	for k := 0; k < vecs.Count; k += stride {
		inst := Instance{Sig: make(map[string][]string, len(sigs)), Rel: make(map[string][][]string, len(rels))}
		for name, col := range sigs {
			// Sig values are atom lists: tuples of arity 1.
			atoms := make([]string, 0, len(col.values[col.index[k]]))
			for _, tuple := range col.values[col.index[k]] {
				atoms = append(atoms, tuple[0])
			}
			inst.Sig[name] = atoms
		}
		for name, col := range rels {
			inst.Rel[name] = col.values[col.index[k]]
		}
		instances = append(instances, inst)
	}
	return vecs, instances, nil
}

// decodeColumns resolves every column's values to atom labels once, and
// validates indices, so decode only selects per instance.
func decodeColumns(vecs vectors, cols map[string]column, relation bool) (map[string]decodedColumn, error) {
	out := make(map[string]decodedColumn, len(cols))
	for name, col := range cols {
		arity := 1
		if relation {
			arity = col.Arity
		}
		if arity < 1 || len(col.Index) != vecs.Count {
			return nil, fmt.Errorf("%w: column %s has arity %d and %d indices, want %d", errGoldenShape, name, arity, len(col.Index), vecs.Count)
		}
		values := make([][][]string, len(col.Values))
		for v, flat := range col.Values {
			if len(flat)%arity != 0 {
				return nil, fmt.Errorf("%w: column %s value %d is not a multiple of arity %d", errGoldenShape, name, v, arity)
			}
			tuples := make([][]string, 0, len(flat)/arity)
			for i := 0; i < len(flat); i += arity {
				tuple := make([]string, arity)
				for j, a := range flat[i : i+arity] {
					if a < 0 || a >= len(vecs.Atoms) {
						return nil, fmt.Errorf("%w: column %s names atom %d of %d", errGoldenShape, name, a, len(vecs.Atoms))
					}
					tuple[j] = vecs.Atoms[a]
				}
				tuples = append(tuples, tuple)
			}
			values[v] = tuples
		}
		for k, v := range col.Index {
			if v < 0 || v >= len(values) {
				return nil, fmt.Errorf("%w: column %s instance %d selects value %d of %d", errGoldenShape, name, k, v, len(values))
			}
		}
		out[name] = decodedColumn{values: values, index: col.Index}
	}
	return out, nil
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
