// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// DefaultRegistry is the global registry used by the virtual runtime.
// Commands are registered during package initialization.
var DefaultRegistry = NewRegistry()

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
func (r *Registry) Run(ctx context.Context, name string, args []string) error {
	cmd, ok := r.Lookup(name)
	if !ok {
		return fmt.Errorf("[uroot] %s: command not found", name)
	}
	return cmd.Run(ctx, args)
}

// RegisterDefault registers a command in the DefaultRegistry.
// This is typically called from init() functions in utility implementation files.
func RegisterDefault(cmd Command) {
	DefaultRegistry.Register(cmd)
}
