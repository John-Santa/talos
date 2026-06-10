// Package taloshome resolves the TALOS_HOME directory.
// TALOS_HOME can be overridden via the environment variable of the same name,
// which is REQUIRED in tests to avoid writing to the real ~/.talos.
package taloshome

import (
	"os"
	"path/filepath"
)

// Dir returns the effective TALOS_HOME directory.
// If the TALOS_HOME environment variable is set, it is returned as-is.
// Otherwise it returns ~/.talos.
func Dir() (string, error) {
	if v := os.Getenv("TALOS_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".talos"), nil
}
