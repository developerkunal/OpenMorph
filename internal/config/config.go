package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete OpenMorph configuration
type Config struct {
	Input              string                   `yaml:"input" json:"input"`
	Output             string                   `yaml:"output" json:"output"`
	Backup             bool                     `yaml:"backup" json:"backup"`
	Validate           bool                     `yaml:"validate" json:"validate"`
	Exclude            []string                 `yaml:"exclude" json:"exclude"`
	Mappings           map[string]string        `yaml:"mappings" json:"mappings"`
	PaginationPriority []string                 `yaml:"pagination_priority" json:"pagination_priority"` // Global pagination strategy priority
	EndpointPagination []EndpointPaginationRule `yaml:"endpoint_pagination" json:"endpoint_pagination"` // Endpoint-specific pagination overrides
	FlattenResponses   bool                     `yaml:"flatten_responses" json:"flatten_responses"`
	VendorExtensions   VendorExtensions         `yaml:"vendor_extensions" json:"vendor_extensions"`
	DefaultValues      DefaultValues            `yaml:"default_values" json:"default_values"`
}

// EndpointPaginationRule defines pagination configuration for specific endpoints
// Allows overriding global pagination priority for individual endpoints
//
// Example:
//
//	endpoint: "/api/v1/users/*"  # Supports wildcard patterns
//	method: "GET"                # HTTP method (case-insensitive)
//	pagination: "cursor"         # Strategy: cursor, offset, page, checkpoint, none
type EndpointPaginationRule struct {
	Endpoint   string `yaml:"endpoint" json:"endpoint"`     // Endpoint pattern (supports wildcards like /api/v1/users/*)
	Method     string `yaml:"method" json:"method"`         // HTTP method (GET, POST, etc.) - case insensitive
	Pagination string `yaml:"pagination" json:"pagination"` // Pagination strategy (cursor, checkpoint, offset, page, none)
}

// VendorExtensions configuration for adding vendor-specific extensions
type VendorExtensions struct {
	Enabled   bool                      `yaml:"enabled" json:"enabled"`
	Providers map[string]ProviderConfig `yaml:"providers" json:"providers"`
}

// ProviderConfig defines configuration for a specific provider
type ProviderConfig struct {
	ExtensionName string                    `yaml:"extension_name" json:"extension_name"`
	TargetLevel   string                    `yaml:"target_level" json:"target_level"`   // "operation", "path", "schema"
	Methods       []string                  `yaml:"methods" json:"methods"`             // ["get", "post"] or empty for all
	PathPatterns  []string                  `yaml:"path_patterns" json:"path_patterns"` // ["/api/v1/*"] or empty for all
	FieldMapping  FieldMapping              `yaml:"field_mapping" json:"field_mapping"`
	Strategies    map[string]StrategyConfig `yaml:"strategies" json:"strategies"`
}

// FieldMapping defines how to map request/response fields
type FieldMapping struct {
	RequestParams  map[string][]string `yaml:"request_params" json:"request_params"`
	ResponseFields map[string][]string `yaml:"response_fields" json:"response_fields"`
}

// StrategyConfig defines the template for a pagination strategy
type StrategyConfig struct {
	Template       map[string]interface{} `yaml:"template" json:"template"`
	RequiredFields []string               `yaml:"required_fields" json:"required_fields"`
	OptionalFields []string               `yaml:"optional_fields" json:"optional_fields"`
}

// DefaultValues configuration for setting defaults in OpenAPI specs
type DefaultValues struct {
	Enabled bool                   `yaml:"enabled" json:"enabled"`
	Rules   map[string]DefaultRule `yaml:"rules" json:"rules"`
}

// DefaultRule defines a rule for setting default values
type DefaultRule struct {
	Target    DefaultTarget          `yaml:"target" json:"target"`
	Condition DefaultCondition       `yaml:"condition" json:"condition"`
	Value     interface{}            `yaml:"value" json:"value"`
	Template  map[string]interface{} `yaml:"template" json:"template"`
	Priority  int                    `yaml:"priority" json:"priority"`
}

// DefaultTarget specifies where the default should be applied
type DefaultTarget struct {
	Location string `yaml:"location" json:"location"` // "parameter", "request_body", "response", "component", "array", "enum"
	Property string `yaml:"property" json:"property"` // specific property name (optional)
	Path     string `yaml:"path" json:"path"`         // JSONPath-like selector (optional)
}

// DefaultCondition specifies when the default should be applied
type DefaultCondition struct {
	Type         string   `yaml:"type" json:"type"`                   // type constraint (e.g., "string", "integer", "boolean")
	ParameterIn  string   `yaml:"parameter_in" json:"parameter_in"`   // for parameters: "query", "path", "header", "cookie"
	HTTPMethods  []string `yaml:"http_methods" json:"http_methods"`   // which HTTP methods to target
	PathPatterns []string `yaml:"path_patterns" json:"path_patterns"` // which API paths to target
	HasEnum      bool     `yaml:"has_enum" json:"has_enum"`           // only apply if field has enum values
	IsArray      bool     `yaml:"is_array" json:"is_array"`           // only apply if field is array
	PropertyName string   `yaml:"property_name" json:"property_name"` // match specific property names
	Required     *bool    `yaml:"required" json:"required"`           // apply only to required/optional fields
}

// LoadConfig loads config from file (YAML/JSON) and merges with inline flags. If noConfig is true, ignores all config files and uses only CLI flags.
func LoadConfig(configPath string, inlineMaps []string, inputDir string, outputFile string, noConfig bool) (*Config, error) {
	cfg := &Config{}

	if !noConfig {
		if err := loadConfigFiles(cfg, configPath); err != nil {
			return nil, err
		}
	}

	applyCliOverrides(cfg, inputDir, outputFile, inlineMaps)

	if cfg.Input == "" {
		return nil, errors.New("input directory is required")
	}

	return cfg, nil
}

// loadConfigFiles loads configuration from config file and .openapirc.yaml
func loadConfigFiles(cfg *Config, configPath string) error {
	// 1. Load from file if provided
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return err
		}
	}

	// 2. Load from .openapirc.yaml if present and not already loaded
	if configPath == "" {
		if _, err := os.Stat(".openapirc.yaml"); err == nil {
			data, err := os.ReadFile(".openapirc.yaml")
			if err == nil {
				if err := yaml.Unmarshal(data, cfg); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// applyCliOverrides applies CLI flag overrides to the configuration
func applyCliOverrides(cfg *Config, inputDir string, outputFile string, inlineMaps []string) {
	// Override with CLI flags
	if inputDir != "" {
		cfg.Input = inputDir
	}
	if outputFile != "" {
		cfg.Output = outputFile
	}
	if len(inlineMaps) > 0 {
		if cfg.Mappings == nil {
			cfg.Mappings = make(map[string]string)
		}
		for _, m := range inlineMaps {
			parts := splitMap(m)
			if parts != nil {
				cfg.Mappings[parts[0]] = parts[1]
			}
		}
	}
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
