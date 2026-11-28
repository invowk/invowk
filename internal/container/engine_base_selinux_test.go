// SPDX-License-Identifier: MPL-2.0

package container

import (
	"slices"
	"testing"
)

func TestBaseCLIEngine_SELinuxModesAndDisableLabeling(t *testing.T) {
	t.Parallel()

	t.Run("podman active SELinux applies auto shared private none", func(t *testing.T) {
		t.Parallel()

		engine := NewBaseCLIEngine(
			"podman",
			WithVolumeFormatter(makeSELinuxLabelAdder(func() bool { return true })),
		)

		autoVol := VolumeMountSpec("/host/auto:/auto")
		sharedVol := VolumeMountSpec("/host/shared:/workspace:ro")
		privateVol := VolumeMountSpec("/host/private:/workspace")
		noneVol := VolumeMountSpec("/host/none:/workspace:ro")
		tmpVol := VolumeMountSpec("/tmp/invowk-script:/.invowk/tmp:ro")

		args := engine.RunArgs(RunOptions{
			Image: "debian:stable-slim",
			Volumes: []VolumeMountSpec{
				autoVol,
				sharedVol,
				privateVol,
				noneVol,
				tmpVol,
			},
			VolumeSELinuxModes: VolumeSELinuxModes{
				sharedVol:  VolumeMountSELinuxShared,
				privateVol: VolumeMountSELinuxPrivate,
				noneVol:    VolumeMountSELinuxNone,
			},
		})

		wantVols := []string{
			"/host/auto:/auto:z",
			"/host/shared:/workspace:ro,z",
			"/host/private:/workspace:Z",
			"/host/none:/workspace:ro",
			"/tmp/invowk-script:/.invowk/tmp:ro,z",
		}
		for _, want := range wantVols {
			if !slices.Contains(args, want) {
				t.Errorf("RunArgs() missing %q in %v", want, args)
			}
		}
		if slices.Contains(args, "label=disable") {
			t.Errorf("RunArgs() unexpectedly emitted label=disable: %v", args)
		}
	})

	t.Run("operator disableSELinuxLabeling emits label=disable and suppresses :z", func(t *testing.T) {
		t.Parallel()

		engine := NewBaseCLIEngine(
			"podman",
			WithVolumeFormatter(makeSELinuxLabelAdder(func() bool { return true })),
			WithRunArgsTransformer(makeUsernsKeepIDAdder()),
		)

		wsVol := VolumeMountSpec("/host/ws:/workspace")
		tmpVol := VolumeMountSpec("/tmp/invowk-script:/.invowk/tmp:ro")
		opts := RunOptions{
			Image: "debian:stable-slim",
			Volumes: []VolumeMountSpec{
				wsVol,
				tmpVol,
			},
			VolumeSELinuxModes: VolumeSELinuxModes{
				wsVol: VolumeMountSELinuxShared,
			},
			DisableSELinuxLabeling: true,
		}
		args := engine.RunArgs(opts)

		if !slices.Contains(args, "--security-opt") || !slices.Contains(args, "label=disable") {
			t.Fatalf("RunArgs() missing --security-opt label=disable: %v", args)
		}
		if !slices.Contains(args, "/host/ws:/workspace") {
			t.Errorf("RunArgs() expected unlabeled workspace mount, got %v", args)
		}
		if !slices.Contains(args, "/tmp/invowk-script:/.invowk/tmp:ro") {
			t.Errorf("RunArgs() expected unlabeled tmp mount, got %v", args)
		}
		// Ensure --userns=keep-id is inserted BEFORE image, not treating "label=disable" as the image name.
		imgIdx := slices.Index(args, "debian:stable-slim")
		keepIDIdx := slices.Index(args, "--userns=keep-id")
		if keepIDIdx < 0 || imgIdx < 0 || keepIDIdx > imgIdx {
			t.Errorf("expected --userns=keep-id before image name in %v", args)
		}

		createArgs := engine.CreateArgs(CreateOptions{
			Image:                  "debian:stable-slim",
			Command:                []string{"sleep", "infinity"},
			Name:                   "persist-selinux-test",
			Volumes:                []VolumeMountSpec{wsVol},
			VolumeSELinuxModes:     VolumeSELinuxModes{wsVol: VolumeMountSELinuxPrivate},
			DisableSELinuxLabeling: true,
		})
		if !slices.Contains(createArgs, "--security-opt") || !slices.Contains(createArgs, "label=disable") {
			t.Fatalf("CreateArgs() missing --security-opt label=disable: %v", createArgs)
		}
		if !slices.Contains(createArgs, "/host/ws:/workspace") {
			t.Errorf("CreateArgs() expected unlabeled workspace mount, got %v", createArgs)
		}
	})

	t.Run("appendSELinuxVolumeOption preserves Windows drive letters", func(t *testing.T) {
		t.Parallel()

		if got := appendSELinuxVolumeOption(`C:\work:/workspace`, "z"); got != `C:\work:/workspace:z` {
			t.Errorf("got %q, want C:\\work:/workspace:z", got)
		}
		if got := appendSELinuxVolumeOption(`C:/work:/workspace:ro`, "Z"); got != `C:/work:/workspace:ro,Z` {
			t.Errorf("got %q, want C:/work:/workspace:ro,Z", got)
		}
		if got := appendSELinuxVolumeOption(`/host:/workspace:z`, "Z"); got != `/host:/workspace:z` {
			t.Errorf("got %q, want unchanged /host:/workspace:z", got)
		}
	})
}

func TestVolumeSELinuxModes_Validate(t *testing.T) {
	t.Parallel()

	valid := VolumeSELinuxModes{
		"/a:/b": VolumeMountSELinuxAuto,
		"/c:/d": VolumeMountSELinuxShared,
		"/e:/f": VolumeMountSELinuxPrivate,
		"/g:/h": VolumeMountSELinuxNone,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid.Validate() = %v, want nil", err)
	}

	invalid := VolumeSELinuxModes{
		"/a:/b": VolumeMountSELinuxMode("bogus"),
	}
	if err := invalid.Validate(); err == nil {
		t.Fatal("invalid.Validate() = nil, want error")
	}
}
