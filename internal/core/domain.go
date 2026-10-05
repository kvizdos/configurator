// Package core holds configurator's domain model, the ports that adapters
// implement, and the application service that ties them together.
package core

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrNotFound is returned by a Store when a key does not exist.
var ErrNotFound = errors.New("variable not found")

// Variable is a single configuration value managed by configurator.
type Variable struct {
	Key       string
	Value     string
	Sensitive bool
}

// keyPattern keeps keys valid as both storage path segments and Terraform
// identifiers (they are emitted as `configurator_<key>`).
var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// ValidateKey reports whether key can be stored and rendered safely.
func ValidateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid key %q: must start with a letter or underscore and contain only letters, digits, '_' or '-'", key)
	}
	return nil
}
