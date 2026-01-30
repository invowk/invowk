# Functional Options Pattern

## Overview

Use the functional options pattern for constructors that accept optional configuration. This pattern provides sensible defaults, self-documenting APIs, and backward-compatible evolution.

Reference: [Dave Cheney - Functional options for friendly APIs](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)

## Pattern Structure

### 1. Define the Option Type

```go
// Option configures a Server.
type Option func(*Server)
```

### 2. Create With* Functions

Each option is a function returning an `Option` that captures the configuration:

```go
// WithTimeout sets the server timeout duration.
func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

// WithLogger sets the server logger.
func WithLogger(logger *slog.Logger) Option {
    return func(s *Server) {
        s.logger = logger
    }
}

// WithMaxConnections sets the maximum concurrent connections.
func WithMaxConnections(max int) Option {
    return func(s *Server) {
        s.maxConns = max
    }
}
```

### 3. Constructor with Variadic Options

```go
// NewServer creates a Server with the given address and options.
// Default timeout is 30s, default max connections is 100.
func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:     addr,
        timeout:  30 * time.Second,  // sensible default
        maxConns: 100,               // sensible default
        logger:   slog.Default(),    // sensible default
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

## Usage Examples

**Default configuration (most common case):**
```go
server := NewServer("localhost:8080")
```

**With custom configuration:**
```go
server := NewServer("localhost:8080",
    WithTimeout(60 * time.Second),
    WithLogger(customLogger),
)
```

**Conditional options:**
```go
opts := []Option{WithTimeout(60 * time.Second)}
if enableTLS {
    opts = append(opts, WithTLS(certPath, keyPath))
}
server := NewServer(addr, opts...)
```

## When to Use

Use functional options when:

- A constructor has more than 2-3 optional parameters
- You want sensible defaults without requiring callers to specify them
- The API may evolve with new options over time
- Configuration is self-documenting (option names describe their effect)

## When NOT to Use

Avoid functional options when:

- All parameters are required (use regular function parameters)
- Only 1-2 optional parameters exist (a single optional parameter or config struct may suffice)
- Performance is critical in hot paths (options add indirection)

## Anti-Patterns

```go
// WRONG: Config struct with zero-value ambiguity
type Config struct {
    Timeout time.Duration  // Is 0 "use default" or "no timeout"?
    MaxConns int           // Is 0 "unlimited" or "use default"?
}
func NewServer(addr string, cfg Config) *Server { ... }

// WRONG: Too many positional parameters
func NewServer(addr string, timeout time.Duration, maxConns int, logger *slog.Logger) *Server { ... }

// CORRECT: Functional options
func NewServer(addr string, opts ...Option) *Server { ... }
```

## Naming Conventions

- Option type: `Option` (or `<Type>Option` if multiple option types exist in the package)
- Option functions: `With<OptionName>` (e.g., `WithTimeout`, `WithLogger`, `WithTLS`)
- Document the default value in the `With*` function's doc comment or constructor comment

## Common Pitfall

- **Missing defaults documentation** - Always document what the default value is when an option is not provided, either in the constructor's doc comment or in each `With*` function.
