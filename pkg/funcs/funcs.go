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

	return r
}

// Register adds functions to the registry.
func (r *Registry) Register(funcs ...Func) {
	r.funcs = append(r.funcs, funcs...)
}

// Apply registers all functions on a SQLite connection.
func (r *Registry) Apply(conn *sqlite.Conn) error {
	for _, f := range r.funcs {
		opts := &sqlite.FunctionOptions{
			NArgs:         f.NumArgs,
			Deterministic: f.Deterministic,
		}

		// Create a scalar function
		fn := f // capture for closure
		err := conn.CreateFunction(f.Name, opts, func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
			return fn.Func(ctx, args)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
