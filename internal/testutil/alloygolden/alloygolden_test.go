// SPDX-License-Identifier: MPL-2.0

package alloygolden

import (
	"bytes"
	"compress/gzip"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fixture has an empty sig (Key), a ternary relation (lock), a relation empty
// in every instance (never), and a duplicate instance (the third repeats the
// first). The same instances are encoded by scripts/test_formal.py.
const fixture = `{"model":"M","command":"golden","fingerprint":"format=2;source=x;","count":3,
"atoms":["Caller$0","Entry$0","Key$0","Key$1"],
"sig":{"Caller":{"index":[0,0,0],"values":[[0]]},"Key":{"index":[0,0,0],"values":[[]]}},
"rel":{"lock":{"arity":3,"index":[1,0,1],"values":[[],[0,2,1,0,3,1]]},"never":{"arity":2,"index":[0,0,0],"values":[[]]}}}`

func gzipped(t *testing.T, text string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestDecode_Format2(t *testing.T) {
	t.Parallel()
	withLock := Instance{
		Sig: map[string][]string{"Caller": {"Caller$0"}, "Key": {}},
		Rel: map[string][][]string{
			"lock":  {{"Caller$0", "Key$0", "Entry$0"}, {"Caller$0", "Key$1", "Entry$0"}},
			"never": {},
		},
	}
	empty := Instance{
		Sig: map[string][]string{"Caller": {"Caller$0"}, "Key": {}},
		Rel: map[string][][]string{"lock": {}, "never": {}},
	}
	_, got, err := decode(gzipped(t, fixture), 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Instance{withLock, empty, withLock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("decode = %#v, want %#v", got, want)
	}
	_, sample, err := decode(gzipped(t, fixture), 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Instance{withLock, withLock}; !reflect.DeepEqual(sample, want) {
		t.Fatalf("decode stride 2 = %#v, want %#v", sample, want)
	}
}

func TestDecode_RejectsFormat1(t *testing.T) {
	t.Parallel()
	_, _, err := decode(gzipped(t, strings.Replace(fixture, "format=2", "format=1", 1)), 1)
	if !errors.Is(err, errGoldenFormat) || !strings.Contains(err.Error(), "make formal-golden") {
		t.Fatalf("decode format 1: err = %v, want %v naming make formal-golden", err, errGoldenFormat)
	}
}

func TestDecode_RejectsMalformedColumns(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"short index":     strings.Replace(fixture, `"Key":{"index":[0,0,0]`, `"Key":{"index":[0,0]`, 1),
		"index range":     strings.Replace(fixture, `"index":[1,0,1]`, `"index":[2,0,1]`, 1),
		"atom range":      strings.Replace(fixture, `"values":[[0]]`, `"values":[[4]]`, 1),
		"arity remainder": strings.Replace(fixture, `"arity":3`, `"arity":4`, 1),
		"no instances":    strings.Replace(fixture, `"count":3`, `"count":0`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := decode(gzipped(t, text), 1); !errors.Is(err, errGoldenShape) {
				t.Fatalf("decode: err = %v, want %v", err, errGoldenShape)
			}
		})
	}
}
