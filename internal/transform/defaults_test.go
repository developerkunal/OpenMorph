package transform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/developerkunal/OpenMorph/internal/config"
)

func TestProcessDefaultsInDir(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) (string, DefaultsOptions)
		expectChanged bool
		expectFiles   int
		expectError   bool
	}{
		{
			name: "disabled defaults",
			setup: func(t *testing.T) (string, DefaultsOptions) {
				dir := t.TempDir()
				opts := DefaultsOptions{
					DefaultValues: config.DefaultValues{
						Enabled: false,
					},
				}
				return dir, opts
			},
			expectChanged: false,
			expectFiles:   0,
			expectError:   false,
		},
		{
			name: "no rules configured",
			setup: func(t *testing.T) (string, DefaultsOptions) {
				dir := t.TempDir()
				opts := DefaultsOptions{
					DefaultValues: config.DefaultValues{
						Enabled: true,
						Rules:   map[string]config.DefaultRule{},
					},
				}
				return dir, opts
			},
			expectChanged: false,
			expectFiles:   0,
			expectError:   false,
		},
		{
			name: "parameter defaults applied",
			setup: func(t *testing.T) (string, DefaultsOptions) {
				dir := t.TempDir()

				// Create a test OpenAPI file
				openAPIContent := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
        - name: sort
          in: query
          schema:
            type: string
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      type: object
`

				testFile := filepath.Join(dir, "api.yaml")
				if err := os.WriteFile(testFile, []byte(openAPIContent), 0600); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}

				opts := DefaultsOptions{
					Options: Options{
						DryRun: true, // Use dry run for testing
					},
					DefaultValues: config.DefaultValues{
						Enabled: true,
						Rules: map[string]config.DefaultRule{
							"query_param_defaults": {
								Target: config.DefaultTarget{
									Location: "parameter",
								},
								Condition: config.DefaultCondition{
									ParameterIn: "query",
									Type:        "integer",
								},
								Value:    20,
								Priority: 1,
							},
							"string_param_defaults": {
								Target: config.DefaultTarget{
									Location: "parameter",
								},
								Condition: config.DefaultCondition{
									ParameterIn:  "query",
									Type:         "string",
									PropertyName: "sort",
								},
								Value:    "asc",
								Priority: 2,
							},
						},
					},
				}
				return dir, opts
			},
			expectChanged: true,
			expectFiles:   1,
			expectError:   false,
		},
		{
			name: "component schema defaults applied",
			setup: func(t *testing.T) (string, DefaultsOptions) {
				dir := t.TempDir()

				// Create a test OpenAPI file with components
				openAPIContent := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    User:
      type: object
      properties:
        active:
          type: boolean
        role:
          type: string
        priority:
          type: integer
paths:
  /users:
    get:
      responses:
        "200":
          description: Success
`

				testFile := filepath.Join(dir, "api.yaml")
				if err := os.WriteFile(testFile, []byte(openAPIContent), 0600); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}

				opts := DefaultsOptions{
					Options: Options{
						DryRun: true,
					},
					DefaultValues: config.DefaultValues{
						Enabled: true,
						Rules: map[string]config.DefaultRule{
							"boolean_defaults": {
								Target: config.DefaultTarget{
									Location: "component",
								},
								Condition: config.DefaultCondition{
									Type: "boolean",
								},
								Value:    true,
								Priority: 1,
							},
							"role_defaults": {
								Target: config.DefaultTarget{
									Location: "component",
								},
								Condition: config.DefaultCondition{
									Type:         "string",
									PropertyName: "role",
								},
								Value:    "user",
								Priority: 2,
							},
						},
					},
				}
				return dir, opts
			},
			expectChanged: true,
			expectFiles:   1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, opts := tt.setup(t)

			result, err := ProcessDefaultsInDir(dir, opts)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if result.Changed != tt.expectChanged {
				t.Errorf("expected Changed=%v, got %v", tt.expectChanged, result.Changed)
			}

			if len(result.ProcessedFiles) != tt.expectFiles {
				t.Errorf("expected %d processed files, got %d", tt.expectFiles, len(result.ProcessedFiles))
			}
		})
	}
}

func TestMatchesPathPattern(t *testing.T) {
	tests := []struct {
		path     string
		patterns []string
		expected bool
	}{
		{"/api/v1/users", []string{}, true},              // No patterns means match all
		{"/api/v1/users", []string{"/api/.*"}, true},     // Regex match
		{"/api/v1/users", []string{"/api/v1/.*"}, true},  // Specific path match
		{"/api/v2/users", []string{"/api/v1/.*"}, false}, // No match
		{"/users", []string{"/api/.*", "/users"}, true},  // Multiple patterns, one matches
		{"/other", []string{"/api/.*", "/users"}, false}, // No pattern matches
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := matchesPathPattern(tt.path, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesPathPattern(%q, %v) = %v, expected %v", tt.path, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestMatchesHTTPMethod(t *testing.T) {
	tests := []struct {
		method   string
		methods  []string
		expected bool
	}{
		{"get", []string{}, true},                  // No methods means match all
		{"get", []string{"get"}, true},             // Exact match
		{"GET", []string{"get"}, true},             // Case insensitive
		{"post", []string{"get", "post"}, true},    // Multiple methods, one matches
		{"delete", []string{"get", "post"}, false}, // No match
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := matchesHTTPMethod(tt.method, tt.methods)
			if result != tt.expected {
				t.Errorf("matchesHTTPMethod(%q, %v) = %v, expected %v", tt.method, tt.methods, result, tt.expected)
			}
		})
	}
}

func TestMatchesPropertyName(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"active", "", true},         // No pattern means match all
		{"active", "active", true},   // Exact match
		{"active", "act.*", true},    // Regex match
		{"status", "act.*", false},   // No match
		{"user_id", ".*_id", true},   // Suffix match
		{"username", ".*_id", false}, // No suffix match
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPropertyName(tt.name, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPropertyName(%q, %q) = %v, expected %v", tt.name, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestCreateDefaultValueNode(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "test", "test"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"float", 3.14, "3.14"},
		{"array", []interface{}{"a", "b"}, ""},              // Arrays need special handling
		{"map", map[string]interface{}{"key": "value"}, ""}, // Maps need special handling
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := createDefaultValueNode(tt.value)
			if tt.expected != "" {
				if node == nil {
					t.Errorf("expected node for %v, got nil", tt.value)
				} else if node.Value != tt.expected {
					t.Errorf("expected node value %q, got %q", tt.expected, node.Value)
				}
			}
		})
	}
}

func TestShouldApplyDefaultToProperty(t *testing.T) {
	tests := []struct {
		name        string
		propYAML    string
		propName    string
		rule        config.DefaultRule
		expectApply bool
	}{
		{
			name: "type matches",
			propYAML: `
type: string
`,
			propName: "test",
			rule: config.DefaultRule{
				Condition: config.DefaultCondition{
					Type: "string",
				},
			},
			expectApply: true,
		},
		{
			name: "type doesn't match",
			propYAML: `
type: integer
`,
			propName: "test",
			rule: config.DefaultRule{
				Condition: config.DefaultCondition{
					Type: "string",
				},
			},
			expectApply: false,
		},
		{
			name: "default already exists",
			propYAML: `
type: string
default: existing
`,
			propName: "test",
			rule: config.DefaultRule{
				Condition: config.DefaultCondition{
					Type: "string",
				},
			},
			expectApply: false,
		},
		{
			name: "property name matches pattern",
			propYAML: `
type: string
`,
			propName: "user_id",
			rule: config.DefaultRule{
				Condition: config.DefaultCondition{
					Type:         "string",
					PropertyName: ".*_id",
				},
			},
			expectApply: true,
		},
		{
			name: "property name doesn't match pattern",
			propYAML: `
type: string
`,
			propName: "username",
			rule: config.DefaultRule{
				Condition: config.DefaultCondition{
					Type:         "string",
					PropertyName: ".*_id",
				},
			},
			expectApply: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML manually to get the root content node
			var doc yaml.Node
			err := yaml.Unmarshal([]byte(strings.TrimSpace(tt.propYAML)), &doc)
			if err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			// Get the actual content node (not document node)
			var propSchema *yaml.Node
			if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
				propSchema = doc.Content[0]
			} else {
				propSchema = &doc
			}

			result := &DefaultsResult{
				SkippedTargets: make(map[string][]string),
			}

			shouldApply := shouldApplyDefaultToProperty(propSchema, tt.propName, tt.rule, "test context", "test.yaml", result)

			if shouldApply != tt.expectApply {
				t.Errorf("expected shouldApplyDefaultToProperty=%v, got %v", tt.expectApply, shouldApply)
				t.Logf("SkippedTargets: %v", result.SkippedTargets)
			}
		})
	}
}

func TestGetSortedDefaultRules(t *testing.T) {
	rules := map[string]config.DefaultRule{
		"low_priority": {
			Priority: 1,
		},
		"high_priority": {
			Priority: 10,
		},
		"medium_priority": {
			Priority: 5,
		},
	}

	sorted := getSortedDefaultRules(rules)

	if len(sorted) != 3 {
		t.Errorf("expected 3 rules, got %d", len(sorted))
	}

	// Should be sorted by priority (highest first)
	if sorted[0].Name != "high_priority" {
		t.Errorf("expected first rule to be 'high_priority', got %q", sorted[0].Name)
	}
	if sorted[1].Name != "medium_priority" {
		t.Errorf("expected second rule to be 'medium_priority', got %q", sorted[1].Name)
	}
	if sorted[2].Name != "low_priority" {
		t.Errorf("expected third rule to be 'low_priority', got %q", sorted[2].Name)
	}
}
