package funcoptions

// --- Detection: too many non-option parameters ---

// TooManyParams has a constructor with 4 params — should suggest functional options.
type TooManyParams struct {
	a int // want `struct field funcoptions\.TooManyParams\.a uses primitive type int`
	b int // want `struct field funcoptions\.TooManyParams\.b uses primitive type int`
	c int // want `struct field funcoptions\.TooManyParams\.c uses primitive type int`
	d int // want `struct field funcoptions\.TooManyParams\.d uses primitive type int`
}

func NewTooManyParams(a, b, c, d int) *TooManyParams { return &TooManyParams{a: a, b: b, c: c, d: d} } // want `constructor NewTooManyParams\(\) for funcoptions\.TooManyParams has 4 non-option parameters; consider using functional options` `parameter "a" of funcoptions\.NewTooManyParams uses primitive type int` `parameter "b" of funcoptions\.NewTooManyParams uses primitive type int` `parameter "c" of funcoptions\.NewTooManyParams uses primitive type int` `parameter "d" of funcoptions\.NewTooManyParams uses primitive type int`

// ThreeParams has exactly 3 params — at the threshold, no flag.
type ThreeParams struct {
	x int // want `struct field funcoptions\.ThreeParams\.x uses primitive type int`
	y int // want `struct field funcoptions\.ThreeParams\.y uses primitive type int`
	z int // want `struct field funcoptions\.ThreeParams\.z uses primitive type int`
}

func NewThreeParams(x, y, z int) *ThreeParams { return &ThreeParams{x: x, y: y, z: z} } // want `parameter "x" of funcoptions\.NewThreeParams uses primitive type int` `parameter "y" of funcoptions\.NewThreeParams uses primitive type int` `parameter "z" of funcoptions\.NewThreeParams uses primitive type int`

// --- Completeness: already has options, all wired correctly ---

// HasOptions is fully wired — no diagnostics for func-options.
type HasOptions struct {
	shell string // want `struct field funcoptions\.HasOptions\.shell uses primitive type string`
	args  string // want `struct field funcoptions\.HasOptions\.args uses primitive type string`
}

// HasOptionsOption is the functional option type for HasOptions.
type HasOptionsOption func(*HasOptions)

// WithShell sets the shell.
func WithShell(s string) HasOptionsOption { return func(h *HasOptions) { h.shell = s } } // want `parameter "s" of funcoptions\.WithShell uses primitive type string`

// WithArgs sets the args.
func WithArgs(a string) HasOptionsOption { return func(h *HasOptions) { h.args = a } } // want `parameter "a" of funcoptions\.WithArgs uses primitive type string`

// NewHasOptions creates a HasOptions with options.
func NewHasOptions(opts ...HasOptionsOption) *HasOptions {
	h := &HasOptions{}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// --- Completeness: missing a WithXxx function ---

// Incomplete has an option type but lacks WithTimeout (for the timeout field).
type Incomplete struct {
	host    string // want `struct field funcoptions\.Incomplete\.host uses primitive type string`
	timeout int    // want `struct field funcoptions\.Incomplete\.timeout uses primitive type int` `struct funcoptions\.Incomplete has IncompleteOption type but field "timeout" has no WithTimeout\(\) function`
}

// IncompleteOption is the functional option type for Incomplete.
type IncompleteOption func(*Incomplete)

// WithHost satisfies the host field.
func WithHost(h string) IncompleteOption { return func(i *Incomplete) { i.host = h } } // want `parameter "h" of funcoptions\.WithHost uses primitive type string`

// NewIncomplete creates an Incomplete.
func NewIncomplete(opts ...IncompleteOption) *Incomplete {
	i := &Incomplete{}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// --- Completeness: constructor not variadic ---

// NoVariadic has an option type but constructor doesn't accept it.
type NoVariadic struct {
	name string // want `struct field funcoptions\.NoVariadic\.name uses primitive type string` `struct funcoptions\.NoVariadic has NoVariadicOption type but field "name" has no WithName\(\) function`
}

// NoVariadicOption is the functional option type for NoVariadic.
type NoVariadicOption func(*NoVariadic)

func NewNoVariadic(name string) *NoVariadic { return &NoVariadic{name: name} } // want `constructor NewNoVariadic\(\) for funcoptions\.NoVariadic does not accept variadic \.\.\.NoVariadicOption` `parameter "name" of funcoptions\.NewNoVariadic uses primitive type string`
