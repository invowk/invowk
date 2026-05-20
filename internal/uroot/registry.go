// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

// ErrCommandNotFound indicates that a u-root command name is not registered.
var ErrCommandNotFound = errors.New("command not found")

// Registry manages the mapping of command names to their u-root implementations.
// It is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	commands map[string]Command
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry.
// Panics if a command with the same name is already registered.
func (r *Registry) Register(cmd Command) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := cmd.Name()
	if name == "" {
		panic("uroot: cannot register command with empty name")
	}
	if _, exists := r.commands[name]; exists {
		panic(fmt.Sprintf("uroot: command %q already registered", name))
	}
	r.commands[name] = cmd
}

// Lookup retrieves a command by name.
// Returns nil, false if the command is not registered.
func (r *Registry) Lookup(name string) (Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd, ok := r.commands[name]
	return cmd, ok
}

// Names returns the names of all registered commands in sorted order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Run executes a command by name with the given context and arguments.
// Returns an error if the command is not found.
// The args slice should include the command name as args[0].
//
// For custom implementations (those not implementing NativePreprocessor),
// Run preprocesses args[1:] with unixflag.ArgsToGoArgs to split POSIX-style
// combined short flags (e.g., "-sf" → "-s", "-f"). Upstream wrappers handle
// this internally in their RunContext method and are skipped to avoid
// double-splitting that would corrupt long flags (e.g., --recursive → -r -e -c ...).
func (r *Registry) Run(ctx context.Context, name string, args []string) error {
	cmd, ok := r.Lookup(name)
	if !ok {
		return fmt.Errorf("[uroot] %s: %w", name, ErrCommandNotFound)
	}

	// Preprocess combined short flags for custom implementations only.
	// Upstream wrappers (NativePreprocessor) handle this internally via
	// unixflag.ArgsToGoArgs in their RunContext — double-preprocessing
	// would corrupt long flags (e.g., --recursive → -r -e -c -u ...).
	if _, isNative := cmd.(NativePreprocessor); !isNative && len(args) > 1 {
		preprocessed := unixflag.ArgsToGoArgs(args[1:])
		args = append([]string{args[0]}, preprocessed...)
	}
	var validateErr error
	args, validateErr = validateUrootCommandPathArgs(ctx, name, args)
	if validateErr != nil {
		return wrapError(name, validateErr)
	}

	return cmd.Run(ctx, args)
}

func validateUrootCommandPathArgs(ctx context.Context, name string, args []string) ([]string, error) {
	hc, ok := pathValidatingHandlerContext(ctx)
	if !ok {
		return args, nil
	}
	switch name {
	case "base64", "cat", "cp", "gzip", "ls", "mkdir", "mv", "rm", "shasum", "touch":
		if name == "shasum" {
			return validateShasumPathArgs(hc, args)
		}
		return validateNonOptionPathArgs(hc, args)
	case "find":
		return validateFindPathArgs(hc, args)
	case "tar":
		return validateTarPathArgs(hc, args)
	default:
		return args, nil
	}
}

func validateNonOptionPathArgs(hc *HandlerContext, args []string) ([]string, error) {
	validated := append([]string(nil), args...)
	for i := 1; i < len(validated); i++ {
		arg := validated[i]
		switch {
		case arg == "--":
			for j := i + 1; j < len(validated); j++ {
				if err := validatePathArg(hc, validated, j); err != nil {
					return nil, err
				}
			}
			return validated, nil
		case arg == "-" || strings.HasPrefix(arg, "-"):
			continue
		default:
			if err := validatePathArg(hc, validated, i); err != nil {
				return nil, err
			}
		}
	}
	return validated, nil
}

func validateFindPathArgs(hc *HandlerContext, args []string) ([]string, error) {
	validated := append([]string(nil), args...)
	for i := 1; i < len(validated); i++ {
		arg := validated[i]
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") || strings.ContainsAny(arg, `()!`) {
			break
		}
		if err := validatePathArg(hc, validated, i); err != nil {
			return nil, err
		}
	}
	return validated, nil
}

func validateShasumPathArgs(hc *HandlerContext, args []string) ([]string, error) {
	validated := append([]string(nil), args...)
	for i := 1; i < len(validated); i++ {
		arg := validated[i]
		switch {
		case arg == "--":
			for j := i + 1; j < len(validated); j++ {
				if err := validatePathArg(hc, validated, j); err != nil {
					return nil, err
				}
			}
			return validated, nil
		case arg == "-a" || arg == "-algorithm" || arg == "--algorithm":
			i++
			continue
		case strings.HasPrefix(arg, "-a="),
			strings.HasPrefix(arg, "-algorithm="),
			strings.HasPrefix(arg, "--algorithm="):
			continue
		case arg == "-" || strings.HasPrefix(arg, "-"):
			continue
		default:
			if err := validatePathArg(hc, validated, i); err != nil {
				return nil, err
			}
		}
	}
	return validated, nil
}

func validateTarPathArgs(hc *HandlerContext, args []string) ([]string, error) {
	validated := append([]string(nil), args...)
	createMode := false
	fileValueIndexes := make(map[int]struct{})
	operandIndexes := make([]int, 0, len(validated))

	for i := 1; i < len(validated); i++ {
		arg := validated[i]
		if arg == "--" {
			for j := i + 1; j < len(validated); j++ {
				operandIndexes = append(operandIndexes, j)
			}
			break
		}
		if arg == "--file" {
			if i+1 < len(validated) {
				i++
				fileValueIndexes[i] = struct{}{}
			}
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--file="); ok {
			resolved, err := hc.ResolvePath(value)
			if err != nil {
				return nil, err
			}
			validated[i] = "--file=" + resolved
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags := strings.TrimPrefix(arg, "-")
			for flagIndex, flag := range flags {
				if flag == 'c' {
					createMode = true
				}
				if flag != 'f' {
					continue
				}
				value := flags[flagIndex+1:]
				if value == "" {
					if i+1 < len(validated) {
						i++
						fileValueIndexes[i] = struct{}{}
					}
					break
				}
				resolved, err := hc.ResolvePath(value)
				if err != nil {
					return nil, err
				}
				validated[i] = "-" + flags[:flagIndex+1] + resolved
				break
			}
			continue
		}
		operandIndexes = append(operandIndexes, i)
	}

	for index := range fileValueIndexes {
		if err := validatePathArg(hc, validated, index); err != nil {
			return nil, err
		}
	}
	if createMode {
		for _, index := range operandIndexes {
			if err := validatePathArg(hc, validated, index); err != nil {
				return nil, err
			}
		}
	}
	return validated, nil
}

func validatePathArg(hc *HandlerContext, args []string, index int) error {
	if args[index] == "-" {
		return nil
	}
	path, err := hc.ResolvePath(args[index])
	if err != nil {
		return err
	}
	args[index] = path
	return nil
}

// BuildDefaultRegistry creates a new Registry pre-populated with all 28
// built-in u-root command implementations. Each call returns a fresh,
// independent instance suitable for injection into ShRuntime.
func BuildDefaultRegistry() *Registry {
	r := NewRegistry()

	// Upstream wrappers (12)
	r.Register(newBase64Command())
	r.Register(newCatCommand())
	r.Register(newCpCommand())
	r.Register(newFindCommand())
	r.Register(newGzipCommand())
	r.Register(newLsCommand())
	r.Register(newMkdirCommand())
	r.Register(newMvCommand())
	r.Register(newRmCommand())
	r.Register(newShasumCommand())
	r.Register(newTarCommand())
	r.Register(newTouchCommand())

	// Custom implementations (16)
	r.Register(newBasenameCommand())
	r.Register(newCutCommand())
	r.Register(newDirnameCommand())
	r.Register(newGrepCommand())
	r.Register(newHeadCommand())
	r.Register(newLnCommand())
	r.Register(newMktempCommand())
	r.Register(newRealpathCommand())
	r.Register(newSeqCommand())
	r.Register(newSleepCommand())
	r.Register(newSortCommand())
	r.Register(newTailCommand())
	r.Register(newTeeCommand())
	r.Register(newTrCommand())
	r.Register(newUniqCommand())
	r.Register(newWcCommand())

	return r
}
