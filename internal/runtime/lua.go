// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/arnodel/golua/lib"
	"github.com/arnodel/golua/lib/base"
	"github.com/arnodel/golua/lib/coroutine"
	"github.com/arnodel/golua/lib/mathlib"
	"github.com/arnodel/golua/lib/packagelib"
	"github.com/arnodel/golua/lib/stringlib"
	"github.com/arnodel/golua/lib/tablelib"
	"github.com/arnodel/golua/lib/utf8lib"
	luart "github.com/arnodel/golua/runtime"

	"github.com/invowk/invowk/internal/uroot"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const luaBinaryRequiredMsg = "binary name is required"

var luaLibraryLoadMu sync.Mutex

type (
	// LuaRuntime executes commands using the embedded golua interpreter.
	LuaRuntime struct {
		//plint:internal -- required constructor param; immutable after construction
		utilitiesEnabled bool
		//plint:internal -- test-injected registry uses Lua-specific constructor wiring; shared WithUrootRegistry belongs to ShRuntime
		urootRegistry *uroot.Registry
		//plint:internal -- field has WithLuaEnvBuilder(); field name doesn't match pattern
		envBuilder EnvBuilder
		//plint:internal -- field uses WithLuaInteractiveCommandFactory to avoid colliding with ShRuntime option naming
		interactiveCommandFactory LuaInteractiveCommandFactory
	}

	// LuaRuntimeOption configures a LuaRuntime.
	LuaRuntimeOption func(*LuaRuntime)

	//goplint:ignore -- internal Lua execution DTO carries already-resolved script/env/argv values through the VM bridge.
	luaExecutionRequest struct {
		script         string
		runtimeCfg     *invowkfile.RuntimeConfig
		env            map[string]string
		policy         *virtualHostBinaryPolicy
		pathResolver   virtualPathResolver
		pathValidator  virtualPathValidator
		workDir        string
		scriptBasePath string
		args           []string
		stdin          io.Reader
		stdout         io.Writer
		stderr         io.Writer
	}

	//goplint:ignore -- internal Lua bridge DTO groups VM dependencies to keep bridge setup readable.
	luaBridgeInstallRequest struct {
		policy           *virtualHostBinaryPolicy
		registry         *uroot.Registry
		pathResolver     virtualPathResolver
		pathValidator    virtualPathValidator
		env              map[string]string
		workDir          string
		scriptBasePath   string
		stdin            io.Reader
		stdout           io.Writer
		stderr           io.Writer
		utilitiesEnabled bool
	}

	//goplint:ignore -- internal Lua command bridge DTO groups execution dependencies for closures installed in golua tables.
	luaCommandBridgeConfig struct {
		policy           *virtualHostBinaryPolicy
		registry         *uroot.Registry
		pathValidator    virtualPathValidator
		env              map[string]string
		workDir          string
		stdin            io.Reader
		stdout           io.Writer
		stderr           io.Writer
		state            *luart.Table
		utilitiesEnabled bool
	}

	luaCommandBridge struct {
		config  luaCommandBridgeConfig
		capture bool
	}

	//goplint:ignore -- internal utility bridge DTO carries already-resolved execution adapters for u-root invocation.
	virtualUtilityRunRequest struct {
		//goplint:ignore -- u-root command invocation consumes raw argv strings at the runtime boundary.
		args          []string
		pathValidator virtualPathValidator
		//goplint:ignore -- runtime environment map is the already-built process environment for the command.
		env map[string]string
		//goplint:ignore -- workDir is an OS-native execution directory supplied by the execution context.
		workDir string
		stdin   io.Reader
		stdout  io.Writer
		stderr  io.Writer
	}
)

// WithLuaEnvBuilder sets the environment builder for the Lua runtime.
func WithLuaEnvBuilder(b EnvBuilder) LuaRuntimeOption {
	return func(r *LuaRuntime) {
		r.envBuilder = b
	}
}

// NewLuaRuntime creates a virtual-lua runtime.
func NewLuaRuntime(utilitiesEnabled bool, opts ...LuaRuntimeOption) *LuaRuntime {
	r := &LuaRuntime{
		utilitiesEnabled: utilitiesEnabled,
		envBuilder:       NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.utilitiesEnabled && r.urootRegistry == nil {
		r.urootRegistry = uroot.BuildDefaultRegistry()
	}
	return r
}

// Name returns the runtime name.
func (r *LuaRuntime) Name() string { return RuntimeTypeVirtualLua.String() }

// Available returns whether this runtime is available.
func (r *LuaRuntime) Available() bool { return true }

// Validate checks whether the selected implementation can run under virtual-lua.
func (r *LuaRuntime) Validate(ctx *ExecutionContext) error {
	if ctx.SelectedImpl == nil {
		return errVirtualNoImpl
	}
	if err := ctx.SelectedImpl.Script.Validate(); err != nil {
		return errVirtualNoScript
	}
	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return err
	}
	if interpErr := validateLuaInterpreter(ctx.SelectedImpl.Script, script); interpErr != nil {
		return interpErr
	}
	_, err = compileLuaChunk(script)
	return err
}

// Execute runs a command using the Lua runtime.
func (r *LuaRuntime) Execute(ctx *ExecutionContext) *Result {
	return r.execute(ctx, ctx.IO.Stdout, ctx.IO.Stderr)
}

// ExecuteCapture runs a command using the Lua runtime and captures stdout.
func (r *LuaRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := r.execute(ctx, &stdout, &stderr)
	result.Output = stdout.String()
	result.ErrOutput = stderr.String()
	return result
}

func (r *LuaRuntime) execute(ctx *ExecutionContext, stdout, stderr io.Writer) *Result {
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return NewErrorResult(1, err)
	}
	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return NewErrorResult(1, err)
	}
	if interpErr := validateLuaInterpreter(ctx.SelectedImpl.Script, script); interpErr != nil {
		return NewErrorResult(1, interpErr)
	}
	env, err := r.envBuilder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		return NewErrorResult(1, fmt.Errorf(failedBuildEnvironmentFmt, err))
	}
	pathResolver, err := newVirtualPathResolver(ctx)
	if err != nil {
		return NewErrorResult(1, err)
	}
	pathValidator := virtualPathValidator{resolver: pathResolver}
	addVirtualRuntimeEnv(env, pathResolver)
	ctx.AddTUIEnv(env)
	return r.executeScript(ctx.Context, luaExecutionRequest{
		script:         script,
		runtimeCfg:     selectedRuntimeConfig(ctx),
		env:            env,
		policy:         hostBinaryPolicy(ctx, env),
		pathResolver:   pathResolver,
		pathValidator:  pathValidator,
		workDir:        ctx.EffectiveWorkDir(),
		scriptBasePath: string(ctx.Invowkfile.GetScriptBasePath()),
		args:           ctx.PositionalArgs,
		stdin:          ctx.IO.Stdin,
		stdout:         stdout,
		stderr:         stderr,
	})
}

func (r *LuaRuntime) executeScript(ctx context.Context, req luaExecutionRequest) *Result {
	luaCtx, err := luaContextDef(req.runtimeCfg)
	if err != nil {
		return NewErrorResult(1, err)
	}
	if req.stdout == nil {
		req.stdout = io.Discard
	}
	if req.stderr == nil {
		req.stderr = io.Discard
	}

	luaRT := luart.New(req.stdout)
	cleanup := loadSafeLuaLibs(luaRT)
	defer cleanup()
	installInvowkLuaBridge(
		ctx,
		luaRT,
		luaBridgeInstallRequest{
			policy:           req.policy,
			registry:         r.urootRegistry,
			pathResolver:     req.pathResolver,
			pathValidator:    req.pathValidator,
			env:              req.env,
			workDir:          req.workDir,
			scriptBasePath:   req.scriptBasePath,
			stdin:            req.stdin,
			stdout:           req.stdout,
			stderr:           req.stderr,
			utilitiesEnabled: r.utilitiesEnabled,
		},
	)
	luaRT.SetEnv(luaRT.GlobalEnv(), "arg", luaArgsTable(req.args))
	argValues := luaArgValues(req.args)
	chunk, err := luaRT.CompileAndLoadLuaChunk("script", []byte(req.script), luart.TableValue(luaRT.GlobalEnv()))
	if err != nil {
		return NewErrorResult(1, fmt.Errorf("compile lua script: %w", err))
	}

	execCtx := ctx
	if execCtx == nil {
		execCtx = context.Background()
	}
	select {
	case <-execCtx.Done():
		return NewErrorResult(1, execCtx.Err())
	default:
	}

	_, err = luaRT.MainThread().CallContext(luaCtx, func() error {
		_, callErr := luart.Call1(luaRT.MainThread(), luart.FunctionValue(chunk), argValues...)
		if callErr != nil {
			return fmt.Errorf("call lua chunk: %w", callErr)
		}
		return nil
	})
	if err != nil {
		return NewErrorResult(1, fmt.Errorf("run lua script: %w", err))
	}
	return NewSuccessResult()
}

func compileLuaChunk(script string) (*luart.Closure, error) {
	r := luart.New(nil)
	cleanup := loadSafeLuaLibs(r)
	defer cleanup()
	chunk, err := r.CompileAndLoadLuaChunk("script", []byte(script), luart.TableValue(r.GlobalEnv()))
	if err != nil {
		return nil, fmt.Errorf("compile lua chunk: %w", err)
	}
	return chunk, nil
}

func loadSafeLuaLibs(r *luart.Runtime) func() {
	// golua library loaders declare compliance on shared GoFunction instances,
	// so VM bootstrap must be serialized under the race detector.
	luaLibraryLoadMu.Lock()
	defer luaLibraryLoadMu.Unlock()
	cleanup := lib.LoadLibs(
		r,
		base.LibLoader,
		packagelib.LibLoader,
		coroutine.LibLoader,
		stringlib.LibLoader,
		tablelib.LibLoader,
		mathlib.LibLoader,
		utf8lib.LibLoader,
	)
	removeUnsafeLuaGlobals(r)
	return func() {
		luaLibraryLoadMu.Lock()
		defer luaLibraryLoadMu.Unlock()
		cleanup()
	}
}

func removeUnsafeLuaGlobals(r *luart.Runtime) {
	for _, name := range []string{"dofile", "loadfile", "package", "rawset", "require"} {
		r.SetEnv(r.GlobalEnv(), name, luart.NilValue)
	}
}

func validateLuaInterpreter(script invowkfile.ImplementationScript, scriptContent string) error {
	interpInfo := script.ResolveInterpreterFromScript(scriptContent)
	if !interpInfo.Found || invowkfile.IsLuaInterpreter(interpInfo.Interpreter) {
		return nil
	}
	return fmt.Errorf("%w (got %q); virtual-lua can execute only Lua-compatible interpreters", invowkfile.ErrInterpreterNotAllowed, interpInfo.Interpreter)
}

func luaContextDef(cfg *invowkfile.RuntimeConfig) (luart.RuntimeContextDef, error) {
	var def luart.RuntimeContextDef
	if cfg == nil {
		return def, nil
	}
	def.HardLimits.Cpu = uint64(cfg.CPULimit)
	if cfg.MemoryLimit != "" {
		memory, err := parseLuaMemoryLimit(cfg.MemoryLimit.String())
		if err != nil {
			return def, err
		}
		def.HardLimits.Memory = memory
	}
	return def, nil
}

func parseLuaMemoryLimit(raw string) (uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	multiplier := uint64(1)
	last := raw[len(raw)-1]
	switch last {
	case 'K', 'k':
		multiplier = 1024
		raw = raw[:len(raw)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		raw = raw[:len(raw)-1]
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		raw = raw[:len(raw)-1]
	case 'B', 'b':
		raw = raw[:len(raw)-1]
	}
	if raw != "" {
		suffix := raw[len(raw)-1]
		switch suffix {
		case 'K', 'k':
			multiplier = 1024
			raw = raw[:len(raw)-1]
		case 'M', 'm':
			multiplier = 1024 * 1024
			raw = raw[:len(raw)-1]
		case 'G', 'g':
			multiplier = 1024 * 1024 * 1024
			raw = raw[:len(raw)-1]
		}
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory_limit %q: %w", raw, err)
	}
	return value * multiplier, nil
}

func installInvowkLuaBridge(
	ctx context.Context,
	r *luart.Runtime,
	req luaBridgeInstallRequest,
) {
	invowk := luart.NewTable()
	state := luart.NewTable()
	r.SetTable(state, luart.StringValue("bin_path"), luart.StringValue(req.env[EnvVarStateBinPath]))
	stateProxy, stateLockFunc := luaReadOnlyProxyTable(r, state, "invowk.state")
	r.SetTable(invowk, luart.StringValue("state"), luart.TableValue(stateProxy))
	r.SetTable(invowk, luart.StringValue("env"), luart.TableValue(luaReadOnlyEnvTable(r, req.env)))
	pathFunc := r.SetEnvGoFunc(invowk, "path", luaPathFunc(req.pathResolver, req.workDir), 1, false)
	commandConfig := luaCommandBridgeConfig{
		policy:           req.policy,
		registry:         req.registry,
		pathValidator:    req.pathValidator,
		env:              req.env,
		workDir:          req.workDir,
		stdin:            req.stdin,
		stdout:           req.stdout,
		stderr:           req.stderr,
		state:            state,
		utilitiesEnabled: req.utilitiesEnabled,
	}
	cmdTable, cmdFuncs := luaCommandHelperTable(ctx, r, commandConfig, false)
	captureTable, captureFuncs := luaCommandHelperTable(ctx, r, commandConfig, true)
	r.SetTable(invowk, luart.StringValue("cmd"), luart.TableValue(cmdTable))
	r.SetTable(invowk, luart.StringValue("capture"), luart.TableValue(captureTable))
	getenvFunc := installLuaOSBridge(r, req.env)
	ioFuncs := installLuaIOBridge(r, req.pathValidator, req.workDir, req.stdin, req.stdout, req.stderr)
	requireFunc := installLuaRequireBridge(r, req.scriptBasePath)
	invowkProxy, invowkLockFunc := luaReadOnlyProxyTable(r, invowk, "invowk")
	r.SetEnv(r.GlobalEnv(), "invowk", luart.TableValue(invowkProxy))

	funcs := append([]*luart.GoFunction{pathFunc, getenvFunc, requireFunc, stateLockFunc, invowkLockFunc}, ioFuncs...)
	funcs = append(funcs, cmdFuncs...)
	funcs = append(funcs, captureFuncs...)
	luart.SolemnlyDeclareCompliance(luart.ComplyCpuSafe|luart.ComplyMemSafe|luart.ComplyIoSafe, funcs...)
}

func luaPathFunc(resolver virtualPathResolver, workDir string) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		path, err := c.StringArg(0)
		if err != nil {
			return nil, fmt.Errorf("read invowk.path argument: %w", err)
		}
		resolved, err := resolver.resolveBridgePath(path, workDir)
		if err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.StringValue(resolved)), nil
	}
}

func installLuaOSBridge(r *luart.Runtime, env map[string]string) *luart.GoFunction {
	osTable := luart.NewTable()
	getenvFunc := r.SetEnvGoFunc(osTable, "getenv", func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		name, err := c.StringArg(0)
		if err != nil {
			return nil, fmt.Errorf("read os.getenv argument: %w", err)
		}
		value, ok := env[name]
		if !ok {
			return c.PushingNext1(t.Runtime, luart.NilValue), nil
		}
		return c.PushingNext1(t.Runtime, luart.StringValue(value)), nil
	}, 1, false)
	r.SetEnv(r.GlobalEnv(), "os", luart.TableValue(osTable))
	return getenvFunc
}

func luaCommandHelperTable(
	ctx context.Context,
	r *luart.Runtime,
	config luaCommandBridgeConfig,
	capture bool,
) (*luart.Table, []*luart.GoFunction) {
	table := luart.NewTable()
	meta := luart.NewTable()
	bridge := luaCommandBridge{
		config:  config,
		capture: capture,
	}
	indexFunc := r.SetEnvGoFunc(meta, "__index", bridge.indexFunc(ctx), 2, false)
	callFunc := r.SetEnvGoFunc(meta, "__call", bridge.callFunc(ctx), 1, true)
	newIndexFunc := luaSetReadOnlyNewIndex(r, meta, "invowk command bridge")
	table.SetMetatable(meta)
	return table, []*luart.GoFunction{indexFunc, callFunc, newIndexFunc}
}

func (b luaCommandBridge) indexFunc(ctx context.Context) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		name, err := c.StringArg(1)
		if err != nil {
			return nil, fmt.Errorf("read invowk command name: %w", err)
		}
		fn := luart.NewGoFunction(b.namedFunc(ctx, name), "invowk command "+name, 0, true)
		fn.SolemnlyDeclareCompliance(luart.ComplyCpuSafe | luart.ComplyMemSafe)
		return c.PushingNext1(t.Runtime, luart.FunctionValue(fn)), nil
	}
}

func (b luaCommandBridge) callFunc(ctx context.Context) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		args, err := luaCommandArgs(c.Etc())
		if err != nil {
			return nil, err
		}
		return b.run(ctx, t, c, args)
	}
}

func (b luaCommandBridge) namedFunc(ctx context.Context, name string) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		args, err := luaCommandArgsWithName(name, c.Etc())
		if err != nil {
			return nil, err
		}
		return b.run(ctx, t, c, args)
	}
}

func (b luaCommandBridge) run(ctx context.Context, t *luart.Thread, c *luart.GoCont, args []string) (luart.Cont, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmdStdout := b.config.stdout
	cmdStderr := b.config.stderr
	if b.capture {
		cmdStdout = &stdout
		cmdStderr = &stderr
	}
	exitCode, runErr := b.runCommand(ctx, args, cmdStdout, cmdStderr)
	runtime := t.Runtime
	runtime.SetTable(b.config.state, luart.StringValue("bin_path"), luart.StringValue(b.config.env[EnvVarStateBinPath]))
	if b.capture {
		return c.PushingNext(
			runtime,
			luart.StringValue(stdout.String()),
			luart.StringValue(stderr.String()),
			luart.IntValue(int64(exitCode)),
		), nil
	}
	if runErr != nil {
		return c.PushingNext(runtime, luart.IntValue(int64(exitCode)), luart.StringValue(runErr.Error())), nil
	}
	return c.PushingNext1(runtime, luart.IntValue(int64(exitCode))), nil
}

func (b luaCommandBridge) runCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 1, errors.New(luaBinaryRequiredMsg)
	}
	if b.config.utilitiesEnabled && b.config.registry != nil {
		if _, found := b.config.registry.Lookup(args[0]); found {
			return runVirtualUtility(ctx, b.config.registry, virtualUtilityRunRequest{
				args:          args,
				pathValidator: b.config.pathValidator,
				env:           b.config.env,
				workDir:       b.config.workDir,
				stdin:         b.config.stdin,
				stdout:        stdout,
				stderr:        stderr,
			})
		}
	}
	return runAllowedHostBinary(ctx, b.config.policy, args, b.config.env, b.config.workDir, stdout, stderr)
}

func luaCommandArgs(values []luart.Value) ([]string, error) {
	if len(values) == 0 {
		return nil, errors.New(luaBinaryRequiredMsg)
	}
	name, ok := values[0].TryString()
	if !ok {
		return nil, errors.New("invowk command name must be a string")
	}
	return luaCommandArgsWithName(name, values[1:])
}

func luaCommandArgsWithName(name string, values []luart.Value) ([]string, error) {
	args := []string{name}
	for _, value := range values {
		arg, ok := value.TryString()
		if !ok {
			return nil, errors.New("invowk command arguments must be strings")
		}
		args = append(args, arg)
	}
	return args, nil
}

func luaReadOnlyEnvTable(r *luart.Runtime, env map[string]string) *luart.Table {
	table := luart.NewTable()
	meta := luart.NewTable()
	r.SetEnvGoFunc(meta, "__index", func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		name, err := c.StringArg(1)
		if err != nil {
			return nil, fmt.Errorf("read invowk.env key: %w", err)
		}
		value, ok := env[name]
		if !ok {
			return c.PushingNext1(t.Runtime, luart.NilValue), nil
		}
		return c.PushingNext1(t.Runtime, luart.StringValue(value)), nil
	}, 2, false).SolemnlyDeclareCompliance(luart.ComplyCpuSafe | luart.ComplyMemSafe)
	r.SetEnvGoFunc(meta, "__newindex", func(_ *luart.Thread, _ *luart.GoCont) (luart.Cont, error) {
		return nil, errors.New("invowk.env is read-only")
	}, 3, false).SolemnlyDeclareCompliance(luart.ComplyCpuSafe | luart.ComplyMemSafe)
	r.SetEnv(meta, "__metatable", luart.StringValue("locked"))
	table.SetMetatable(meta)
	return table
}

func luaReadOnlyProxyTable(r *luart.Runtime, backing *luart.Table, name string) (*luart.Table, *luart.GoFunction) {
	proxy := luart.NewTable()
	meta := luart.NewTable()
	r.SetTable(meta, luart.StringValue("__index"), luart.TableValue(backing))
	newIndexFunc := luaSetReadOnlyNewIndex(r, meta, name)
	r.SetEnv(meta, "__metatable", luart.StringValue("locked"))
	proxy.SetMetatable(meta)
	return proxy, newIndexFunc
}

func luaSetReadOnlyNewIndex(r *luart.Runtime, meta *luart.Table, name string) *luart.GoFunction {
	return r.SetEnvGoFunc(meta, "__newindex", func(_ *luart.Thread, _ *luart.GoCont) (luart.Cont, error) {
		return nil, fmt.Errorf("%s is read-only", name)
	}, 3, false)
}

func luaArgsTable(args []string) luart.Value {
	table := luart.NewTable()
	for i, arg := range args {
		table.Set(luart.IntValue(int64(i+1)), luart.StringValue(arg))
	}
	table.Set(luart.StringValue("n"), luart.IntValue(int64(len(args))))
	return luart.TableValue(table)
}

func luaArgValues(args []string) []luart.Value {
	values := make([]luart.Value, len(args))
	for i, arg := range args {
		values[i] = luart.StringValue(arg)
	}
	return values
}

func runAllowedHostBinary(ctx context.Context, policy *virtualHostBinaryPolicy, args []string, env map[string]string, workDir string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 1, errors.New(luaBinaryRequiredMsg)
	}
	path, err := policy.resolve(args[0])
	if err != nil {
		return int(errVirtualHostBinaryDeniedExitStatus), err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, path, args[1:]...)
	cmd.Env = EnvToSlice(env)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = cmdWaitDelay
	if runErr := cmd.Run(); runErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](runErr); ok {
			return exitErr.ExitCode(), runErr
		}
		return 1, runErr
	}
	return 0, nil
}

func runVirtualUtility(ctx context.Context, registry *uroot.Registry, req virtualUtilityRunRequest) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.stdin == nil {
		req.stdin = bytes.NewReader(nil)
	}
	if req.stdout == nil {
		req.stdout = io.Discard
	}
	if req.stderr == nil {
		req.stderr = io.Discard
	}
	handler := &uroot.HandlerContext{
		Stdin:  req.stdin,
		Stdout: req.stdout,
		Stderr: req.stderr,
		Dir:    req.workDir,
		LookupEnv: func(name string) (string, bool) {
			value, ok := req.env[name]
			return value, ok
		},
		ValidatePath: req.pathValidator.validate,
	}
	err := registry.Run(uroot.WithHandlerContext(ctx, handler), req.args[0], req.args)
	if err != nil {
		return 1, err
	}
	return 0, nil
}
