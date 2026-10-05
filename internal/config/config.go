// Package config reads and writes the project's .configurator file, a simple
// KEY=VALUE (dotenv-style) file.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const FileName = ".configurator"

// ErrNotFound is returned by Find when no .configurator file exists in the
// directory or any of its parents.
var ErrNotFound = errors.New(FileName + " not found (run `configurator init`)")

type Config struct {
	Provider         string
	ProviderProfile  string
	ProviderRegion   string
	ProjectName      string
	VariableBasePath string
}

// basePathPattern matches the characters AWS SSM allows in parameter names,
// which also keeps the path safe to embed in generated Terraform strings.
var basePathPattern = regexp.MustCompile(`^/[A-Za-z0-9_./-]*$`)

func (c Config) Validate() error {
	var errs []error
	if c.Provider == "" {
		errs = append(errs, errors.New("PROVIDER is required"))
	}
	if c.ProjectName == "" {
		errs = append(errs, errors.New("PROJECT_NAME is required"))
	}
	if !basePathPattern.MatchString(c.VariableBasePath) {
		errs = append(errs, fmt.Errorf("VARIABLE_BASE_PATH %q must start with '/' and contain only letters, digits, '_', '.', '-' or '/'", c.VariableBasePath))
	}
	return errors.Join(errs...)
}

// BasePath returns VARIABLE_BASE_PATH without a trailing slash.
func (c Config) BasePath() string {
	return strings.TrimRight(c.VariableBasePath, "/")
}

// Parse reads a .configurator file. Blank lines and lines starting with '#'
// are ignored; values may optionally be wrapped in single or double quotes.
func Parse(r io.Reader) (Config, error) {
	var c Config
	fields := map[string]*string{
		"PROVIDER":           &c.Provider,
		"PROVIDER_PROFILE":   &c.ProviderProfile,
		"PROVIDER_REGION":    &c.ProviderRegion,
		"PROJECT_NAME":       &c.ProjectName,
		"VARIABLE_BASE_PATH": &c.VariableBasePath,
	}

	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: expected KEY=VALUE", n)
		}
		key = strings.TrimSpace(key)
		dst, known := fields[key]
		if !known {
			return Config{}, fmt.Errorf("line %d: unknown key %q", n, key)
		}
		*dst = unquote(strings.TrimSpace(value))
	}
	if err := sc.Err(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// Encode renders c in .configurator format. Optional empty fields are omitted.
func (c Config) Encode() []byte {
	var b strings.Builder
	write := func(k, v string) { fmt.Fprintf(&b, "%s=%s\n", k, v) }
	write("PROVIDER", c.Provider)
	if c.ProviderProfile != "" {
		write("PROVIDER_PROFILE", c.ProviderProfile)
	}
	if c.ProviderRegion != "" {
		write("PROVIDER_REGION", c.ProviderRegion)
	}
	write("PROJECT_NAME", c.ProjectName)
	write("VARIABLE_BASE_PATH", c.VariableBasePath)
	return []byte(b.String())
}

// Find walks up from dir looking for a .configurator file and returns its path.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, FileName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// Load finds, parses and validates the nearest .configurator file. It also
// returns the directory containing it.
func Load(dir string) (Config, string, error) {
	path, err := Find(dir)
	if err != nil {
		return Config{}, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return Config{}, "", err
	}
	defer f.Close()

	c, err := Parse(f)
	if err != nil {
		return Config{}, "", fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, "", fmt.Errorf("%s: %w", path, err)
	}
	return c, filepath.Dir(path), nil
}
