package transform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/developerkunal/OpenMorph/internal/config"
	"github.com/developerkunal/OpenMorph/internal/pagination"
)

// parseYAMLToNode parses YAML content into a yaml.Node and returns the content part
func parseYAMLToNode(t *testing.T, yamlContent string) *yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// When YAML is unmarshaled into a yaml.Node, the result is a DocumentNode
	// that contains the actual content in its Content[0]
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}

	return &doc
}

func TestProcessVendorExtensionsInDir(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) (string, VendorExtensionOptions)
		expectChanged bool
		expectFiles   int
		expectError   bool
	}{
		{
			name: "disabled vendor extensions",
			setup: func(t *testing.T) (string, VendorExtensionOptions) {
				dir := t.TempDir()
				opts := VendorExtensionOptions{
					VendorExtensions: config.VendorExtensions{
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
			name: "no providers configured",
			setup: func(t *testing.T) (string, VendorExtensionOptions) {
				dir := t.TempDir()
				opts := VendorExtensionOptions{
					VendorExtensions: config.VendorExtensions{
						Enabled:   true,
						Providers: map[string]config.ProviderConfig{},
					},
				}
				return dir, opts
			},
			expectChanged: false,
			expectFiles:   0,
			expectError:   false,
		},
		{
			name: "valid openapi file with pagination",
			setup: func(t *testing.T) (string, VendorExtensionOptions) {
				dir := t.TempDir()

				// Create a test OpenAPI file with pagination
				openAPIContent := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      parameters:
        - name: cursor
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
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

				opts := VendorExtensionOptions{
					Options: Options{
						DryRun: true, // Use dry run for testing
					},
					VendorExtensions: config.VendorExtensions{
						Enabled: true,
						Providers: map[string]config.ProviderConfig{
							"fern": {
								ExtensionName: "x-fern-pagination",
								TargetLevel:   "operation",
								Methods:       []string{"get"},
								FieldMapping: config.FieldMapping{
									RequestParams: map[string][]string{
										"cursor": {"cursor", "next_cursor"},
										"limit":  {"limit", "size"},
									},
								},
								Strategies: map[string]config.StrategyConfig{
									"cursor": {
										Template: map[string]interface{}{
											"type":            "cursor",
											"cursor_param":    "$request.{cursor_param}",
											"page_size_param": "$request.{limit_param}",
											"results_path":    "$response.{results_field}",
										},
										RequiredFields: []string{"cursor_param", "results_field"},
									},
								},
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

			result, err := ProcessVendorExtensionsInDir(dir, opts)

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

func TestOperationMatchesProvider(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		pathName  string
		config    config.ProviderConfig
		expected  bool
	}{
		{
			name:      "matches method and path",
			operation: "get",
			pathName:  "/api/users",
			config: config.ProviderConfig{
				Methods:      []string{"get", "post"},
				PathPatterns: []string{"/api/*"},
			},
			expected: true,
		},
		{
			name:      "method mismatch",
			operation: "post",
			pathName:  "/api/users",
			config: config.ProviderConfig{
				Methods:      []string{"get"},
				PathPatterns: []string{"/api/*"},
			},
			expected: false,
		},
		{
			name:      "path mismatch",
			operation: "get",
			pathName:  "/other/path",
			config: config.ProviderConfig{
				Methods:      []string{"get"},
				PathPatterns: []string{"/api/*"},
			},
			expected: false,
		},
		{
			name:      "no restrictions - matches all",
			operation: "post",
			pathName:  "/any/path",
			config:    config.ProviderConfig{},
			expected:  true,
		},
		{
			name:      "case insensitive method matching",
			operation: "GET",
			pathName:  "/api/users",
			config: config.ProviderConfig{
				Methods: []string{"get"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := operationMatchesProvider(tt.operation, tt.pathName, tt.config)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildTemplateContext(t *testing.T) {
	tests := []struct {
		name           string
		paginationInfo pagination.DetectedPagination
		config         config.ProviderConfig
		params         string // YAML content for parameters
		responses      string // YAML content for responses
		expected       map[string]string
	}{
		{
			name: "cursor pagination with auto-detected results field",
			paginationInfo: pagination.DetectedPagination{
				Strategy: "cursor",
			},
			config: config.ProviderConfig{
				FieldMapping: config.FieldMapping{
					RequestParams: map[string][]string{
						"cursor": {"cursor", "next_cursor"},
						"limit":  {"limit", "size"},
					},
				},
			},
			params: `
- name: cursor
  in: query
  schema:
    type: string
- name: limit
  in: query
  schema:
    type: integer
`,
			responses: `
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
`,
			expected: map[string]string{
				"cursor_param":  "cursor",
				"limit_param":   "limit",
				"results_field": "data",
			},
		},
		{
			name: "no matching parameters",
			paginationInfo: pagination.DetectedPagination{
				Strategy: "offset",
			},
			config: config.ProviderConfig{
				FieldMapping: config.FieldMapping{
					RequestParams: map[string][]string{
						"offset": {"offset", "skip"},
					},
				},
			},
			params: `
- name: page
  in: query
  schema:
    type: integer
`,
			responses: `
"200":
  description: Success
  content:
    application/json:
      schema:
        type: object
`,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML content into proper yaml.Node structures
			var paramsNode, responsesNode *yaml.Node

			if tt.params != "" {
				paramsNode = parseYAMLToNode(t, tt.params)
			}

			if tt.responses != "" {
				responsesNode = parseYAMLToNode(t, tt.responses)
			}

			result := buildTemplateContext(tt.paginationInfo, tt.config, paramsNode, responsesNode, nil)

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("expected %s=%s, got %s", key, expectedValue, result[key])
				}
			}

			// Check that we don't have unexpected keys
			for key := range result {
				if _, exists := tt.expected[key]; !exists {
					t.Errorf("unexpected key in result: %s=%s", key, result[key])
				}
			}
		})
	}
}

func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		context  map[string]string
		expected string
	}{
		{
			name:     "substitute request parameter",
			template: "$request.{cursor_param}",
			context: map[string]string{
				"cursor_param": "cursor",
			},
			expected: "$request.cursor",
		},
		{
			name:     "substitute response field",
			template: "$response.{results_field}",
			context: map[string]string{
				"results_field": "data",
			},
			expected: "$response.data",
		},
		{
			name:     "multiple substitutions",
			template: "Get data from $response.{results_field} using $request.{cursor_param}",
			context: map[string]string{
				"results_field": "items",
				"cursor_param":  "next_cursor",
			},
			expected: "Get data from $response.items using $request.next_cursor",
		},
		{
			name:     "no matching context",
			template: "$request.{missing_param}",
			context: map[string]string{
				"other_param": "value",
			},
			expected: "$request.{missing_param}",
		},
		{
			name:     "no substitution needed",
			template: "static string",
			context: map[string]string{
				"param": "value",
			},
			expected: "static string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteTemplate(tt.template, tt.context)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProcessTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template map[string]interface{}
		context  map[string]string
		expected map[string]interface{}
	}{
		{
			name: "process string values",
			template: map[string]interface{}{
				"type":            "cursor",
				"cursor_param":    "$request.{cursor_param}",
				"page_size_param": "$request.{limit_param}",
				"results_path":    "$response.{results_field}",
				"count":           42, // non-string value
			},
			context: map[string]string{
				"cursor_param":  "cursor",
				"limit_param":   "limit",
				"results_field": "data",
			},
			expected: map[string]interface{}{
				"type":            "cursor",
				"cursor_param":    "$request.cursor",
				"page_size_param": "$request.limit",
				"results_path":    "$response.data",
				"count":           42,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processTemplate(tt.template, tt.context)

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("expected %s=%v, got %v", key, expectedValue, result[key])
				}
			}
		})
	}
}

func TestHasRequiredFields(t *testing.T) {
	tests := []struct {
		name           string
		context        map[string]string
		requiredFields []string
		expected       bool
	}{
		{
			name: "all required fields present",
			context: map[string]string{
				"cursor_param":  "cursor",
				"results_field": "data",
				"limit_param":   "limit",
			},
			requiredFields: []string{"cursor_param", "results_field"},
			expected:       true,
		},
		{
			name: "missing required field",
			context: map[string]string{
				"cursor_param": "cursor",
			},
			requiredFields: []string{"cursor_param", "results_field"},
			expected:       false,
		},
		{
			name:           "no required fields",
			context:        map[string]string{"any": "value"},
			requiredFields: []string{},
			expected:       true,
		},
		{
			name:           "empty context with required fields",
			context:        map[string]string{},
			requiredFields: []string{"required"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRequiredFields(tt.context, tt.requiredFields)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			slice:    []string{"a", "b", "c"},
			item:     "b",
			expected: true,
		},
		{
			name:     "item does not exist",
			slice:    []string{"a", "b", "c"},
			item:     "d",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "a",
			expected: false,
		},
		{
			name:     "case sensitive",
			slice:    []string{"Hello"},
			item:     "hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{
			name:     "wildcard pattern matches",
			path:     "/api/v1/users",
			pattern:  "/api/v1/*",
			expected: true,
		},
		{
			name:     "wildcard pattern does not match",
			path:     "/other/path",
			pattern:  "/api/v1/*",
			expected: false,
		},
		{
			name:     "exact match",
			path:     "/api/users",
			pattern:  "/api/users",
			expected: true,
		},
		{
			name:     "exact no match",
			path:     "/api/users",
			pattern:  "/api/posts",
			expected: false,
		},
		{
			name:     "partial prefix without wildcard",
			path:     "/api/v1/users",
			pattern:  "/api/v1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := globMatch(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractParameterNames(t *testing.T) {
	tests := []struct {
		name     string
		params   string // YAML content
		expected []string
	}{
		{
			name: "multiple parameters",
			params: `
- name: cursor
  in: query
  schema:
    type: string
- name: limit
  in: query
  schema:
    type: integer
`,
			expected: []string{"cursor", "limit"},
		},
		{
			name:     "no parameters",
			params:   `[]`,
			expected: []string{},
		},
		{
			name: "parameter with reference",
			params: `
- $ref: "#/components/parameters/CursorParam"
- name: limit
  in: query
  schema:
    type: integer
`,
			expected: []string{"limit"}, // $ref resolution would need root document
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramsNode := parseYAMLToNode(t, tt.params)

			result := extractParameterNames(paramsNode, nil)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d parameters, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("expected parameter %d to be %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestAddExtensionToOperation(t *testing.T) {
	tests := []struct {
		name        string
		operation   string // YAML content for operation
		extension   string
		value       map[string]interface{}
		expectAdded bool
	}{
		{
			name: "add extension to operation",
			operation: `
summary: Get users
parameters:
  - name: limit
    in: query
responses:
  "200":
    description: Success
`,
			extension: "x-fern-pagination",
			value: map[string]interface{}{
				"type": "cursor",
			},
			expectAdded: true,
		},
		{
			name: "extension already exists",
			operation: `
summary: Get users
x-fern-pagination:
  type: cursor
responses:
  "200":
    description: Success
`,
			extension: "x-fern-pagination",
			value: map[string]interface{}{
				"type": "offset",
			},
			expectAdded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operationNode := parseYAMLToNode(t, tt.operation)

			result := addExtensionToOperation(operationNode, tt.extension, tt.value)

			if result != tt.expectAdded {
				t.Errorf("expected addExtensionToOperation to return %v, got %v", tt.expectAdded, result)
			}

			// If we expected the extension to be added, verify it's there
			if tt.expectAdded {
				found := false
				for i := 0; i < len(operationNode.Content); i += 2 {
					if operationNode.Content[i].Value == tt.extension {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected extension %s to be added to operation", tt.extension)
				}
			}
		})
	}
}

func TestIsSuccessResponse(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"2xx success", "200", true},
		{"201 created", "201", true},
		{"3xx redirect", "301", true},
		{"4xx client error", "404", false},
		{"5xx server error", "500", false},
		{"default response", "default", true},
		{"invalid code", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSuccessResponse(tt.code)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCreateYAMLNodeFromMap(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
	}{
		{
			name: "simple map",
			data: map[string]interface{}{
				"type":   "cursor",
				"param":  "$request.cursor",
				"count":  42,
				"active": true,
			},
		},
		{
			name: "empty map",
			data: map[string]interface{}{},
		},
		{
			name: "nested map",
			data: map[string]interface{}{
				"pagination": map[string]interface{}{
					"type":  "cursor",
					"param": "$request.cursor",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createYAMLNodeFromMap(tt.data)

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Kind != yaml.MappingNode {
				t.Errorf("expected MappingNode, got %v", result.Kind)
			}

			// Verify the structure by checking key-value pairs
			expectedPairs := len(tt.data) * 2 // Each key-value pair becomes 2 nodes
			if len(result.Content) != expectedPairs {
				t.Errorf("expected %d content nodes, got %d", expectedPairs, len(result.Content))
			}
		})
	}
}

func TestCreateYAMLNodeFromMapOrdering(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected []string // expected order of keys
	}{
		{
			name: "pagination fields in correct order",
			input: map[string]interface{}{
				"results": "$response.data",
				"offset":  "$request.page",
				"limit":   "$request.per_page",
			},
			expected: []string{"offset", "results", "limit"},
		},
		{
			name: "pagination fields with cursor",
			input: map[string]interface{}{
				"cursor":  "$request.cursor",
				"results": "$response.items",
				"next":    "$response.next_cursor",
			},
			expected: []string{"cursor", "results", "next"},
		},
		{
			name: "mixed known and unknown fields",
			input: map[string]interface{}{
				"unknown_field": "value",
				"results":       "$response.data",
				"offset":        "$request.page",
				"custom":        "custom_value",
			},
			expected: []string{"offset", "results", "custom", "unknown_field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := createYAMLNodeFromMap(tt.input)

			// Extract the key order from the YAML node
			var actualKeys []string
			for i := 0; i < len(node.Content); i += 2 {
				actualKeys = append(actualKeys, node.Content[i].Value)
			}

			// Verify the order matches expected
			if len(actualKeys) != len(tt.expected) {
				t.Errorf("Expected %d keys, got %d", len(tt.expected), len(actualKeys))
				return
			}

			for i, expectedKey := range tt.expected {
				if actualKeys[i] != expectedKey {
					t.Errorf("At position %d: expected key %q, got %q", i, expectedKey, actualKeys[i])
				}
			}
		})
	}
}

func TestAddProcessedExtension(t *testing.T) {
	result := &VendorExtensionResult{
		AddedExtensions: make(map[string][]string),
	}

	addProcessedExtension(result, "file1.yaml", "extension1")
	addProcessedExtension(result, "file1.yaml", "extension2")
	addProcessedExtension(result, "file2.yaml", "extension3")

	if len(result.AddedExtensions["file1.yaml"]) != 2 {
		t.Errorf("expected 2 extensions for file1.yaml, got %d", len(result.AddedExtensions["file1.yaml"]))
	}

	if len(result.AddedExtensions["file2.yaml"]) != 1 {
		t.Errorf("expected 1 extension for file2.yaml, got %d", len(result.AddedExtensions["file2.yaml"]))
	}
}

func TestAddSkippedOperation(t *testing.T) {
	result := &VendorExtensionResult{
		SkippedOperations: make(map[string][]string),
	}

	addSkippedOperation(result, "file1.yaml", "GET /users", "no pagination detected")
	addSkippedOperation(result, "file1.yaml", "POST /users", "method not supported")

	if len(result.SkippedOperations["file1.yaml"]) != 2 {
		t.Errorf("expected 2 skipped operations for file1.yaml, got %d", len(result.SkippedOperations["file1.yaml"]))
	}

	expected := "GET /users: no pagination detected"
	if result.SkippedOperations["file1.yaml"][0] != expected {
		t.Errorf("expected %q, got %q", expected, result.SkippedOperations["file1.yaml"][0])
	}
}

func TestAddVendorExtension(t *testing.T) {
	tests := []struct {
		name           string
		operationYAML  string
		paginationInfo pagination.DetectedPagination
		config         config.ProviderConfig
		paramsYAML     string
		responsesYAML  string
		expectAdded    bool
	}{
		{
			name: "successful extension addition",
			operationYAML: `
summary: Get users
responses:
  "200":
    description: Success
`,
			paginationInfo: pagination.DetectedPagination{
				Strategy: "cursor",
			},
			config: config.ProviderConfig{
				ExtensionName: "x-fern-pagination",
				FieldMapping: config.FieldMapping{
					RequestParams: map[string][]string{
						"cursor": {"cursor"},
						"limit":  {"limit"},
					},
				},
				Strategies: map[string]config.StrategyConfig{
					"cursor": {
						Template: map[string]interface{}{
							"type":         "cursor",
							"cursor_param": "$request.{cursor_param}",
							"results_path": "$response.{results_field}",
						},
						RequiredFields: []string{"cursor_param", "results_field"},
					},
				},
			},
			paramsYAML: `
- name: cursor
  in: query
  schema:
    type: string
`,
			responsesYAML: `
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
`,
			expectAdded: true,
		},
		{
			name: "missing required fields",
			operationYAML: `
summary: Get users
responses:
  "200":
    description: Success
`,
			paginationInfo: pagination.DetectedPagination{
				Strategy: "cursor",
			},
			config: config.ProviderConfig{
				ExtensionName: "x-fern-pagination",
				FieldMapping: config.FieldMapping{
					RequestParams: map[string][]string{
						"offset": {"offset"}, // Different param name
					},
				},
				Strategies: map[string]config.StrategyConfig{
					"cursor": {
						Template: map[string]interface{}{
							"type":         "cursor",
							"cursor_param": "$request.{cursor_param}",
						},
						RequiredFields: []string{"cursor_param"}, // Required but not found
					},
				},
			},
			paramsYAML: `
- name: cursor
  in: query
  schema:
    type: string
`,
			responsesYAML: `
"200":
  description: Success
`,
			expectAdded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operationNode := parseYAMLToNode(t, tt.operationYAML)
			paramsNode := parseYAMLToNode(t, tt.paramsYAML)
			responsesNode := parseYAMLToNode(t, tt.responsesYAML)

			result := addVendorExtension(operationNode, tt.paginationInfo, tt.config, paramsNode, responsesNode, nil)

			if result != tt.expectAdded {
				t.Errorf("expected %v, got %v", tt.expectAdded, result)
			}

			if tt.expectAdded {
				// Verify the extension was actually added
				found := false
				for i := 0; i < len(operationNode.Content); i += 2 {
					if operationNode.Content[i].Value == tt.config.ExtensionName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected extension %s to be added", tt.config.ExtensionName)
				}
			}
		})
	}
}

func TestGetVendorNodeValue(t *testing.T) {
	yamlContent := `
openapi: 3.0.0
info:
  title: Test API
paths:
  /users:
    get:
      summary: Get users
`

	rootNode := parseYAMLToNode(t, yamlContent)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "find openapi version",
			key:      "openapi",
			expected: "3.0.0",
		},
		{
			name:     "find paths",
			key:      "paths",
			expected: "", // Will return the node, not string value
		},
		{
			name:     "missing key",
			key:      "nonexistent",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVendorNodeValue(rootNode, tt.key)

			if tt.key == "openapi" {
				if result == nil || result.Value != tt.expected {
					t.Errorf("expected %s, got %v", tt.expected, result)
				}
			} else if tt.key == "paths" {
				if result == nil {
					t.Errorf("expected paths node to be found")
				}
			} else if tt.key == "nonexistent" {
				if result != nil {
					t.Errorf("expected nil for nonexistent key")
				}
			}
		})
	}
}

func TestGetVendorStringValue(t *testing.T) {
	yamlContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
`

	rootNode := parseYAMLToNode(t, yamlContent)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "get openapi version",
			key:      "openapi",
			expected: "3.0.0",
		},
		{
			name:     "missing key",
			key:      "nonexistent",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVendorStringValue(rootNode, tt.key)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWriteVendorExtensionsDocument(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create a test document
	yamlContent := `
openapi: 3.0.0
info:
  title: Test API
`
	doc := parseYAMLToNode(t, yamlContent)

	tests := []struct {
		name         string
		dryRun       bool
		expectWrite  bool
		expectChange bool
	}{
		{
			name:         "dry run mode",
			dryRun:       true,
			expectWrite:  false,
			expectChange: true,
		},
		{
			name:         "actual write",
			dryRun:       false,
			expectWrite:  true,
			expectChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the original file
			if err := os.WriteFile(testFile, []byte(yamlContent), 0600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
			defer os.Remove(testFile)

			// Create a full document node structure
			fullDoc := &yaml.Node{
				Kind:    yaml.DocumentNode,
				Content: []*yaml.Node{doc},
			}

			changed, err := writeVendorExtensionsDocument(fullDoc, testFile, tt.dryRun)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if changed != tt.expectChange {
				t.Errorf("expected changed=%v, got %v", tt.expectChange, changed)
			}

			// In dry run mode, file should not be modified
			if tt.dryRun {
				content, err := os.ReadFile(testFile)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}
				if string(content) != yamlContent {
					t.Errorf("file was modified in dry run mode")
				}
			}
		})
	}
}

func TestConsistentPaginationOrdering(t *testing.T) {
	// Test the exact case mentioned in the user request
	paginationData := map[string]interface{}{
		"results": "$response.network_acls",
		"offset":  "$request.page",
	}

	// Generate the YAML node multiple times to ensure consistent ordering
	for i := 0; i < 5; i++ {
		node := createYAMLNodeFromMap(paginationData)

		// Convert to YAML string to verify output
		var buffer strings.Builder
		encoder := yaml.NewEncoder(&buffer)
		encoder.SetIndent(2)
		err := encoder.Encode(node)
		if err != nil {
			t.Fatalf("Failed to encode YAML: %v", err)
		}
		encoder.Close()

		yamlOutput := buffer.String()

		// Verify that "offset" appears before "results" in the output
		offsetPos := strings.Index(yamlOutput, "offset:")
		resultsPos := strings.Index(yamlOutput, "results:")

		if offsetPos == -1 || resultsPos == -1 {
			t.Fatalf("Missing expected fields in YAML output: %s", yamlOutput)
		}

		if offsetPos >= resultsPos {
			t.Errorf("Expected 'offset' to appear before 'results' in YAML output, but got:\n%s", yamlOutput)
		}

		// For the first iteration, log the output for visual verification
		if i == 0 {
			t.Logf("Generated YAML output:\n%s", yamlOutput)
		}
	}
}
