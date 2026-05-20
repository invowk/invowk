// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"time"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	invowkruntime "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
	"golang.org/x/term"
)

const (
	cmdWaitDelay = 10 * time.Second
)

type (
	dependencyHostProbe struct{}

	dependencyCapabilityChecker struct{}

	dependencyLockProvider struct{}
)

// NewDependencyHostProbe creates the production host probe for dependency checks.
func NewDependencyHostProbe() deps.HostProbe {
	return dependencyHostProbe{}
}

// NewDependencyScriptFileReader returns the production reader for module-contained script files.
func NewDependencyScriptFileReader() deps.ScriptFileReader {
	return os.ReadFile
}

// NewDependencyLockProvider creates the production lock provider for command scope checks.
func NewDependencyLockProvider() deps.CommandScopeLockProvider {
	return dependencyLockProvider{}
}

// NewDependencyCapabilityChecker creates the production capability checker for dependency checks.
func NewDependencyCapabilityChecker() deps.CapabilityChecker {
	return dependencyCapabilityChecker{}
}

// Validate returns nil because DependencyHostProbe is stateless.
func (dependencyHostProbe) Validate() error {
	return nil
}

// Validate returns nil because DependencyLockProvider is stateless.
func (dependencyLockProvider) Validate() error {
	return nil
}

// Validate returns nil because DependencyCapabilityChecker is stateless.
func (dependencyCapabilityChecker) Validate() error {
	return nil
}

// CheckTool validates a tool dependency against the host system PATH.
func (dependencyHostProbe) CheckTool(toolName invowkfile.BinaryName) error {
	_, err := exec.LookPath(string(toolName))
	if err != nil {
		return fmt.Errorf("%s - not found in PATH", toolName)
	}
	return nil
}

// CheckFilepath checks whether a host filepath exists and has the required permissions.
func (dependencyHostProbe) CheckFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error {
	resolvedPathStr := string(resolvedPath)

	info, err := os.Stat(resolvedPathStr)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", displayPath, deps.ErrPathNotExists)
	}
	if err != nil {
		return fmt.Errorf("%s: cannot access path: %w", displayPath, err)
	}

	var permErrors []string
	if fp.Readable && !isReadable(resolvedPathStr, info) {
		permErrors = append(permErrors, "read")
	}
	if fp.Writable && !isWritable(resolvedPathStr, info) {
		permErrors = append(permErrors, "write")
	}
	if fp.Executable && !isExecutable(resolvedPathStr, info) {
		permErrors = append(permErrors, "execute")
	}
	if len(permErrors) > 0 {
		return fmt.Errorf("%s: missing permissions: %s", displayPath, strings.Join(permErrors, ", "))
	}

	return nil
}

// RunCustomCheck runs a custom check script in the host dependency environment.
func (dependencyHostProbe) RunCustomCheck(ctx context.Context, check invowkfile.CustomCheck) (deps.CustomCheckResult, error) {
	result := runHostCustomCheck(ctx, check)
	outputText := deps.CustomCheckOutput(strings.TrimSpace(result.Output + result.ErrOutput))
	if validateErr := outputText.Validate(); validateErr != nil {
		return deps.CustomCheckResult{}, fmt.Errorf("custom check output: %w", validateErr)
	}
	if result.Error != nil {
		return deps.CustomCheckResult{}, fmt.Errorf(dependencyErrorFormat, check.Name, result.Error)
	}
	return deps.NewCustomCheckResult(outputText, result.ExitCode)
}

func runHostCustomCheck(ctx context.Context, check invowkfile.CustomCheck) *invowkruntime.Result {
	scriptText := string(check.Script.Content)
	interpInfo := check.Script.ResolveInterpreterFromScript(scriptText)
	if interpInfo.Found && !invowkfile.IsShellInterpreter(interpInfo.Interpreter) {
		return runHostCustomCheckNative(ctx, check.Script)
	}
	return runHostCustomCheckVirtual(ctx, check.Script)
}

func runHostCustomCheckNative(ctx context.Context, script invowkfile.CustomCheckScript) *invowkruntime.Result {
	rt := invowkruntime.NewNativeRuntime(invowkruntime.WithEnvBuilder(hostCustomCheckEnvBuilder()))
	return rt.ExecuteCapture(hostCustomCheckExecutionContext(ctx, script, invowkfile.RuntimeNative))
}

func runHostCustomCheckVirtual(ctx context.Context, script invowkfile.CustomCheckScript) *invowkruntime.Result {
	rt := invowkruntime.NewShRuntime(false, invowkruntime.WithShEnvBuilder(hostCustomCheckEnvBuilder()))
	return rt.ExecuteCapture(hostCustomCheckExecutionContext(ctx, script, invowkfile.RuntimeVirtualSh))
}

func hostCustomCheckExecutionContext(ctx context.Context, script invowkfile.CustomCheckScript, mode invowkfile.RuntimeMode) *invowkruntime.ExecutionContext {
	rtConfig := invowkfile.RuntimeConfig{Name: mode}
	if mode == invowkfile.RuntimeVirtualSh {
		rtConfig.AllowedBinaries = []invowkfile.AllowedBinary{"*"}
	}
	return &invowkruntime.ExecutionContext{
		Context:         ctx,
		Invowkfile:      &invowkfile.Invowkfile{FilePath: "."},
		Command:         &invowkfile.Command{},
		SelectedRuntime: mode,
		SelectedImpl: &invowkfile.Implementation{
			Script: invowkfile.ImplementationScript{
				Content:     script.Content,
				Interpreter: script.Interpreter,
			},
			Runtimes:  []invowkfile.RuntimeConfig{rtConfig},
			Platforms: invowkfile.AllPlatformConfigs(),
		},
	}
}

func hostCustomCheckEnvBuilder() invowkruntime.EnvBuilder {
	return &invowkruntime.MockEnvBuilder{Env: hostCustomCheckEnvMap()}
}

//goplint:ignore -- os/exec environment is represented as primitive string keys and values.
func hostCustomCheckEnvMap() map[string]string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			env[name] = value
		}
	}
	return env
}

// LoadCommandScopeLock loads lock-file state for command-scope validation.
func (dependencyLockProvider) LoadCommandScopeLock(inv *invowkfile.Invowkfile) (*invowkmod.LockFile, error) {
	if inv == nil || inv.ModulePath == "" {
		return &invowkmod.LockFile{}, nil
	}
	lockPath := filepath.Join(string(inv.ModulePath), invowkmod.LockFileName)
	typedPath := types.FilesystemPath(lockPath)
	if pathErr := typedPath.Validate(); pathErr != nil {
		return nil, &deps.CommandScopeLockError{
			Path: typedPath,
			Err:  pathErr,
		}
	}
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, &deps.CommandScopeLockError{
			Path: typedPath,
			Err:  err,
		}
	}
	return lock, nil
}

// Check validates that a system capability is available.
func (dependencyCapabilityChecker) Check(ctx context.Context, ioCtx deps.IOContext, capability invowkfile.CapabilityName) error {
	if ctx == nil {
		ctx = context.Background()
	}

	switch capability {
	case invowkfile.CapabilityLocalAreaNetwork:
		return checkLocalAreaNetwork()
	case invowkfile.CapabilityInternet:
		return checkInternet(ctx)
	case invowkfile.CapabilityContainers:
		return checkContainers(ctx)
	case invowkfile.CapabilityTTY:
		return checkTTY(ioCtx)
	default:
		return &invowkfile.CapabilityError{
			Capability: capability,
			Message:    "unknown capability",
		}
	}
}

// checkLocalAreaNetwork checks for non-loopback, routable network interfaces.
func checkLocalAreaNetwork() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityLocalAreaNetwork,
			Message:    fmt.Sprintf("failed to list network interfaces: %v", err),
		}
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			return nil
		}
	}

	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityLocalAreaNetwork,
		Message:    "no active network interface with routable IP address found",
	}
}

// checkInternet checks for working internet connectivity with lightweight DNS probes.
func checkInternet(parentCtx context.Context) error {
	if checkLocalAreaNetwork() != nil {
		return &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityInternet,
			Message:    "no local network available",
		}
	}

	dnsServers := []string{
		"8.8.8.8:53",
		"1.1.1.1:53",
		"208.67.222.222:53",
	}

	dialer := &net.Dialer{Timeout: invowkfile.DefaultCapabilityTimeout}
	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(parentCtx, invowkfile.DefaultCapabilityTimeout)
	defer cancel()

	var lastErr error
	for _, server := range dnsServers {
		conn, err := dialer.DialContext(ctx, "udp", server)
		if err != nil {
			lastErr = err
			continue
		}
		_ = conn.Close()

		if _, err := resolver.LookupHost(ctx, "dns.google"); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	msg := "unable to reach internet DNS servers"
	if lastErr != nil {
		msg = fmt.Sprintf("unable to reach internet: %v", lastErr)
	}
	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityInternet,
		Message:    msg,
	}
}

// checkContainers checks if Docker or Podman is available and ready.
func checkContainers(parentCtx context.Context) error {
	foundEngine := false
	var lastErr error
	for _, engine := range []config.ContainerEngine{config.ContainerEnginePodman, config.ContainerEngineDocker} {
		path, err := exec.LookPath(string(engine))
		if err != nil {
			continue
		}
		foundEngine = true

		ctx, cancel := context.WithTimeout(parentCtx, invowkfile.DefaultCapabilityTimeout)
		cmd := exec.CommandContext(ctx, path, containerCapabilityProbeArgs(engine)...)
		cmd.WaitDelay = cmdWaitDelay
		err = cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if !foundEngine {
		return &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityContainers,
			Message:    "no container engine (podman or docker) found in PATH",
		}
	}

	msg := "container engine is not ready"
	if lastErr != nil {
		msg = fmt.Sprintf("container engine is not ready: %v", lastErr)
	}
	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityContainers,
		Message:    msg,
	}
}

//goplint:ignore -- exec.CommandContext requires argv as primitive strings.
func containerCapabilityProbeArgs(engine config.ContainerEngine) []string {
	switch engine {
	case config.ContainerEnginePodman:
		return []string{"version", "--format", "{{.Version}}"}
	case config.ContainerEngineDocker:
		return []string{"version", "--format", "{{.Server.Version}}"}
	default:
		return []string{"version"}
	}
}

// checkTTY checks whether invowk is running in an interactive terminal.
func checkTTY(ioCtx deps.IOContext) error {
	stdin, stdinOK := ioCtx.Stdin.(*os.File)
	stdout, stdoutOK := ioCtx.Stdout.(*os.File)
	if stdinOK && stdoutOK && term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd())) {
		return nil
	}

	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityTTY,
		Message:    "not running in an interactive TTY (stdin/stdout)",
	}
}

//goplint:ignore -- adapter probes OS-native path strings.
func isReadable(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return canOpenPath(path)
	}
	return canOpenReadOnly(path)
}

//goplint:ignore -- adapter probes OS-native path strings.
func isWritable(path string, info os.FileInfo) bool {
	if info.IsDir() {
		f, err := os.CreateTemp(path, ".invowk-wcheck-*")
		if err != nil {
			return false
		}
		tmpName := f.Name()
		defer func() { _ = os.Remove(tmpName) }()
		_ = f.Close()
		return true
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

//goplint:ignore -- adapter probes OS-native path strings.
func isExecutable(path string, info os.FileInfo) bool {
	if goruntime.GOOS == platform.Windows {
		return isExecutableOnWindows(path, info)
	}
	return info.Mode()&0o111 != 0
}

//goplint:ignore -- adapter probes OS-native path strings.
func isExecutableOnWindows(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return canOpenPath(path)
	}
	if !windowsPathHasExecutableExtension(path) {
		return false
	}
	return canOpenReadOnly(path)
}

//goplint:ignore -- adapter probes OS-native path strings.
func windowsPathHasExecutableExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	execExts := []string{".exe", ".cmd", ".bat", ".com", ".ps1"}
	if slices.Contains(execExts, ext) {
		return true
	}

	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		return false
	}

	for pathExt := range strings.SplitSeq(strings.ToLower(pathext), ";") {
		if pathExt != "" && pathExt == ext {
			return true
		}
	}
	return false
}

//goplint:ignore -- adapter probes OS-native path strings.
func canOpenPath(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

//goplint:ignore -- adapter probes OS-native path strings.
func canOpenReadOnly(path string) bool {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
