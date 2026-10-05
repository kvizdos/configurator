// Package memory is an in-process core.Store, useful for tests.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/kvizdos/configurator/internal/core"
)

type Store struct {
	mu   sync.RWMutex
	vars map[string]core.Variable
}

var _ core.Store = (*Store)(nil)

func New() *Store {
	return &Store{vars: map[string]core.Variable{}}
}

func (s *Store) Put(_ context.Context, v core.Variable) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vars[v.Key] = v
	return nil
}

func (s *Store) Get(_ context.Context, key string) (core.Variable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vars[key]
	if !ok {
		return core.Variable{}, fmt.Errorf("%q: %w", key, core.ErrNotFound)
	}
	return v, nil
}

func (s *Store) List(_ context.Context) ([]core.Variable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vars := make([]core.Variable, 0, len(s.vars))
	for _, v := range s.vars {
		vars = append(vars, v)
	}
	return vars, nil
}
