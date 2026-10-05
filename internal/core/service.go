package core

import (
	"context"
	"fmt"
	"sort"
)

// Service implements configurator's use cases on top of a Store.
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Set(ctx context.Context, v Variable) error {
	if err := ValidateKey(v.Key); err != nil {
		return err
	}
	return s.store.Put(ctx, v)
}

func (s *Service) Get(ctx context.Context, key string) (Variable, error) {
	if err := ValidateKey(key); err != nil {
		return Variable{}, err
	}
	return s.store.Get(ctx, key)
}

// List returns all variables sorted by key.
func (s *Service) List(ctx context.Context) ([]Variable, error) {
	vars, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Key < vars[j].Key })
	return vars, nil
}

// Load stores every variable in vars. All keys are validated before anything
// is written so a bad file doesn't leave the store half-updated.
func (s *Service) Load(ctx context.Context, vars []Variable) error {
	for _, v := range vars {
		if err := ValidateKey(v.Key); err != nil {
			return err
		}
	}
	for _, v := range vars {
		if err := s.store.Put(ctx, v); err != nil {
			return fmt.Errorf("storing %q: %w", v.Key, err)
		}
	}
	return nil
}

// Render lists all variables and renders them with r.
func (s *Service) Render(ctx context.Context, r Renderer) ([]byte, error) {
	vars, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	return r.Render(vars)
}
