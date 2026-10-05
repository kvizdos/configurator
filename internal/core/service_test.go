package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kvizdos/configurator/internal/adapters/storage/memory"
	"github.com/kvizdos/configurator/internal/core"
)

func TestServiceSetGetList(t *testing.T) {
	ctx := context.Background()
	svc := core.NewService(memory.New())

	for _, v := range []core.Variable{
		{Key: "zeta", Value: "z"},
		{Key: "alpha", Value: "a", Sensitive: true},
	} {
		if err := svc.Set(ctx, v); err != nil {
			t.Fatalf("Set(%q): %v", v.Key, err)
		}
	}

	got, err := svc.Get(ctx, "alpha")
	if err != nil || got.Value != "a" || !got.Sensitive {
		t.Fatalf("Get(alpha) = %+v, %v", got, err)
	}

	if _, err := svc.Get(ctx, "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
	}

	vars, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 2 || vars[0].Key != "alpha" || vars[1].Key != "zeta" {
		t.Fatalf("List not sorted: %+v", vars)
	}
}

func TestServiceRejectsInvalidKeys(t *testing.T) {
	svc := core.NewService(memory.New())
	for _, key := range []string{"", "1abc", "a/b", "a.b", "has space"} {
		if err := svc.Set(context.Background(), core.Variable{Key: key, Value: "x"}); err == nil {
			t.Errorf("Set(%q) succeeded, want error", key)
		}
	}
}

func TestServiceLoadValidatesBeforeWriting(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	svc := core.NewService(store)

	err := svc.Load(ctx, []core.Variable{{Key: "good", Value: "1"}, {Key: "bad/key", Value: "2"}})
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if vars, _ := store.List(ctx); len(vars) != 0 {
		t.Fatalf("store modified despite invalid input: %+v", vars)
	}
}
