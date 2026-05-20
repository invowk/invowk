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

	"github.com/arnodel/golua/lib"
	"github.com/arnodel/golua/lib/base"
	"github.com/arnodel/golua/lib/coroutine"
	"github.com/arnodel/golua/lib/mathlib"
	"github.com/arnodel/golua/lib/stringlib"
	"github.com/arnodel/golua/lib/tablelib"
	"github.com/arnodel/golua/lib/utf8lib"
	luart "github.com/arnodel/golua/runtime"

	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// LuaRuntime executes commands using the embedded golua interpreter.
	LuaRuntime struct {
		//plint:internal -- required constructor param; immutable after construction
		utilitiesEnabled bool
		//plint:internal -- field has WithLuaEnvBuilder(); field name doesn't match pattern
		envBuilder EnvBuilder
	}

	// LuaRuntimeOption configures a LuaRuntime.
	LuaRuntimeOption func(*LuaRuntime)
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
	env[EnvVarStateBinPath] = ""
	ctx.AddTUIEnv(env)

	luaCtx, err := luaContextDef(selectedRuntimeConfig(ctx))
	if err != nil {
		return NewErrorResult(1, err)
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	luaRT := luart.New(stdout)
	cleanup := loadSafeLuaLibs(luaRT)
	defer cleanup()
	policy := hostBinaryPolicy(ctx, env)
	installInvowkLuaBridge(ctx.Context, luaRT, policy, env, ctx.EffectiveWorkDir(), stdout, stderr, r.utilitiesEnabled)
	luaRT.SetEnv(luaRT.GlobalEnv(), "arg", luaArgsTable(ctx.PositionalArgs))
	chunk, err := luaRT.CompileAndLoadLuaChunk("script", []byte(script), luart.TableValue(luaRT.GlobalEnv()))
	if err != nil {
		return NewErrorResult(1, fmt.Errorf("compile lua script: %w", err))
	}

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}
	select {
	case <-execCtx.Done():
		return NewErrorResult(1, execCtx.Err())
	default:
	}

	_, err = luaRT.MainThread().CallContext(luaCtx, func() error {
		_, callErr := luart.Call1(luaRT.MainThread(), luart.FunctionValue(chunk))
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
	return lib.LoadLibs(
		r,
		base.LibLoader,
		coroutine.LibLoader,
		stringlib.LibLoader,
		tablelib.LibLoader,
		mathlib.LibLoader,
		utf8lib.LibLoader,
	)
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

func installInvowkLuaBridge(ctx context.Context, r *luart.Runtime, policy *virtualHostBinaryPolicy, env map[string]string, workDir string, stdout, stderr io.Writer, utilitiesEnabled bool) {
	invowk := luart.NewTable()
	state := luart.NewTable()
	r.SetTable(state, luart.StringValue("bin_path"), luart.StringValue(env[EnvVarStateBinPath]))
	r.SetTable(invowk, luart.StringValue("state"), luart.TableValue(state))
	r.SetTable(invowk, luart.StringValue("env"), luaEnvTable(env))
	cmdFunc := r.SetEnvGoFunc(invowk, "cmd", luaCommandFunc(ctx, policy, env, workDir, state, stdout, stderr, utilitiesEnabled, false), 1, true)
	captureFunc := r.SetEnvGoFunc(invowk, "capture", luaCommandFunc(ctx, policy, env, workDir, state, stdout, stderr, utilitiesEnabled, true), 1, true)
	r.SetEnv(r.GlobalEnv(), "invowk", luart.TableValue(invowk))

	luart.SolemnlyDeclareCompliance(luart.ComplyCpuSafe|luart.ComplyMemSafe, cmdFunc, captureFunc)
}

func luaCommandFunc(ctx context.Context, policy *virtualHostBinaryPolicy, env map[string]string, workDir string, state *luart.Table, stdout, stderr io.Writer, utilitiesEnabled, capture bool) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if !utilitiesEnabled {
			return nil, errors.New("virtual utilities are disabled")
		}
		args, err := luaCommandArgs(c)
		if err != nil {
			return nil, err
		}
		var out bytes.Buffer
		cmdStdout := stdout
		if capture {
			cmdStdout = &out
		}
		exitCode, runErr := runAllowedHostBinary(ctx, policy, args, env, workDir, cmdStdout, stderr)
		runtime := t.Runtime
		runtime.SetTable(state, luart.StringValue("bin_path"), luart.StringValue(env[EnvVarStateBinPath]))
		if runErr != nil {
			return c.PushingNext(runtime, luart.IntValue(int64(exitCode)), luart.StringValue(runErr.Error())), nil
		}
		if capture {
			return c.PushingNext(runtime, luart.StringValue(out.String()), luart.IntValue(int64(exitCode))), nil
		}
		return c.PushingNext1(runtime, luart.IntValue(int64(exitCode))), nil
	}
}

func luaCommandArgs(c *luart.GoCont) ([]string, error) {
	name, err := c.StringArg(0)
	if err != nil {
		return nil, fmt.Errorf("read invowk command name: %w", err)
	}
	args := []string{name}
	for _, value := range c.Etc() {
		arg, ok := value.TryString()
		if !ok {
			return nil, errors.New("invowk command arguments must be strings")
		}
		args = append(args, arg)
	}
	return args, nil
}

func luaEnvTable(env map[string]string) luart.Value {
	table := luart.NewTable()
	for key, value := range env {
		table.Set(luart.StringValue(key), luart.StringValue(value))
	}
	return luart.TableValue(table)
}

func luaArgsTable(args []string) luart.Value {
	table := luart.NewTable()
	for i, arg := range args {
		table.Set(luart.IntValue(int64(i+1)), luart.StringValue(arg))
	}
	table.Set(luart.StringValue("n"), luart.IntValue(int64(len(args))))
	return luart.TableValue(table)
}

func runAllowedHostBinary(ctx context.Context, policy *virtualHostBinaryPolicy, args []string, env map[string]string, workDir string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 1, errors.New("binary name is required")
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
