// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

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
		return fmt.Errorf("[uroot] %s: command not found", name)
	}

	// Preprocess combined short flags for custom implementations only.
	// Upstream wrappers (NativePreprocessor) handle this internally via
	// unixflag.ArgsToGoArgs in their RunContext — double-preprocessing
	// would corrupt long flags (e.g., --recursive → -r -e -c -u ...).
	if _, isNative := cmd.(NativePreprocessor); !isNative && len(args) > 1 {
		preprocessed := unixflag.ArgsToGoArgs(args[1:])
		args = append([]string{args[0]}, preprocessed...)
	}

	return cmd.Run(ctx, args)
}

// BuildDefaultRegistry creates a new Registry pre-populated with all 28
// built-in u-root command implementations. Each call returns a fresh,
// independent instance suitable for injection into VirtualRuntime.
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
