// Package cli is the driving adapter: Cobra commands that wire config,
// storage providers and output formats into the core service.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kvizdos/configurator/internal/adapters/storage/ssm"
	"github.com/kvizdos/configurator/internal/config"
	"github.com/kvizdos/configurator/internal/core"
)

// storeFactories maps a PROVIDER value to its storage adapter.
var storeFactories = map[string]func(ctx context.Context, c config.Config) (core.Store, error){
	"aws": func(ctx context.Context, c config.Config) (core.Store, error) {
		return ssm.NewFromProfile(ctx, c.ProviderProfile, c.ProviderRegion, c.BasePath())
	},
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "configurator",
		Short:         "Manage variables & secrets for Terraform",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newInitCmd(),
		newSetCmd(),
		newGetCmd(),
		newListCmd(),
		newLoadCmd(),
		newSyncCmd(),
	)
	return root
}

// project is the loaded .configurator file plus a service for its provider.
type project struct {
	cfg     config.Config
	dir     string
	service *core.Service
}

func loadProject(ctx context.Context) (*project, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg, dir, err := config.Load(wd)
	if err != nil {
		return nil, err
	}
	factory, ok := storeFactories[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported PROVIDER %q", cfg.Provider)
	}
	store, err := factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &project{cfg: cfg, dir: dir, service: core.NewService(store)}, nil
}
