package cmd

import (
	"os"
	"strings"
)

// Version is set by GoReleaser at build time. Do not update manually.
var Version = "dev"

// GetVersion returns the current version, preferring build-time version,
// then falling back to .version file, then "dev"
func GetVersion() string {
	// If version was set at build time (not "dev"), use it
	if Version != "dev" {
		return Version
	}

	// Try to read from .version file
	if content, err := os.ReadFile(".version"); err == nil {
		if v := strings.TrimSpace(string(content)); v != "" {
			return "v" + v
		}
	}

	// Fallback to "dev"
	return "dev"
}
