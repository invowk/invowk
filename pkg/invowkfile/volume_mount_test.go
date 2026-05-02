// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
)

func TestVolumeMountSpec_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    VolumeMountSpec
		want    bool
		wantErr bool
	}{
		{"host:container", VolumeMountSpec("/host:/container"), true, false},
		{"with ro option", VolumeMountSpec("/host:/container:ro"), true, false},
		{"with exec option", VolumeMountSpec("/host:/container:ro,exec"), true, false},
		{"relative paths", VolumeMountSpec("./data:/data"), true, false},
		{"var home workspace path", VolumeMountSpec("/var/home/user/project:/workspace"), true, false},
		{"empty is invalid", VolumeMountSpec(""), false, true},
		{"no colon is invalid", VolumeMountSpec("/just-a-path"), false, true},
		{"relative container is invalid", VolumeMountSpec("/host:relative"), false, true},
		{"sensitive path is invalid", VolumeMountSpec("/etc/shadow:/data"), false, true},
		{"home ssh path is invalid", VolumeMountSpec("/home/user/.ssh:/data"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.spec.Validate()
			if (err == nil) != tt.want {
				t.Errorf("VolumeMountSpec(%q).Validate() error = %v, want valid=%v", tt.spec, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("VolumeMountSpec(%q).Validate() returned nil, want error", tt.spec)
				}
				if !errors.Is(err, ErrInvalidVolumeMountSpec) {
					t.Errorf("error should wrap ErrInvalidVolumeMountSpec, got: %v", err)
				}
				var vmErr *InvalidVolumeMountSpecError
				if !errors.As(err, &vmErr) {
					t.Errorf("error should be *InvalidVolumeMountSpecError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("VolumeMountSpec(%q).Validate() returned unexpected error: %v", tt.spec, err)
			}
		})
	}
}

// TestVolumeMountSpec_Validate_Matrix exercises the canonical eight-vector
// volume-mount matrix. v0.10.0 bug #4 was a Windows backslash host path
// embedded in a `host:container` spec — this matrix documents what the
// current validator does with each compound shape so a future refactor that
// changes any one vector is caught at CI rather than at release time.
func TestVolumeMountSpec_Validate_Matrix(t *testing.T) {
	t.Parallel()
	rejectInvalid := pathmatrix.RejectIs(ErrInvalidVolumeMountSpec)
	pathmatrix.VolumeMount(t, func(spec string) error {
		return VolumeMountSpec(spec).Validate()
	}, pathmatrix.VolumeMountVectors{
		// Standard Unix host + Unix container — accepted everywhere.
		UnixHostUnixContainer: pathmatrix.PassAny(nil),

		// Windows backslash host with a Unix container path. The current
		// validator parses on the FIRST colon, which puts everything before
		// `C:\host:` into the host portion — including the drive-letter
		// colon. This rejects the spec on every platform, which is the
		// desired behavior because a backslash host path needs to be
		// `filepath.ToSlash`-converted before being concatenated. v0.10.0
		// bug #4 was the unconverted form leaking through.
		WindowsBackslashHostUnix: rejectInvalid,

		// Windows forward-slash host: `C:/host:/container`. The current
		// validator accepts this — the FIRST-colon split yields host="C"
		// and container="/host:/container", and the validator treats both
		// portions as well-formed. This may be a latent bug class on
		// non-Windows hosts (host="C" is meaningless on Linux) but it's
		// the documented behavior; flag changes here for review.
		WindowsForwardSlashHostUnix: pathmatrix.PassAny(nil),

		// A named Docker volume bound at a Unix container path. The
		// validator accepts named volumes as the host portion — `myvol`
		// is a valid Docker volume name.
		NamedVolumeUnix: pathmatrix.PassAny(nil),

		// Relative host `./host:/container` is explicitly valid (matches
		// the "./data:/data" case in the literal test table above).
		RelativeHostUnix: pathmatrix.PassAny(nil),

		// Host path with a literal colon embedded — the FIRST-colon parser
		// splits at the drive-letter colon, leaving an unparseable
		// remainder. Rejected.
		HostWithColonInPath: rejectInvalid,

		// Malformed: missing host or missing container.
		EmptyHost:      rejectInvalid,
		EmptyContainer: rejectInvalid,
	})
}

func TestVolumeMountSpec_String(t *testing.T) {
	t.Parallel()
	v := VolumeMountSpec("/host:/container:ro")
	if v.String() != "/host:/container:ro" {
		t.Errorf("VolumeMountSpec.String() = %q, want %q", v.String(), "/host:/container:ro")
	}
}
