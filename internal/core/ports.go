package core

import "context"

// Store is the driven port for variable storage providers (e.g. AWS SSM).
type Store interface {
	// Put creates or overwrites a variable.
	Put(ctx context.Context, v Variable) error
	// Get returns the variable for key, or ErrNotFound.
	Get(ctx context.Context, key string) (Variable, error)
	// List returns every variable managed under the configured base path.
	List(ctx context.Context) ([]Variable, error)
}

// Renderer is the driven port for output formats (e.g. Terraform).
type Renderer interface {
	Render(vars []Variable) ([]byte, error)
}
