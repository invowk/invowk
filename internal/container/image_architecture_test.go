// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

const imageArchitectureQueryKey = "image inspect --format {{.Architecture}} debian:stable-slim"

func TestQueryImageArchitecture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		engine string
		answer string
		fail   error
		want   ImageArchitecture
	}{
		{name: "docker amd64", engine: "docker", answer: "amd64\n", want: "amd64"},
		{name: "podman arm64 with surrounding whitespace", engine: "podman", answer: "  arm64 \n", want: "arm64"},
		{name: "empty output", engine: "docker", answer: "\n", want: ImageArchitectureUnknown},
		{name: "two lines", engine: "podman", answer: "amd64\narm64\n", want: ImageArchitectureUnknown},
		{name: "query error", engine: "docker", fail: errors.New("No such image: debian:stable-slim"), want: ImageArchitectureUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := &fakeQueryEngine{
				name:    tt.engine,
				answers: map[string]string{imageArchitectureQueryKey: tt.answer},
				fail:    map[string]error{imageArchitectureQueryKey: tt.fail},
			}
			got := QueryImageArchitecture(t.Context(), engine, "debian:stable-slim")
			if got != tt.want {
				t.Fatalf("QueryImageArchitecture() = %q, want %q", got, tt.want)
			}
			if len(engine.calls) != 1 || engine.calls[0] != imageArchitectureQueryKey {
				t.Fatalf("queries = %q, want exactly [%q]", engine.calls, imageArchitectureQueryKey)
			}
		})
	}
}

func TestQueryImageArchitectureWithoutRunnerOrImage(t *testing.T) {
	t.Parallel()

	if got := QueryImageArchitecture(t.Context(), nil, "debian:stable-slim"); got != ImageArchitectureUnknown {
		t.Fatalf("QueryImageArchitecture(nil runner) = %q, want unknown", got)
	}
	engine := &fakeQueryEngine{name: "docker"}
	if got := QueryImageArchitecture(t.Context(), engine, ""); got != ImageArchitectureUnknown {
		t.Fatalf("QueryImageArchitecture(empty image) = %q, want unknown", got)
	}
	if len(engine.calls) != 0 {
		t.Fatalf("queries = %q, want none for an empty image", engine.calls)
	}
}

func TestImageArchitectureValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []ImageArchitecture{"amd64", "arm64", ImageArchitectureUnknown} {
		if err := valid.Validate(); err != nil {
			t.Errorf("ImageArchitecture(%q).Validate() = %v, want nil", valid, err)
		}
	}
	for _, invalid := range []ImageArchitecture{"", "amd 64", "arm64\n"} {
		err := invalid.Validate()
		if !errors.Is(err, ErrInvalidImageArchitecture) {
			t.Errorf("ImageArchitecture(%q).Validate() = %v, want ErrInvalidImageArchitecture", invalid, err)
		}
		var archErr *InvalidImageArchitectureError
		if !errors.As(err, &archErr) || archErr.Value != invalid {
			t.Errorf("ImageArchitecture(%q).Validate() error = %#v, want InvalidImageArchitectureError", invalid, err)
		}
	}
	if !ImageArchitectureUnknown.IsUnknown() || ImageArchitecture("amd64").IsUnknown() {
		t.Error("IsUnknown() mismatch")
	}
}
