package pagination

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDetectPaginationInParams(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string // expected strategy names
	}{
		{
			name: "offset pagination",
			yaml: `
- name: offset
  in: query
  schema:
    type: integer
- name: limit
  in: query
  schema:
    type: integer
`,
			expected: []string{"offset"},
		},
		{
			name: "checkpoint pagination",
			yaml: `
- name: from
  in: query
  schema:
    type: string
- name: take
  in: query
  schema:
    type: integer
`,
			expected: []string{"checkpoint"},
		},
		{
			name: "mixed pagination",
			yaml: `
- name: offset
  in: query
  schema:
    type: integer
- name: from
  in: query
  schema:
    type: string
- name: page
  in: query
  schema:
    type: integer
`,
			expected: []string{"offset", "checkpoint", "page"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			// Get the actual content node (skip document wrapper)
			contentNode := &node
			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				contentNode = node.Content[0]
			}

			detected := DetectPaginationInParams(contentNode)

			if len(detected) != len(tt.expected) {
				t.Errorf("Expected %d strategies, got %d", len(tt.expected), len(detected))
				return
			}

			detectedStrategies := make(map[string]bool)
			for _, d := range detected {
				detectedStrategies[d.Strategy] = true
			}

			for _, expected := range tt.expected {
				if !detectedStrategies[expected] {
					t.Errorf("Expected strategy %s not found", expected)
				}
			}
		})
	}
}

func TestDetectPaginationInResponses(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name: "offset response",
			yaml: `
"200":
  content:
    application/json:
      schema:
        type: object
        properties:
          total:
            type: integer
          offset:
            type: integer
          users:
            type: array
            items:
              $ref: "#/components/schemas/User"
`,
			expected: []string{"offset", "page"}, // total field matches both strategies
		},
		{
			name: "checkpoint response",
			yaml: `
"200":
  content:
    application/json:
      schema:
        type: object
        properties:
          next:
            type: string
          users:
            type: array
            items:
              $ref: "#/components/schemas/User"
`,
			expected: []string{"checkpoint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			// Get the actual content node (skip document wrapper)
			contentNode := &node
			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				contentNode = node.Content[0]
			}

			detected := DetectPaginationInResponses(contentNode)

			if len(detected) != len(tt.expected) {
				t.Errorf("Expected %d strategies, got %d", len(tt.expected), len(detected))
				return
			}

			detectedStrategies := make(map[string]bool)
			for _, d := range detected {
				detectedStrategies[d.Strategy] = true
			}

			for _, expected := range tt.expected {
				if !detectedStrategies[expected] {
					t.Errorf("Expected strategy %s not found", expected)
				}
			}
		})
	}
}

func TestProcessEndpoint(t *testing.T) {
	tests := []struct {
		name                string
		yaml                string
		priority            []string
		expectChanged       bool
		expectRemovedParams int
	}{
		{
			name: "prefer checkpoint over offset",
			yaml: `
parameters:
  - name: offset
    in: query
    schema:
      type: integer
  - name: from
    in: query
    schema:
      type: string
responses:
  "200":
    content:
      application/json:
        schema:
          oneOf:
            - type: object
              properties:
                total:
                  type: integer
                users:
                  type: array
                  items:
                    $ref: "#/components/schemas/User"
            - type: object
              properties:
                next:
                  type: string
                users:
                  type: array
                  items:
                    $ref: "#/components/schemas/User"
`,
			priority:            []string{"checkpoint", "offset"},
			expectChanged:       true,
			expectRemovedParams: 1, // offset should be removed
		},
		{
			name: "single pagination strategy",
			yaml: `
parameters:
  - name: offset
    in: query
    schema:
      type: integer
responses:
  "200":
    content:
      application/json:
        schema:
          type: object
          properties:
            total:
              type: integer
            users:
              type: array
              items:
                $ref: "#/components/schemas/User"
`,
			priority:            []string{"checkpoint", "offset"},
			expectChanged:       false, // single strategy with compatible response - no changes needed
			expectRemovedParams: 0,     // no params should be removed since offset is selected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			// Get the actual content node (skip document wrapper)
			contentNode := &node
			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				contentNode = node.Content[0]
			}

			opts := Options{Priority: tt.priority}
			result, err := ProcessEndpoint(contentNode, opts)
			if err != nil {
				t.Fatalf("ProcessEndpoint failed: %v", err)
			}

			if result.Changed != tt.expectChanged {
				t.Errorf("Expected changed=%v, got %v", tt.expectChanged, result.Changed)
			}

			if len(result.RemovedParams) != tt.expectRemovedParams {
				t.Errorf("Expected %d removed params, got %d", tt.expectRemovedParams, len(result.RemovedParams))
			}
		})
	}
}

func TestMatchesParam(t *testing.T) {
	tests := []struct {
		paramName     string
		strategyParam string
		expected      bool
	}{
		{"offset", "offset", true},
		{"OFFSET", "offset", true},
		{"Offset", "offset", true},
		{"limit", "offset", false},
		{"from", "from", true},
	}

	for _, tt := range tests {
		result := matchesParam(tt.paramName, tt.strategyParam)
		if result != tt.expected {
			t.Errorf("matchesParam(%q, %q) = %v, expected %v",
				tt.paramName, tt.strategyParam, result, tt.expected)
		}
	}
}

func TestMatchesField(t *testing.T) {
	tests := []struct {
		fieldName     string
		strategyField string
		expected      bool
	}{
		{"total", "total", true},
		{"TOTAL", "total", true},
		{"Total", "total", true},
		{"count", "total", false},
		{"next", "next", true},
	}

	for _, tt := range tests {
		result := matchesField(tt.fieldName, tt.strategyField)
		if result != tt.expected {
			t.Errorf("matchesField(%q, %q) = %v, expected %v",
				tt.fieldName, tt.strategyField, result, tt.expected)
		}
	}
}

func TestIsSuccessResponse(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"200", true},
		{"201", true},
		{"206", true},
		{"300", true},
		{"400", false},
		{"404", false},
		{"500", false},
		{"default", true},
	}

	for _, tt := range tests {
		result := isSuccessResponse(tt.code)
		if result != tt.expected {
			t.Errorf("isSuccessResponse(%q) = %v, expected %v",
				tt.code, result, tt.expected)
		}
	}
}

func TestExtractFieldsFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name: "simple properties",
			yaml: `
type: object
properties:
  total:
    type: integer
  offset:
    type: integer
  users:
    type: array
`,
			expected: []string{"total", "offset", "users"},
		},
		{
			name: "oneOf schema",
			yaml: `
oneOf:
  - type: object
    properties:
      total:
        type: integer
  - type: object
    properties:
      next:
        type: string
`,
			expected: []string{"total", "next"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			// Get the actual content node (skip document wrapper)
			contentNode := &node
			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				contentNode = node.Content[0]
			}

			fields := extractFieldsFromSchema(contentNode)

			if len(fields) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d: %v", len(tt.expected), len(fields), fields)
				return
			}

			fieldMap := make(map[string]bool)
			for _, field := range fields {
				fieldMap[field] = true
			}

			for _, expected := range tt.expected {
				if !fieldMap[expected] {
					t.Errorf("Expected field %s not found in %v", expected, fields)
				}
			}
		})
	}
}

// Integration test with a complete OpenAPI operation
func TestCompleteOperation(t *testing.T) {
	operationYAML := `
get:
  parameters:
    - name: offset
      in: query
      schema:
        type: integer
    - name: limit
      in: query
      schema:
        type: integer
    - name: from
      in: query
      schema:
        type: string
    - name: take
      in: query
      schema:
        type: integer
    - name: include_totals
      in: query
      schema:
        type: boolean
  responses:
    "200":
      content:
        application/json:
          schema:
            oneOf:
              - type: object
                properties:
                  total:
                    type: integer
                  offset:
                    type: integer
                  users:
                    type: array
                    items:
                      $ref: "#/components/schemas/User"
              - type: object
                properties:
                  next:
                    type: string
                  users:
                    type: array
                    items:
                      $ref: "#/components/schemas/User"
`

	var operation yaml.Node
	err := yaml.Unmarshal([]byte(operationYAML), &operation)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	// Get the actual content node (skip document wrapper if exists)
	contentNode := &operation
	if operation.Kind == yaml.DocumentNode && len(operation.Content) > 0 {
		contentNode = operation.Content[0]
	}

	// Get the actual operation node (skip the "get" key)
	var operationNode *yaml.Node
	if contentNode.Kind == yaml.MappingNode && len(contentNode.Content) >= 2 {
		operationNode = contentNode.Content[1]
	} else {
		t.Fatal("Invalid operation structure")
	}

	opts := Options{Priority: []string{"checkpoint", "offset", "page"}}
	result, err := ProcessEndpoint(operationNode, opts)
	if err != nil {
		t.Fatalf("ProcessEndpoint failed: %v", err)
	}

	if !result.Changed {
		t.Error("Expected operation to be changed")
	}

	// Should remove offset and limit params (keeping checkpoint: from, take)
	expectedRemovedParams := []string{"offset", "limit", "include_totals"}
	if len(result.RemovedParams) != len(expectedRemovedParams) {
		t.Errorf("Expected %d removed params, got %d: %v",
			len(expectedRemovedParams), len(result.RemovedParams), result.RemovedParams)
	}

	// Check that checkpoint params are still there
	params := getNodeValue(operationNode, "parameters")
	if params == nil {
		t.Fatal("Parameters node not found")
	}

	var remainingParams []string
	for _, param := range params.Content {
		paramName := getStringValue(param, "name")
		if paramName != "" {
			remainingParams = append(remainingParams, paramName)
		}
	}

	expectedRemaining := []string{"from", "take"}
	if len(remainingParams) != len(expectedRemaining) {
		t.Errorf("Expected %d remaining params, got %d: %v",
			len(expectedRemaining), len(remainingParams), remainingParams)
	}

	for _, expected := range expectedRemaining {
		found := false
		for _, remaining := range remainingParams {
			if remaining == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected remaining param %s not found", expected)
		}
	}
}

func TestSharedParameterHandling(t *testing.T) {
	// Test case where include_totals is shared between offset and page strategies
	// When page is selected, include_totals should NOT be removed
	paramsYAML := `
- name: offset
  in: query
  schema:
    type: integer
- name: limit
  in: query
  schema:
    type: integer
- name: page
  in: query
  schema:
    type: integer
- name: per_page
  in: query
  schema:
    type: integer
- name: include_totals
  in: query
  schema:
    type: boolean
`

	var paramsNode yaml.Node
	err := yaml.Unmarshal([]byte(paramsYAML), &paramsNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal params YAML: %v", err)
	}

	// Get the actual content node (skip document wrapper)
	contentNode := &paramsNode
	if paramsNode.Kind == yaml.DocumentNode && len(paramsNode.Content) > 0 {
		contentNode = paramsNode.Content[0]
	}

	// Detect pagination strategies
	detected := DetectPaginationInParams(contentNode)

	// Should detect both offset and page
	expectedStrategies := map[string]bool{"offset": false, "page": false}
	for _, d := range detected {
		if _, exists := expectedStrategies[d.Strategy]; exists {
			expectedStrategies[d.Strategy] = true
		}
	}

	for strategy, found := range expectedStrategies {
		if !found {
			t.Errorf("Expected strategy %s not detected", strategy)
		}
	}

	// Test parameter removal with page priority
	opts := Options{Priority: []string{"page", "offset", "cursor", "checkpoint", "none"}}

	// Create a mock operation node
	operationYAML := `
parameters:
` + paramsYAML

	var opNode yaml.Node
	err = yaml.Unmarshal([]byte(operationYAML), &opNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	opContentNode := &opNode
	if opNode.Kind == yaml.DocumentNode && len(opNode.Content) > 0 {
		opContentNode = opNode.Content[0]
	}

	result, err := ProcessEndpoint(opContentNode, opts)
	if err != nil {
		t.Fatalf("ProcessEndpoint failed: %v", err)
	}

	// include_totals should NOT be in removed params since it belongs to the selected "page" strategy
	for _, removed := range result.RemovedParams {
		if removed == "include_totals" {
			t.Errorf("include_totals was incorrectly removed - it belongs to the selected 'page' strategy")
		}
	}

	// offset and limit should be removed since they belong only to the "offset" strategy
	expectedRemoved := map[string]bool{"offset": false, "limit": false}
	for _, removed := range result.RemovedParams {
		if _, exists := expectedRemoved[removed]; exists {
			expectedRemoved[removed] = true
		}
	}

	for param, found := range expectedRemoved {
		if !found {
			t.Errorf("Expected parameter %s to be removed but it wasn't", param)
		}
	}

	t.Logf("Removed params: %v", result.RemovedParams)
}

func TestResponseRemoval(t *testing.T) {
	// Test case with mixed pagination in responses
	operationYAML := `
parameters:
- name: page
  in: query
  schema:
    type: integer
- name: offset
  in: query
  schema:
    type: integer
responses:
  '200':
    description: Success
    content:
      application/json:
        schema:
          type: object
          properties:
            data:
              type: array
              items:
                type: string
            offset:
              type: integer
              description: offset pagination field - should be removed
            start:
              type: integer
              description: page pagination field - should be kept
  '400':
    description: Error
    content:
      application/json:
        schema:
          type: object
          properties:
            error:
              type: string
            message:
              type: string
              description: error message, no pagination fields
`

	var opNode yaml.Node
	err := yaml.Unmarshal([]byte(operationYAML), &opNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	opContentNode := &opNode
	if opNode.Kind == yaml.DocumentNode && len(opNode.Content) > 0 {
		opContentNode = opNode.Content[0]
	}

	// Test with page priority (should keep start, remove offset from 200 response)
	opts := Options{Priority: []string{"page", "offset", "cursor", "checkpoint", "none"}}

	result, err := ProcessEndpoint(opContentNode, opts)
	if err != nil {
		t.Fatalf("ProcessEndpoint failed: %v", err)
	}

	t.Logf("Removed params: %v", result.RemovedParams)
	t.Logf("Removed responses: %v", result.RemovedResponses)
	t.Logf("Modified schemas: %v", result.ModifiedSchemas)

	// The 200 response should have been modified to remove the "offset" field
	// since it belongs only to the non-selected "offset" strategy
	if len(result.ModifiedSchemas) == 0 {
		t.Errorf("Expected some schemas to be modified to remove unwanted pagination fields")
	}
}

func TestNoneStrategyHandling(t *testing.T) {
	// Test case where "none" is the highest priority
	// Should remove ALL pagination parameters and response fields
	operationYAML := `
parameters:
- name: page
  in: query
  schema:
    type: integer
- name: per_page
  in: query
  schema:
    type: integer
- name: include_totals
  in: query
  schema:
    type: boolean
- name: non_pagination_param
  in: query
  schema:
    type: string
responses:
  '200':
    description: Success
    content:
      application/json:
        schema:
          type: object
          properties:
            data:
              type: array
              items:
                type: string
            total:
              type: integer
            start:
              type: integer
`

	var opNode yaml.Node
	err := yaml.Unmarshal([]byte(operationYAML), &opNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	opContentNode := &opNode
	if opNode.Kind == yaml.DocumentNode && len(opNode.Content) > 0 {
		opContentNode = opNode.Content[0]
	}

	// Test with "none" as highest priority
	opts := Options{Priority: []string{"none", "page", "offset", "cursor", "checkpoint"}}

	result, err := ProcessEndpoint(opContentNode, opts)
	if err != nil {
		t.Fatalf("ProcessEndpoint failed: %v", err)
	}

	t.Logf("Removed params: %v", result.RemovedParams)
	t.Logf("Removed responses: %v", result.RemovedResponses)
	t.Logf("Modified schemas: %v", result.ModifiedSchemas)

	// When "none" is selected, we might expect different behavior
	// Currently, "none" won't be detected since it has no params/fields
	// So this will test the current behavior
}

func TestNoneStrategySpecialHandling(t *testing.T) {
	// Test the concept that "none" strategy should remove ALL pagination
	// This test explores what the behavior should be

	// This is more of a design question: should "none" be handled specially?
	// Option 1: "none" means "don't detect any pagination, leave endpoint as-is"
	// Option 2: "none" means "remove all pagination from this endpoint"

	// Currently, "none" has empty Params and Fields, so it never gets detected
	// But if it's in the priority list, maybe it should be handled specially

	t.Skip("This test is for exploring the intended behavior of 'none' strategy")
}

func TestEndToEndPaginationPriority(t *testing.T) {
	// Comprehensive test with mixed pagination strategies
	operationYAML := `
parameters:
- name: offset
  in: query
  description: Offset pagination
  schema:
    type: integer
- name: limit
  in: query
  description: Limit for offset pagination
  schema:
    type: integer
- name: include_totals
  in: query
  description: Shared between offset and page
  schema:
    type: boolean
- name: page
  in: query
  description: Page number
  schema:
    type: integer
- name: per_page
  in: query
  description: Items per page
  schema:
    type: integer
- name: cursor
  in: query
  description: Cursor pagination
  schema:
    type: string
- name: api_key
  in: query
  description: Non-pagination parameter
  schema:
    type: string
responses:
  '200':
    description: Success with mixed pagination
    content:
      application/json:
        schema:
          type: object
          properties:
            data:
              type: array
              items:
                type: string
            total:
              type: integer
              description: offset pagination field
            start:
              type: integer
              description: page pagination field
            next_cursor:
              type: string
              description: cursor pagination field
            metadata:
              type: object
              description: Non-pagination field
  '400':
    description: Error response
    content:
      application/json:
        schema:
          type: object
          properties:
            error:
              type: string
            total:
              type: integer
              description: only offset field
`

	testCases := []struct {
		name             string
		priority         []string
		expectedSelected string
		expectedRemoved  []string
		expectedKept     []string
	}{
		{
			name:             "Page priority",
			priority:         []string{"page", "offset", "cursor", "checkpoint", "none"},
			expectedSelected: "page",
			expectedRemoved:  []string{"offset", "limit", "cursor"},
			expectedKept:     []string{"page", "per_page", "include_totals", "api_key"},
		},
		{
			name:             "Cursor priority",
			priority:         []string{"cursor", "page", "offset", "checkpoint", "none"},
			expectedSelected: "cursor",
			expectedRemoved:  []string{"offset", "limit", "page", "per_page", "include_totals"},
			expectedKept:     []string{"cursor", "api_key"},
		},
		{
			name:             "None priority - remove all pagination",
			priority:         []string{"none", "page", "offset", "cursor", "checkpoint"},
			expectedSelected: "none",
			expectedRemoved:  []string{"offset", "limit", "include_totals", "page", "per_page", "cursor"},
			expectedKept:     []string{"api_key"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var opNode yaml.Node
			err := yaml.Unmarshal([]byte(operationYAML), &opNode)
			if err != nil {
				t.Fatalf("Failed to unmarshal operation YAML: %v", err)
			}

			opContentNode := &opNode
			if opNode.Kind == yaml.DocumentNode && len(opNode.Content) > 0 {
				opContentNode = opNode.Content[0]
			}

			// Debug: Let's check what's being detected
			params := getNodeValue(opContentNode, "parameters")
			if params != nil {
				paramPagination := DetectPaginationInParams(params)
				t.Logf("Detected param pagination strategies:")
				for _, d := range paramPagination {
					t.Logf("  Strategy: %s, Parameters: %v", d.Strategy, d.Parameters)
				}
			}

			// Use the same priority as in the user's config
			opts := Options{Priority: tc.priority}

			result, err := ProcessEndpoint(opContentNode, opts)
			if err != nil {
				t.Fatalf("ProcessEndpoint failed: %v", err)
			}

			t.Logf("Priority: %v", tc.priority)
			t.Logf("Removed params: %v", result.RemovedParams)

			// Debug: Let's see which strategy was actually selected
			allParams := []string{"offset", "limit", "include_totals", "page", "per_page", "cursor", "api_key"}
			keptParams := []string{}
			removedMap := make(map[string]bool)
			for _, removed := range result.RemovedParams {
				removedMap[removed] = true
			}
			for _, param := range allParams {
				if !removedMap[param] {
					keptParams = append(keptParams, param)
				}
			}
			t.Logf("Kept params: %v", keptParams)

			// Check that expected parameters were removed
			for _, expected := range tc.expectedRemoved {
				if !removedMap[expected] {
					t.Errorf("Expected parameter %s to be removed but it wasn't", expected)
				}
			}

			// Check that expected parameters were kept (by verifying they're not in removed list)
			for _, expectedKept := range tc.expectedKept {
				if removedMap[expectedKept] {
					t.Errorf("Expected parameter %s to be kept but it was removed", expectedKept)
				}
			}

			// Should have modified schemas since there are response pagination fields
			if len(result.ModifiedSchemas) == 0 {
				t.Errorf("Expected some schemas to be modified for pagination cleanup")
			}
		})
	}
}

func TestClientGrantsScenario(t *testing.T) {
	// Reproduce the exact client-grants scenario from the user's issue
	operationYAML := `
parameters:
- name: per_page
  in: query
  description: Number of results per page.
  schema:
    type: integer
- name: page
  in: query
  description: Page index of the results to return.
  schema:
    type: integer
- name: include_totals
  in: query
  description: Return results inside an object.
  schema:
    type: boolean
- name: from
  in: query
  description: Optional Id from which to start selection.
  schema:
    type: string
- name: take
  in: query
  description: Number of results per page.
  schema:
    type: integer
- name: audience
  in: query
  description: Optional filter on audience.
  schema:
    type: string
- name: client_id
  in: query
  description: Optional filter on client_id.
  schema:
    type: string
`

	var opNode yaml.Node
	err := yaml.Unmarshal([]byte(operationYAML), &opNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	opContentNode := &opNode
	if opNode.Kind == yaml.DocumentNode && len(opNode.Content) > 0 {
		opContentNode = opNode.Content[0]
	}

	// Debug: Let's check what's being detected
	params := getNodeValue(opContentNode, "parameters")
	if params != nil {
		paramPagination := DetectPaginationInParams(params)
		t.Logf("Detected param pagination strategies:")
		for _, d := range paramPagination {
			t.Logf("  Strategy: %s, Parameters: %v", d.Strategy, d.Parameters)
		}
	}

	// Use the same priority as in the user's config
	opts := Options{Priority: []string{"cursor", "offset", "page", "checkpoint", "none"}}

	result, err := ProcessEndpoint(opContentNode, opts)
	if err != nil {
		t.Fatalf("ProcessEndpoint failed: %v", err)
	}

	t.Logf("Detected strategies and priority: %v", opts.Priority)
	t.Logf("Removed params: %v", result.RemovedParams)

	// Debug: Let's see which strategy was actually selected
	allParams := []string{"per_page", "page", "include_totals", "from", "take", "audience", "client_id"}
	keptParams := []string{}
	removedMap := make(map[string]bool)
	for _, removed := range result.RemovedParams {
		removedMap[removed] = true
	}
	for _, param := range allParams {
		if !removedMap[param] {
			keptParams = append(keptParams, param)
		}
	}
	t.Logf("Kept params: %v", keptParams)

	// Expected behavior:
	// - Detected strategies: page (per_page, page, include_totals) and checkpoint (from, take)
	// - Priority order: cursor (not detected), offset (not detected), page (detected), checkpoint (detected)
	// - Selected strategy: "page" (first detected in priority list)
	// - Should keep: per_page, page, include_totals, audience, client_id
	// - Should remove: from, take

	expectedRemoved := map[string]bool{"from": false, "take": false}
	expectedKept := map[string]bool{"per_page": false, "page": false, "include_totals": false}

	// Check that expected params were removed
	for param := range expectedRemoved {
		if !removedMap[param] {
			t.Errorf("Expected parameter %s to be removed but it wasn't", param)
		} else {
			expectedRemoved[param] = true
		}
	}

	// Check that expected params were kept (not in removed list)
	for param := range expectedKept {
		if removedMap[param] {
			t.Errorf("Expected parameter %s to be kept but it was removed", param)
		} else {
			expectedKept[param] = true
		}
	}

	// Verify all expected removals happened
	for param, found := range expectedRemoved {
		if !found {
			t.Errorf("Expected parameter %s to be removed", param)
		}
	}

	// Verify all expected keeps happened
	for param, found := range expectedKept {
		if !found {
			t.Errorf("Expected parameter %s to be kept", param)
		}
	}
}

func TestClientGrantsWithCheckpointPriority(t *testing.T) {
	// Test the exact scenario from the user's requirement:
	// Priority: [checkpoint, page, none]
	// Expected: Select checkpoint strategy, keep [from, take], remove [per_page, page, include_totals]

	operationYAML := `
parameters:
- name: per_page
  in: query
  description: Number of results per page.
  schema:
    type: integer
- name: page
  in: query
  description: Page index of the results to return.
  schema:
    type: integer
- name: include_totals
  in: query
  description: Return results inside an object.
  schema:
    type: boolean
- name: from
  in: query
  description: Optional Id from which to start selection.
  schema:
    type: string
- name: take
  in: query
  description: Number of results per page.
  schema:
    type: integer
- name: audience
  in: query
  description: Optional filter on audience.
  schema:
    type: string
- name: client_id
  in: query
  description: Optional filter on client_id.
  schema:
    type: string
`

	var opNode yaml.Node
	err := yaml.Unmarshal([]byte(operationYAML), &opNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	opContentNode := &opNode
	if opNode.Kind == yaml.DocumentNode && len(opNode.Content) > 0 {
		opContentNode = opNode.Content[0]
	}

	// Use priority: [checkpoint, page, none] as specified by user
	opts := Options{Priority: []string{"checkpoint", "page", "none"}}

	result, err := ProcessEndpoint(opContentNode, opts)
	if err != nil {
		t.Fatalf("ProcessEndpoint failed: %v", err)
	}

	t.Logf("Priority: %v", opts.Priority)
	t.Logf("Removed params: %v", result.RemovedParams)

	// Expected behavior according to user requirement:
	// - Detected: checkpoint [from, take] and page [per_page, page, include_totals]
	// - Priority: checkpoint comes first, so select checkpoint
	// - Keep: from, take, audience, client_id (checkpoint + non-pagination)
	// - Remove: per_page, page, include_totals (page strategy params)

	expectedRemoved := []string{"per_page", "page", "include_totals"}
	expectedKept := []string{"from", "take", "audience", "client_id"}

	removedMap := make(map[string]bool)
	for _, removed := range result.RemovedParams {
		removedMap[removed] = true
	}

	// Check that expected params were removed
	for _, expected := range expectedRemoved {
		if !removedMap[expected] {
			t.Errorf("Expected parameter %s to be removed but it wasn't", expected)
		}
	}

	// Check that expected params were kept (not in removed list)
	for _, expected := range expectedKept {
		if removedMap[expected] {
			t.Errorf("Expected parameter %s to be kept but it was removed", expected)
		}
	}

	t.Logf("SUCCESS: Checkpoint priority working correctly")
}

// TestSharedParameterRemoval tests that shared parameters are correctly removed
// when they don't belong to the selected strategy
func TestSharedParameterRemoval(t *testing.T) {
	// Test case: /users/{user_id}/sessions endpoint scenario
	// Parameters: [include_totals, from, take]
	// Priority: [checkpoint cursor offset page none]
	// Expected: Select "checkpoint", keep [from, take], remove [include_totals]

	yamlContent := `
parameters:
  - name: include_totals
    in: query
    description: Return results inside an object that contains the total result count
    required: false
    schema:
      type: boolean
  - name: from
    in: query
    description: A checkpoint value from which to begin retrieving results
    required: false
    schema:
      type: string
  - name: take
    in: query
    description: The maximum number of results to return
    required: false
    schema:
      type: integer
`

	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &node)
	if err != nil {
		t.Fatal(err)
	}

	// Get the actual content node (skip document wrapper)
	contentNode := &node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		contentNode = node.Content[0]
	}

	opts := Options{
		Priority: []string{"checkpoint", "cursor", "offset", "page", "none"},
	}

	result, err := ProcessEndpoint(contentNode, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the endpoint was changed
	if !result.Changed {
		t.Error("Expected endpoint to be changed, but it wasn't")
	}

	// Verify that include_totals was removed
	expectedRemoved := []string{"include_totals"}
	if len(result.RemovedParams) != len(expectedRemoved) {
		t.Errorf("Expected %d removed params, got %d: %v", len(expectedRemoved), len(result.RemovedParams), result.RemovedParams)
	}

	for _, expected := range expectedRemoved {
		found := false
		for _, removed := range result.RemovedParams {
			if removed == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected parameter %s to be removed but it wasn't", expected)
		}
	}

	// Get the parameters node to check what's left
	paramsNode := getNodeValue(contentNode, "parameters")
	if paramsNode == nil {
		t.Fatal("No parameters node found")
	}

	finalParams := extractParamNames(paramsNode)
	expectedKept := []string{"from", "take"}

	if len(finalParams) != len(expectedKept) {
		t.Errorf("Expected %d final params, got %d: %v", len(expectedKept), len(finalParams), finalParams)
	}

	for _, expected := range expectedKept {
		found := false
		for _, param := range finalParams {
			if param == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected parameter %s to be kept but it wasn't found in final params", expected)
		}
	}
}

// Helper function to extract parameter names from a parameters node
func extractParamNames(params *yaml.Node) []string {
	var names []string
	if params.Kind != yaml.SequenceNode {
		return names
	}

	for _, param := range params.Content {
		if param.Kind != yaml.MappingNode {
			continue
		}

		for i := 0; i < len(param.Content); i += 2 {
			if param.Content[i].Value == "name" {
				names = append(names, param.Content[i+1].Value)
				break
			}
		}
	}
	return names
}

// TestProcessEndpointWithRefParams tests that $ref parameters are properly handled
func TestProcessEndpointWithRefParams(t *testing.T) {
	// Define a complete OpenAPI document with $ref parameters
	docYAML := `
openapi: 3.0.0
components:
  parameters:
    PageParameter:
      name: page
      in: query
      schema:
        type: integer
    PerPageParameter:
      name: per_page
      in: query
      schema:
        type: integer
    OffsetParameter:
      name: offset
      in: query
      schema:
        type: integer
    LimitParameter:
      name: limit
      in: query
      schema:
        type: integer
`

	// Parse the document
	var doc yaml.Node
	err := yaml.Unmarshal([]byte(docYAML), &doc)
	if err != nil {
		t.Fatalf("Failed to parse document YAML: %v", err)
	}

	// Get the root node for $ref resolution
	root := doc.Content[0]

	// Test operation with $ref parameters (mixed pagination strategies)
	operationYAML := `
parameters:
  - $ref: "#/components/parameters/PageParameter"
  - $ref: "#/components/parameters/PerPageParameter"
  - $ref: "#/components/parameters/OffsetParameter"
  - $ref: "#/components/parameters/LimitParameter"
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
            page:
              type: integer
            per_page:
              type: integer
`

	var operation yaml.Node
	err = yaml.Unmarshal([]byte(operationYAML), &operation)
	if err != nil {
		t.Fatalf("Failed to parse operation YAML: %v", err)
	}

	// Get the operation mapping node from the document node
	operationMapping := operation.Content[0]

	opts := Options{
		Priority: []string{"page", "offset", "checkpoint", "cursor", "none"},
	}

	// Process the endpoint - this should detect both page and offset strategies
	// and choose page (higher priority), removing offset parameters
	result, err := ProcessEndpointWithDoc(operationMapping, root, opts)
	if err != nil {
		t.Fatalf("ProcessEndpointWithDoc failed: %v", err)
	}

	if !result.Changed {
		t.Error("Expected endpoint to be changed")
	}

	// Should remove offset and limit parameters, keeping page and per_page
	expectedRemoved := []string{"offset", "limit"}
	if len(result.RemovedParams) != len(expectedRemoved) {
		t.Errorf("Expected %d removed parameters, got %d: %v", len(expectedRemoved), len(result.RemovedParams), result.RemovedParams)
	}

	for _, expected := range expectedRemoved {
		found := false
		for _, removed := range result.RemovedParams {
			if removed == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected parameter %s to be removed", expected)
		}
	}

	// Verify that the correct parameters remain
	params := getNodeValue(operationMapping, "parameters")
	if params == nil || params.Kind != yaml.SequenceNode {
		t.Fatal("Expected parameters to be a sequence")
	}

	// Should have 2 parameters left (page and per_page $refs)
	if len(params.Content) != 2 {
		t.Errorf("Expected 2 parameters to remain, got %d", len(params.Content))
	}

	// Verify the remaining parameters are the correct $refs
	remainingRefs := make([]string, 0)
	for _, param := range params.Content {
		if ref := getNodeValue(param, "$ref"); ref != nil {
			remainingRefs = append(remainingRefs, ref.Value)
		}
	}

	expectedRefs := []string{
		"#/components/parameters/PageParameter",
		"#/components/parameters/PerPageParameter",
	}

	if len(remainingRefs) != len(expectedRefs) {
		t.Errorf("Expected %d remaining refs, got %d: %v", len(expectedRefs), len(remainingRefs), remainingRefs)
	}

	for _, expected := range expectedRefs {
		found := false
		for _, remaining := range remainingRefs {
			if remaining == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ref %s to remain", expected)
		}
	}
}

// TestDetectPaginationWithRefParams tests that $ref parameters are properly detected
func TestDetectPaginationWithRefParams(t *testing.T) {
	// Define a complete OpenAPI document with $ref parameters
	docYAML := `
openapi: 3.0.0
components:
  parameters:
    PageParameter:
      name: page
      in: query
      schema:
        type: integer
    PerPageParameter:
      name: per_page
      in: query
      schema:
        type: integer
    OffsetParameter:
      name: offset
      in: query
      schema:
        type: integer
    LimitParameter:
      name: limit
      in: query
      schema:
        type: integer
`

	// Parse the document
	var doc yaml.Node
	err := yaml.Unmarshal([]byte(docYAML), &doc)
	if err != nil {
		t.Fatalf("Failed to parse document YAML: %v", err)
	}

	// Get the root node for $ref resolution
	root := doc.Content[0]

	// Test parameters with $refs
	paramsYAML := `
- $ref: "#/components/parameters/PageParameter"
- $ref: "#/components/parameters/PerPageParameter"
- $ref: "#/components/parameters/OffsetParameter"
- $ref: "#/components/parameters/LimitParameter"
`

	var params yaml.Node
	err = yaml.Unmarshal([]byte(paramsYAML), &params)
	if err != nil {
		t.Fatalf("Failed to parse params YAML: %v", err)
	}

	// Get the sequence node from the document node
	paramsSequence := params.Content[0]

	// Test detection with doc context
	detected := DetectPaginationInParamsWithDoc(paramsSequence, root)

	t.Logf("Detected strategies: %v", detected)

	// Should detect both page and offset strategies
	expectedStrategies := []string{"page", "offset"}
	if len(detected) != len(expectedStrategies) {
		t.Errorf("Expected %d strategies, got %d", len(expectedStrategies), len(detected))
	}

	for _, expected := range expectedStrategies {
		found := false
		for _, d := range detected {
			if d.Strategy == expected {
				found = true
				t.Logf("Found strategy %s with params: %v", d.Strategy, d.Parameters)
				break
			}
		}
		if !found {
			t.Errorf("Expected strategy %s not found", expected)
		}
	}
}

// TestDetectPaginationWithRefParamsDebug tests with debug output
func TestDetectPaginationWithRefParamsDebug(t *testing.T) {
	docYAML := `
openapi: 3.0.0
components:
  parameters:
    PageParameter:
      name: page
      in: query
      schema:
        type: integer
`

	var doc yaml.Node
	err := yaml.Unmarshal([]byte(docYAML), &doc)
	if err != nil {
		t.Fatalf("Failed to parse document YAML: %v", err)
	}

	root := doc.Content[0]

	paramsYAML := `
- $ref: "#/components/parameters/PageParameter"
`

	var params yaml.Node
	err = yaml.Unmarshal([]byte(paramsYAML), &params)
	if err != nil {
		t.Fatalf("Failed to parse params YAML: %v", err)
	}

	// Debug the structure
	t.Logf("Params Kind: %v", params.Kind)
	t.Logf("Params Content Length: %d", len(params.Content))

	if len(params.Content) > 0 {
		param := params.Content[0]
		t.Logf("First param Kind: %v", param.Kind)

		// Check for $ref
		ref := getNodeValue(param, "$ref")
		if ref != nil {
			t.Logf("Found $ref: %s", ref.Value)

			// Try to resolve it
			resolved := resolveRef(ref.Value, root)
			if resolved != nil {
				t.Logf("Successfully resolved $ref")
				name := getStringValue(resolved, "name")
				t.Logf("Resolved name: %s", name)
			} else {
				t.Log("Failed to resolve $ref")
			}
		} else {
			t.Log("No $ref found")
		}
	}

	// Test detection
	detected := DetectPaginationInParamsWithDoc(&params, root)
	t.Logf("Detected strategies: %v", detected)
}

// TestParamsYAMLStructure tests the structure of the params YAML
func TestParamsYAMLStructure(t *testing.T) {
	paramsYAML := `
- $ref: "#/components/parameters/PageParameter"
- name: inline_param
  in: query
  schema:
    type: string
`

	var params yaml.Node
	err := yaml.Unmarshal([]byte(paramsYAML), &params)
	if err != nil {
		t.Fatalf("Failed to parse params YAML: %v", err)
	}

	t.Logf("Params Kind: %v", params.Kind)
	t.Logf("Params Content Length: %d", len(params.Content))

	for i, param := range params.Content {
		t.Logf("Param %d Kind: %v", i, param.Kind)
		t.Logf("Param %d Content Length: %d", i, len(param.Content))

		if param.Kind == yaml.MappingNode {
			for j := 0; j < len(param.Content); j += 2 {
				key := param.Content[j].Value
				value := param.Content[j+1].Value
				t.Logf("  %s: %s", key, value)
			}
		}
	}
}

// TestParamsYAMLStructureFixed tests the corrected structure access
func TestParamsYAMLStructureFixed(t *testing.T) {
	paramsYAML := `
- $ref: "#/components/parameters/PageParameter"
- name: inline_param
  in: query
  schema:
    type: string
`

	var params yaml.Node
	err := yaml.Unmarshal([]byte(paramsYAML), &params)
	if err != nil {
		t.Fatalf("Failed to parse params YAML: %v", err)
	}

	// The params node is a DocumentNode, we need the SequenceNode inside it
	if params.Kind == yaml.DocumentNode && len(params.Content) > 0 {
		sequence := params.Content[0]
		t.Logf("Sequence Kind: %v", sequence.Kind)
		t.Logf("Sequence Content Length: %d", len(sequence.Content))

		for i, param := range sequence.Content {
			t.Logf("Param %d Kind: %v", i, param.Kind)
			t.Logf("Param %d Content Length: %d", i, len(param.Content))

			if param.Kind == yaml.MappingNode {
				for j := 0; j < len(param.Content); j += 2 {
					key := param.Content[j].Value
					value := param.Content[j+1].Value
					t.Logf("  %s: %s", key, value)
				}
			}
		}
	}
}
