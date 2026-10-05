package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kvizdos/configurator/internal/adapters/input/jsonfile"
	"github.com/kvizdos/configurator/internal/adapters/output/terraform"
	"github.com/kvizdos/configurator/internal/config"
	"github.com/kvizdos/configurator/internal/core"
)

const mask = "********"

func newInitCmd() *cobra.Command {
	var c config.Config
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a " + config.FileName + " file in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if c.ProjectName == "" {
				c.ProjectName = filepath.Base(wd)
			}
			if c.VariableBasePath == "" {
				c.VariableBasePath = "/configurator/" + c.ProjectName
			}
			if err := c.Validate(); err != nil {
				return err
			}
			if _, ok := storeFactories[c.Provider]; !ok {
				return fmt.Errorf("unsupported provider %q", c.Provider)
			}

			path := filepath.Join(wd, config.FileName)
			flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			if !force {
				flags |= os.O_EXCL
			}
			f, err := os.OpenFile(path, flags, 0o644)
			if os.IsExist(err) {
				return fmt.Errorf("%s already exists (use --force to overwrite)", path)
			} else if err != nil {
				return err
			}
			if _, err := f.Write(c.Encode()); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&c.Provider, "provider", "aws", "storage provider")
	cmd.Flags().StringVar(&c.ProviderProfile, "profile", "", "provider profile (e.g. AWS shared config profile)")
	cmd.Flags().StringVar(&c.ProviderRegion, "region", "", "provider region (defaults to the profile's region)")
	cmd.Flags().StringVar(&c.ProjectName, "project", "", "project name (defaults to the current directory name)")
	cmd.Flags().StringVar(&c.VariableBasePath, "base-path", "", "variable base path (defaults to /configurator/<project>)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing "+config.FileName)
	return cmd
}

func newSetCmd() *cobra.Command {
	var sensitive bool
	cmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set a variable",
		Long: "Set a variable. If value is omitted it is read from piped stdin, or\n" +
			"prompted for interactively (hidden input with --sensitive).",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if err := core.ValidateKey(key); err != nil {
				return err
			}
			p, err := loadProject(cmd.Context())
			if err != nil {
				return err
			}

			var value string
			if len(args) == 2 {
				value = args[1]
			} else if value, err = readValue(cmd, key, sensitive); err != nil {
				return err
			}

			if err := p.service.Set(cmd.Context(), core.Variable{Key: key, Value: value, Sensitive: sensitive}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Set %s\n", key)
			return nil
		},
	}
	cmd.Flags().BoolVar(&sensitive, "sensitive", false, "store as a secret (e.g. SSM SecureString)")
	return cmd
}

// readValue reads a value from piped stdin, or prompts on a terminal.
func readValue(cmd *cobra.Command, key string, sensitive bool) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Value for %s: ", key)
	if sensitive {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(cmd.ErrOrStderr())
		return string(b), err
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print a variable's value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadProject(cmd.Context())
			if err != nil {
				return err
			}
			v, err := p.service.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v.Value)
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all variables (sensitive values are masked)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := loadProject(cmd.Context())
			if err != nil {
				return err
			}
			vars, err := p.service.List(cmd.Context())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tVALUE\tSENSITIVE")
			for _, v := range vars {
				value := v.Value
				if v.Sensitive {
					value = mask
				}
				fmt.Fprintf(w, "%s\t%s\t%t\n", v.Key, value, v.Sensitive)
			}
			return w.Flush()
		},
	}
}

func newLoadCmd() *cobra.Command {
	var sensitive bool
	cmd := &cobra.Command{
		Use:   "load <file.json>",
		Short: "Store every variable from a JSON file",
		Long: `Store every variable from a JSON file. Values are either plain scalars
or objects that mark sensitivity:

  {
    "db_host": "db.internal",
    "db_pass": {"value": "hunter2", "sensitive": true}
  }

With --sensitive, every variable in the file is stored as sensitive.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			vars, err := jsonfile.Parse(f)
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			if sensitive {
				for i := range vars {
					vars[i].Sensitive = true
				}
			}

			p, err := loadProject(cmd.Context())
			if err != nil {
				return err
			}
			if err := p.service.Load(cmd.Context(), vars); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Loaded %d variable(s)\n", len(vars))
			return nil
		},
	}
	cmd.Flags().BoolVar(&sensitive, "sensitive", false, "store every variable in the file as a secret")
	return cmd
}

func newSyncCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Generate " + terraform.FileName + " from the stored variables",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := loadProject(cmd.Context())
			if err != nil {
				return err
			}
			r, err := terraform.New(p.cfg.Provider, p.cfg.BasePath())
			if err != nil {
				return err
			}
			out, err := p.service.Render(cmd.Context(), r)
			if err != nil {
				return err
			}
			if output == "" {
				output = filepath.Join(p.dir, terraform.FileName)
			}
			if err := os.WriteFile(output, out, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path (defaults to "+terraform.FileName+" next to "+config.FileName+")")
	return cmd
}
