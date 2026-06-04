// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"errors"
	"strings"
	"testing"
)

type (
	parseMutationDecodeTarget struct {
		Count int `json:"count"`
	}

	parseMutationLooseConfig struct {
		Mode string `json:"mode,omitempty"`
	}
)

func TestParseAndDecodeMutationErrorContracts(t *testing.T) {
	t.Parallel()

	t.Run("default filename is used for early size errors", func(t *testing.T) {
		t.Parallel()

		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), []byte("abcd"), "#TestConfig", WithMaxFileSize(3))
		requireParseError(t, err, "<input>: file size exceeds maximum: 4 bytes exceeds 3 bytes")
		if !errors.Is(err, ErrFileSizeExceeded) {
			t.Fatalf("ParseAndDecode() error = %v, want ErrFileSizeExceeded", err)
		}
	})

	t.Run("schema compile failures are internal errors", func(t *testing.T) {
		t.Parallel()

		data := []byte(`name: "test"
count: 1
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(`#TestConfig: {name: `), data, "#TestConfig")
		requireParseError(t, err, "internal error: failed to compile schema:")
	})

	t.Run("user compile failures are formatted with filename", func(t *testing.T) {
		t.Parallel()

		data := []byte(`name: "test"
count:
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig", WithFilename("bad.cue"))
		requireParseError(t, err, "bad.cue:")
	})

	t.Run("missing schema definition is an internal error", func(t *testing.T) {
		t.Parallel()

		data := []byte(`name: "test"
count: 1
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#Missing")
		requireParseError(t, err, "internal error: schema definition #Missing not found:")
	})

	t.Run("concrete validation formats incomplete values", func(t *testing.T) {
		t.Parallel()

		data := []byte(`name: "test"
enabled: true
`)
		_, err := ParseAndDecode[TestConfig]([]byte(testSchema), data, "#TestConfig")
		requireParseError(t, err, "<input>: #TestConfig.count:")
	})

	t.Run("non-concrete validation still rejects invalid values", func(t *testing.T) {
		t.Parallel()

		schema := []byte(`#Loose: {
mode?: "on" | "off"
}`)
		data := []byte(`mode: "bad"`)
		_, err := ParseAndDecode[parseMutationLooseConfig](schema, data, "#Loose", WithConcrete(false))
		requireParseError(t, err, "#Loose.mode:")
	})

	t.Run("decode failures are formatted with filename", func(t *testing.T) {
		t.Parallel()

		schema := []byte(`#DecodeTarget: {
count: string
}`)
		data := []byte(`count: "forty-two"`)
		_, err := ParseAndDecode[parseMutationDecodeTarget](
			schema,
			data,
			"#DecodeTarget",
			WithFilename("decode.cue"),
		)
		requireParseError(t, err, "decode.cue:")
	})
}

func requireParseError(t *testing.T, err error, wantSubstring string) {
	t.Helper()

	if err == nil {
		t.Fatalf("ParseAndDecode() error = nil, want substring %q", wantSubstring)
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("ParseAndDecode() error = %q, want substring %q", err.Error(), wantSubstring)
	}
}
