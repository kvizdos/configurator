// Package ssm is a core.Store backed by AWS Systems Manager Parameter Store.
// Each variable lives at <basePath>/<key>; sensitive variables are stored as
// SecureString and everything else as String.
package ssm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/kvizdos/configurator/internal/core"
)

// API is the subset of the SSM client used by Store.
type API interface {
	PutParameter(ctx context.Context, in *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	GetParameter(ctx context.Context, in *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	GetParametersByPath(ctx context.Context, in *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
}

type Store struct {
	client   API
	basePath string
}

var _ core.Store = (*Store)(nil)

func New(client API, basePath string) *Store {
	return &Store{client: client, basePath: strings.TrimRight(basePath, "/")}
}

// NewFromProfile builds a Store using the shared AWS config. Empty profile or
// region fall back to the SDK's default resolution chain.
func NewFromProfile(ctx context.Context, profile, region, basePath string) (*Store, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return New(ssm.NewFromConfig(cfg), basePath), nil
}

func (s *Store) name(key string) string {
	return s.basePath + "/" + key
}

func (s *Store) Put(ctx context.Context, v core.Variable) error {
	if v.Value == "" {
		return fmt.Errorf("%q: SSM does not allow empty values", v.Key)
	}
	typ := types.ParameterTypeString
	if v.Sensitive {
		typ = types.ParameterTypeSecureString
	}
	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.name(v.Key)),
		Value:     aws.String(v.Value),
		Type:      typ,
		Overwrite: aws.Bool(true),
	})
	return err
}

func (s *Store) Get(ctx context.Context, key string) (core.Variable, error) {
	out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(s.name(key)),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		var nf *types.ParameterNotFound
		if errors.As(err, &nf) {
			return core.Variable{}, fmt.Errorf("%q: %w", key, core.ErrNotFound)
		}
		return core.Variable{}, err
	}
	return s.toVariable(out.Parameter), nil
}

func (s *Store) List(ctx context.Context) ([]core.Variable, error) {
	p := ssm.NewGetParametersByPathPaginator(s.client, &ssm.GetParametersByPathInput{
		Path:           aws.String(s.basePath),
		Recursive:      aws.Bool(false),
		WithDecryption: aws.Bool(true),
	})
	var vars []core.Variable
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, param := range page.Parameters {
			vars = append(vars, s.toVariable(&param))
		}
	}
	return vars, nil
}

func (s *Store) toVariable(p *types.Parameter) core.Variable {
	return core.Variable{
		Key:       strings.TrimPrefix(aws.ToString(p.Name), s.basePath+"/"),
		Value:     aws.ToString(p.Value),
		Sensitive: p.Type == types.ParameterTypeSecureString,
	}
}
