package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Input              string            `yaml:"input" json:"input"`
	Backup             bool              `yaml:"backup" json:"backup"`
	Validate           bool              `yaml:"validate" json:"validate"`
	Exclude            []string          `yaml:"exclude" json:"exclude"`
	Mappings           map[string]string `yaml:"mappings" json:"mappings"`
	PaginationPriority []string          `yaml:"pagination_priority" json:"pagination_priority"`
}

// LoadConfig loads config from file (YAML/JSON) and merges with inline flags. If noConfig is true, ignores all config files and uses only CLI flags.
func LoadConfig(configPath string, inlineMaps []string, inputDir string, noConfig bool) (*Config, error) {
	cfg := &Config{}

	if !noConfig {
		// 1. Load from file if provided
		if configPath != "" {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}

		// 2. Load from .openapirc.yaml if present and not already loaded
		if configPath == "" {
			if _, err := os.Stat(".openapirc.yaml"); err == nil {
				data, err := os.ReadFile(".openapirc.yaml")
				if err == nil {
					if err := yaml.Unmarshal(data, cfg); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	// 3. Override with CLI flags
	if inputDir != "" {
		cfg.Input = inputDir
	}
	if len(inlineMaps) > 0 {
		if cfg.Mappings == nil {
			cfg.Mappings = make(map[string]string)
		}
		for _, m := range inlineMaps {
			parts := splitMap(m)
			if parts == nil {
				return nil, errors.New("invalid --map format, expected from=to")
			}
			cfg.Mappings[parts[0]] = parts[1]
		}
	}

	if cfg.Input == "" {
		return nil, errors.New("input directory is required")
	}

	return cfg, nil
}

// splitMap splits "from=to" into [from, to]
func splitMap(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}
