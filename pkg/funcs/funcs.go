// Package funcs provides custom SQL functions for GoPage.
// These functions extend SQLite with capabilities like HTTP requests,
// LLM calls, and utility functions.
package funcs

import (
	"zombiezen.com/go/sqlite"
)

// Registry holds all custom SQL functions.
type Registry struct {
	funcs []Func
}

// Func represents a custom SQL function.
type Func struct {
	Name       string
	NumArgs    int
	Func       func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error)
	Deterministic bool
}

// New creates a new function registry with all built-in functions.
func New() *Registry {
	r := &Registry{}

	// Register all built-in functions
	r.Register(StringFuncs()...)
	r.Register(HashFuncs()...)
	r.Register(JSONFuncs()...)
	r.Register(HTTPFuncs()...)
	r.Register(LLMFuncs()...)
	r.Register(UtilFuncs()...)
	r.Register(SSEFuncs()...)

	return r
}

// Register adds functions to the registry.
func (r *Registry) Register(funcs ...Func) {
	r.funcs = append(r.funcs, funcs...)
}

// Apply registers all functions on a SQLite connection.
func (r *Registry) Apply(conn *sqlite.Conn) error {
	for _, f := range r.funcs {
		fn := f // capture for closure
		impl := &sqlite.FunctionImpl{
			NArgs:         fn.NumArgs,
			Deterministic: fn.Deterministic,
			Scalar:        fn.Func,
		}

		err := conn.CreateFunction(f.Name, impl)
		if err != nil {
			return err
		}
	}
	return nil
}
