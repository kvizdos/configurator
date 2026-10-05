package ssm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/kvizdos/configurator/internal/core"
)

// fakeSSM is an in-memory stand-in for the SSM API.
type fakeSSM struct {
	params map[string]types.Parameter
}

func (f *fakeSSM) PutParameter(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	f.params[*in.Name] = types.Parameter{Name: in.Name, Value: in.Value, Type: in.Type}
	return &ssm.PutParameterOutput{}, nil
}

func (f *fakeSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	p, ok := f.params[*in.Name]
	if !ok {
		return nil, &types.ParameterNotFound{}
	}
	return &ssm.GetParameterOutput{Parameter: &p}, nil
}

func (f *fakeSSM) GetParametersByPath(_ context.Context, in *ssm.GetParametersByPathInput, _ ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	var out []types.Parameter
	prefix := *in.Path + "/"
	for name, p := range f.params {
		if rest, ok := strings.CutPrefix(name, prefix); ok && !strings.Contains(rest, "/") {
			out = append(out, p)
		}
	}
	return &ssm.GetParametersByPathOutput{Parameters: out}, nil
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	fake := &fakeSSM{params: map[string]types.Parameter{
		"/other/project/x": {Name: aws.String("/other/project/x"), Value: aws.String("nope"), Type: types.ParameterTypeString},
	}}
	s := New(fake, "/configurator/name/")

	if err := s.Put(ctx, core.Variable{Key: "plain", Value: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, core.Variable{Key: "secret", Value: "v2", Sensitive: true}); err != nil {
		t.Fatal(err)
	}

	if p := fake.params["/configurator/name/secret"]; p.Type != types.ParameterTypeSecureString {
		t.Fatalf("secret stored as %s, want SecureString", p.Type)
	}
	if p := fake.params["/configurator/name/plain"]; p.Type != types.ParameterTypeString {
		t.Fatalf("plain stored as %s, want String", p.Type)
	}

	got, err := s.Get(ctx, "secret")
	if err != nil || got != (core.Variable{Key: "secret", Value: "v2", Sensitive: true}) {
		t.Fatalf("Get(secret) = %+v, %v", got, err)
	}

	if _, err := s.Get(ctx, "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
	}

	vars, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 2 {
		t.Fatalf("List = %+v, want 2 variables under base path", vars)
	}
}

func TestPutRejectsEmptyValue(t *testing.T) {
	s := New(&fakeSSM{params: map[string]types.Parameter{}}, "/x")
	if err := s.Put(context.Background(), core.Variable{Key: "k"}); err == nil {
		t.Fatal("expected error for empty value")
	}
}
