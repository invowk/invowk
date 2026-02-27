// SPDX-License-Identifier: MPL-2.0

package constructorvalidates

import "fmt"

// --- Types with Validate() ---

type Config struct {
	name string // want `struct field constructorvalidates\.Config\.name uses primitive type string`
}

func (c *Config) Validate() error {
	if c.name == "" {
		return fmt.Errorf("empty name")
	}
	return nil
}

// NewConfig calls Validate() — should NOT be flagged by constructor-validates.
func NewConfig(name string) (*Config, error) { // want `parameter "name" of constructorvalidates\.NewConfig uses primitive type string`
	c := &Config{name: name}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// --- Constructor that does NOT call Validate() ---

type Server struct {
	addr string // want `struct field constructorvalidates\.Server\.addr uses primitive type string`
}

func (s *Server) Validate() error {
	if s.addr == "" {
		return fmt.Errorf("empty addr")
	}
	return nil
}

// NewServer does NOT call Validate() — should be flagged.
func NewServer(addr string) (*Server, error) { // want `parameter "addr" of constructorvalidates\.NewServer uses primitive type string` `constructor constructorvalidates\.NewServer returns constructorvalidates\.Server which has Validate\(\) but never calls it`
	return &Server{addr: addr}, nil
}

// --- Non-validating factory (with ignore directive) ---

type Options struct {
	debug bool
}

func (o *Options) Validate() error {
	return nil
}

//goplint:ignore -- non-validating factory for tests
func NewOptionsFromDefaults() *Options {
	return &Options{}
}

// --- Type WITHOUT Validate() — constructors should not be flagged ---

type SimpleType struct {
	value string // want `struct field constructorvalidates\.SimpleType\.value uses primitive type string`
}

func NewSimpleType(value string) *SimpleType { // want `parameter "value" of constructorvalidates\.NewSimpleType uses primitive type string`
	return &SimpleType{value: value}
}

// --- Constructor returning interface — should not be flagged ---

type Engine interface {
	Run() error
}

type engineImpl struct {
	name string // want `struct field constructorvalidates\.engineImpl\.name uses primitive type string`
}

func (e *engineImpl) Run() error     { return nil }
func (e *engineImpl) Validate() error { return nil }

func NewEngine(name string) Engine { // want `parameter "name" of constructorvalidates\.NewEngine uses primitive type string`
	return &engineImpl{name: name}
}

// --- Variant constructor calling Validate() — should NOT be flagged ---

type Resolver struct {
	path string // want `struct field constructorvalidates\.Resolver\.path uses primitive type string`
}

func (r *Resolver) Validate() error {
	if r.path == "" {
		return fmt.Errorf("empty path")
	}
	return nil
}

func NewResolverFromPath(path string) (*Resolver, error) { // want `parameter "path" of constructorvalidates\.NewResolverFromPath uses primitive type string`
	r := &Resolver{path: path}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return r, nil
}

// --- False-negative test: validates parameter, not return type ---

// Handler has Validate().
type Handler struct {
	config Config
}

func (h *Handler) Validate() error {
	return h.config.Validate()
}

// NewHandler validates Config (the parameter type) but NOT Handler (the return type).
// The old heuristic would accept this because cfg.Validate() exists in the body.
// The receiver-aware check correctly flags this.
func NewHandler(cfg Config) (*Handler, error) { // want `constructor constructorvalidates\.NewHandler returns constructorvalidates\.Handler which has Validate\(\) but never calls it`
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Handler{config: cfg}, nil
}
