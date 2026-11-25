// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// contextRecordingEngine records the Dockerfile and whether the invowk
	// binary was in the build context, before the provisioner removes it.
	contextRecordingEngine struct {
		*mockEngine
		dockerfiles []string
		hadBinary   []bool
	}

	// queryingEngine answers the image architecture query like a real engine.
	queryingEngine struct {
		*mockEngine
		mu      sync.Mutex
		answer  string
		queries []string
	}

	// resolutionHost is the simulated host and engine report of a test.
	resolutionHost struct {
		linux    bool
		arch     container.ImageArchitecture
		reported *container.ImageArchitecture
	}
)

func (e *contextRecordingEngine) Build(ctx context.Context, opts container.BuildOptions) error {
	data, err := os.ReadFile(filepath.Join(string(opts.ContextDir), string(opts.Dockerfile)))
	if err != nil {
		return err
	}
	e.dockerfiles = append(e.dockerfiles, string(data))
	_, statErr := os.Stat(filepath.Join(string(opts.ContextDir), "invowk"))
	e.hadBinary = append(e.hadBinary, statErr == nil)
	return e.mockEngine.Build(ctx, opts)
}

func (e *queryingEngine) RunQuery(_ context.Context, args ...string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.queries = append(e.queries, strings.Join(args, " "))
	return []byte(e.answer), nil
}

func reportedArch(arch container.ImageArchitecture) *container.ImageArchitecture { return new(arch) }

// linuxAMD64NoReport is an amd64 Linux host whose engine does not have the image.
func linuxAMD64NoReport() resolutionHost {
	return resolutionHost{linux: true, arch: "amd64", reported: reportedArch(container.ImageArchitectureUnknown)}
}

func newResolutionProvisioner(t *testing.T, engine imageBuilder, cfg *Config, host resolutionHost) *LayerProvisioner {
	t.Helper()
	if cfg.CacheDir == "" {
		cfg.CacheDir = types.FilesystemPath(t.TempDir())
	}
	if cfg.BinaryMountPath == "" {
		cfg.BinaryMountPath = "/invowk/bin"
	}
	if cfg.ModulesMountPath == "" {
		cfg.ModulesMountPath = defaultModulesMountPath
	}
	cfg.Enabled = true
	p, err := NewLayerProvisioner(engine, cfg)
	if err != nil {
		t.Fatalf("NewLayerProvisioner() error = %v", err)
	}
	p.hostLinux, p.hostArch = host.linux, host.arch
	if host.reported != nil {
		p.imageArchitecture = func(context.Context, container.ImageTag) container.ImageArchitecture { return *host.reported }
	}
	return p
}

func provisionBase(t *testing.T, p *LayerProvisioner) (*Result, error) {
	t.Helper()
	return p.Provision(t.Context(), Request{BaseImage: testProvisionBaseImage})
}

func assertSkipWarning(t *testing.T, result *Result, wantParts ...string) {
	t.Helper()
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %+v, want one skip warning", result.Warnings)
	}
	msg := result.Warnings[0].Message.String()
	for _, part := range append([]string{"container.auto_provision.binary_path", "Nested invowk commands"}, wantParts...) {
		if !strings.Contains(msg, part) {
			t.Errorf("warning %q does not contain %q", msg, part)
		}
	}
}

func TestProvisionSkipsUnusableDefaultBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := &contextRecordingEngine{mockEngine: newMockEngine()}
	cfg := &Config{DefaultInvowkBinaryPath: types.FilesystemPath(testutil.WriteMachOFixture(t, filepath.Join(dir, "invowk")))}
	p := newResolutionProvisioner(t, engine, cfg, resolutionHost{linux: false, arch: "arm64", reported: reportedArch(container.ImageArchitectureUnknown)})

	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if len(engine.buildCalls) != 1 {
		t.Fatalf("Build calls = %d, want 1", len(engine.buildCalls))
	}
	if engine.hadBinary[0] {
		t.Error("build context contains the invowk binary, want it skipped")
	}
	dockerfile := engine.dockerfiles[0]
	if strings.Contains(dockerfile, "COPY invowk") || strings.Contains(dockerfile, "ENV PATH") {
		t.Errorf("Dockerfile installs the skipped binary:\n%s", dockerfile)
	}
	if !strings.Contains(dockerfile, "COPY modules/") {
		t.Errorf("Dockerfile lost the modules layer:\n%s", dockerfile)
	}
	if _, ok := result.EnvVars["PATH"]; ok {
		t.Errorf("EnvVars PATH = %q, want no PATH for a skipped binary", result.EnvVars["PATH"])
	}
	assertSkipWarning(t, result, "Mach-O", "for the image's architecture")

	// A later run that reuses the cached image warns again.
	engine.imageExistsResult = true
	cached, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision(cache hit) error = %v", err)
	}
	if len(engine.buildCalls) != 1 {
		t.Fatalf("Build calls after cache hit = %d, want 1", len(engine.buildCalls))
	}
	if cached.ImageTag != result.ImageTag {
		t.Errorf("cache hit tag = %q, want %q", cached.ImageTag, result.ImageTag)
	}
	assertSkipWarning(t, cached, "Mach-O")
}

func TestProvisionSkipsDefaultBinaryForReportedArchitecture(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := &contextRecordingEngine{mockEngine: newMockEngine()}
	cfg := &Config{DefaultInvowkBinaryPath: types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk"), "amd64", nil))}
	p := newResolutionProvisioner(t, engine, cfg, resolutionHost{linux: true, arch: "amd64", reported: reportedArch("arm64")})

	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if engine.hadBinary[0] {
		t.Error("build context contains the amd64 binary for an arm64 image")
	}
	assertSkipWarning(t, result, "built for amd64, container is arm64", "a Linux arm64 build of invowk")
}

func TestProvisionInstallsUsableDefaultBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := &contextRecordingEngine{mockEngine: newMockEngine()}
	cfg := &Config{DefaultInvowkBinaryPath: types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk"), "amd64", nil))}
	p := newResolutionProvisioner(t, engine, cfg, linuxAMD64NoReport())

	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if !engine.hadBinary[0] || !strings.Contains(engine.dockerfiles[0], "COPY invowk /invowk/bin/invowk") ||
		!strings.Contains(engine.dockerfiles[0], `ENV PATH="/invowk/bin:$PATH"`) {
		t.Errorf("binary not installed; context had binary = %v, Dockerfile:\n%s", engine.hadBinary[0], engine.dockerfiles[0])
	}
	if !strings.HasPrefix(result.EnvVars["PATH"], "/invowk/bin:") {
		t.Errorf("EnvVars PATH = %q, want the binary directory first", result.EnvVars["PATH"])
	}
	if len(result.Warnings) != 0 {
		t.Errorf("Warnings = %+v, want none", result.Warnings)
	}
}

func TestProvisionRejectsUnusableConfiguredBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tests := []struct {
		name       string
		path       string
		host       resolutionHost
		wantKind   BinaryRejectionKind
		wantInText []string
	}{
		{
			name:       "Mach-O",
			path:       testutil.WriteMachOFixture(t, filepath.Join(dir, "macho")),
			host:       linuxAMD64NoReport(),
			wantKind:   BinaryRejectionNotELF,
			wantInText: []string{"not a Linux executable", "Mach-O"},
		},
		{
			name:       "PE on a non-Linux host",
			path:       testutil.WritePEFixture(t, filepath.Join(dir, "pe")),
			host:       resolutionHost{linux: false, arch: "amd64", reported: reportedArch(container.ImageArchitectureUnknown)},
			wantKind:   BinaryRejectionNotELF,
			wantInText: []string{"not a Linux executable", "PE"},
		},
		{
			name:       "missing file",
			path:       filepath.Join(dir, "missing"),
			host:       linuxAMD64NoReport(),
			wantKind:   BinaryRejectionUnreadable,
			wantInText: []string{"cannot read the file"},
		},
		{
			name:       "reported architecture mismatch",
			path:       testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "amd64"), "amd64", nil),
			host:       resolutionHost{linux: true, arch: "amd64", reported: reportedArch("arm64")},
			wantKind:   BinaryRejectionMachine,
			wantInText: []string{"built for amd64, container is arm64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine := newMockEngine()
			cfg := &Config{
				InvowkBinaryPath:        types.FilesystemPath(tt.path),
				DefaultInvowkBinaryPath: writeHostLinuxBinary(t, filepath.Join(t.TempDir(), "default"), nil),
				Strict:                  false,
			}
			p := newResolutionProvisioner(t, engine, cfg, tt.host)

			_, err := provisionBase(t, p)
			if !errors.Is(err, ErrUnusableInvowkBinary) {
				t.Fatalf("Provision() error = %v, want ErrUnusableInvowkBinary", err)
			}
			var unusable *UnusableInvowkBinaryError
			if !errors.As(err, &unusable) {
				t.Fatalf("Provision() error = %T, want *UnusableInvowkBinaryError", err)
			}
			if string(unusable.Path) != tt.path || unusable.Reason.Kind != tt.wantKind || unusable.ArchitectureAssumed {
				t.Errorf("error = %+v, want path %q, kind %s, not assumed", unusable, tt.path, tt.wantKind)
			}
			for _, part := range append([]string{"container.auto_provision.binary_path", tt.path}, tt.wantInText...) {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q does not contain %q", err, part)
				}
			}
			if len(engine.buildCalls) != 0 || len(engine.imageExistsCalls) != 0 {
				t.Errorf("Build calls = %d, ImageExists calls = %d, want none", len(engine.buildCalls), len(engine.imageExistsCalls))
			}
		})
	}
}

func TestProvisionConfiguredBinaryAgainstAssumedArchitecture(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := &contextRecordingEngine{mockEngine: newMockEngine()}
	arm64 := testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk-arm64"), "arm64", nil)
	host := linuxAMD64NoReport()
	p := newResolutionProvisioner(t, engine, &Config{InvowkBinaryPath: types.FilesystemPath(arm64)}, host)

	_, err := provisionBase(t, p)
	var unusable *UnusableInvowkBinaryError
	if !errors.As(err, &unusable) || !unusable.ArchitectureAssumed {
		t.Fatalf("Provision() error = %v, want an assumed-architecture UnusableInvowkBinaryError", err)
	}
	for _, part := range []string{
		arm64, "built for arm64", "did not report the architecture of image debian:stable-slim",
		"assumed the host architecture amd64", "pull debian:stable-slim", "build its Containerfile", "Linux amd64 build",
	} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not contain %q", err, part)
		}
	}
	if len(engine.buildCalls) != 0 {
		t.Fatalf("Build calls = %d, want none", len(engine.buildCalls))
	}

	// After the user pulls the image, the engine reports arm64.
	*host.reported = "arm64"
	if _, err := provisionBase(t, p); err != nil {
		t.Fatalf("Provision() after the pull error = %v", err)
	}
	if len(engine.hadBinary) != 1 || !engine.hadBinary[0] {
		t.Fatalf("arm64 binary not installed after the engine reported arm64 (contexts: %v)", engine.hadBinary)
	}
}

func TestProvisionAcceptsConfiguredBinaryWithoutHostFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	arm64 := types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk-arm64"), "arm64", nil))

	tests := []struct {
		name string
		host resolutionHost
	}{
		// mips64le is outside the table: no machine comparison, and amd64
		// is not assumed in its place.
		{name: "reported mips64le on an amd64 Linux host", host: resolutionHost{linux: true, arch: "amd64", reported: reportedArch("mips64le")}},
		{name: "unknown image on a macOS host", host: resolutionHost{linux: false, arch: "arm64", reported: reportedArch(container.ImageArchitectureUnknown)}},
		{name: "unknown image on a Windows amd64 host", host: resolutionHost{linux: false, arch: "amd64", reported: reportedArch(container.ImageArchitectureUnknown)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine := &contextRecordingEngine{mockEngine: newMockEngine()}
			p := newResolutionProvisioner(t, engine, &Config{InvowkBinaryPath: arm64}, tt.host)
			result, err := provisionBase(t, p)
			if err != nil {
				t.Fatalf("Provision() error = %v", err)
			}
			if !engine.hadBinary[0] || len(result.Warnings) != 0 {
				t.Errorf("binary installed = %v, warnings = %+v; want installed without warnings", engine.hadBinary[0], result.Warnings)
			}
		})
	}
}

func TestGetProvisionedImageTagNeverAssumesHostArchitecture(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	arm64 := types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk-arm64"), "arm64", nil))

	withBinary := newResolutionProvisioner(t, newMockEngine(), &Config{InvowkBinaryPath: arm64}, linuxAMD64NoReport())
	tag, err := withBinary.GetProvisionedImageTag(t.Context(), testProvisionBaseImage)
	if err != nil {
		t.Fatalf("GetProvisionedImageTag(arm64 binary, no report) error = %v, want none", err)
	}
	withoutBinary := newResolutionProvisioner(t, newMockEngine(), &Config{}, linuxAMD64NoReport())
	bareTag, err := withoutBinary.GetProvisionedImageTag(t.Context(), testProvisionBaseImage)
	if err != nil {
		t.Fatalf("GetProvisionedImageTag(no binary) error = %v", err)
	}
	if tag == bareTag {
		t.Errorf("tag with the arm64 binary = tag without a binary (%q), want the binary keyed", tag)
	}

	machO := types.FilesystemPath(testutil.WriteMachOFixture(t, filepath.Join(dir, "invowk-macho")))
	rejecting := newResolutionProvisioner(t, newMockEngine(), &Config{InvowkBinaryPath: machO}, linuxAMD64NoReport())
	if _, err := rejecting.GetProvisionedImageTag(t.Context(), testProvisionBaseImage); !errors.Is(err, ErrUnusableInvowkBinary) {
		t.Fatalf("GetProvisionedImageTag(Mach-O) error = %v, want ErrUnusableInvowkBinary", err)
	}
	if _, err := rejecting.IsImageProvisioned(t.Context(), testProvisionBaseImage); !errors.Is(err, ErrUnusableInvowkBinary) {
		t.Fatalf("IsImageProvisioned(Mach-O) error = %v, want ErrUnusableInvowkBinary", err)
	}
}

func TestProvisionedTagsReflectWhetherTheBinaryIsInstalled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	amd64 := types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk"), "amd64", nil))
	machO := types.FilesystemPath(testutil.WriteMachOFixture(t, filepath.Join(dir, "invowk-macho")))
	cacheDir := types.FilesystemPath(t.TempDir())
	tagFor := func(cfg *Config, host resolutionHost) string {
		t.Helper()
		cfg.CacheDir = cacheDir
		tag, err := newResolutionProvisioner(t, newMockEngine(), cfg, host).GetProvisionedImageTag(t.Context(), testProvisionBaseImage)
		if err != nil {
			t.Fatalf("GetProvisionedImageTag() error = %v", err)
		}
		return tag
	}
	macOSHost := resolutionHost{linux: false, arch: "arm64", reported: reportedArch(container.ImageArchitectureUnknown)}

	skipped := tagFor(&Config{DefaultInvowkBinaryPath: machO}, macOSHost)
	none := tagFor(&Config{}, macOSHost)
	configured := tagFor(&Config{InvowkBinaryPath: amd64, DefaultInvowkBinaryPath: machO}, macOSHost)
	if skipped != none {
		t.Errorf("skipped-default tag %q != no-binary tag %q, want the same image", skipped, none)
	}
	if configured == skipped {
		t.Errorf("tag with binary_path set = skipped-default tag %q, want a new image", configured)
	}
}

func TestGetProvisionedImageTagMatchesBuiltTagWhenBaseIsReported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := &contextRecordingEngine{mockEngine: newMockEngine()}
	cfg := &Config{DefaultInvowkBinaryPath: types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk"), "amd64", nil))}
	p := newResolutionProvisioner(t, engine, cfg, resolutionHost{linux: true, arch: "amd64", reported: reportedArch("arm64")})

	identity, err := p.GetProvisionedImageTag(t.Context(), testProvisionBaseImage)
	if err != nil {
		t.Fatalf("GetProvisionedImageTag() error = %v", err)
	}
	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if string(result.ImageTag) != identity {
		t.Errorf("Provision() tag = %q, GetProvisionedImageTag() = %q, want equal", result.ImageTag, identity)
	}
	if engine.hadBinary[0] {
		t.Error("default amd64 binary installed into an arm64 image")
	}
}

func TestProvisionQueriesEngineForImageArchitecture(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := &queryingEngine{mockEngine: newMockEngine(), answer: "arm64\n"}
	cfg := &Config{
		DefaultInvowkBinaryPath: types.FilesystemPath(testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk"), "amd64", nil)),
		CacheDir:                types.FilesystemPath(t.TempDir()),
		Enabled:                 true,
		BinaryMountPath:         "/invowk/bin",
		ModulesMountPath:        defaultModulesMountPath,
	}
	p, err := NewLayerProvisioner(engine, cfg)
	if err != nil {
		t.Fatalf("NewLayerProvisioner() error = %v", err)
	}
	p.hostLinux, p.hostArch = true, "amd64"

	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	assertSkipWarning(t, result, "container is arm64")
	want := "image inspect --format {{.Architecture}} " + string(testProvisionBaseImage)
	if len(engine.queries) != 1 || engine.queries[0] != want {
		t.Errorf("queries = %q, want [%q]", engine.queries, want)
	}

	// No candidate binary: no query at all.
	engine.queries = nil
	p.config.DefaultInvowkBinaryPath = ""
	if _, err := provisionBase(t, p); err != nil {
		t.Fatalf("Provision(no binary) error = %v", err)
	}
	if len(engine.queries) != 0 {
		t.Errorf("queries without a candidate binary = %q, want none", engine.queries)
	}
}
