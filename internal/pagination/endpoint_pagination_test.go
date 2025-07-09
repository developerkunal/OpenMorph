package pagination

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEndpointSpecificPagination(t *testing.T) {
	// Test the endpoint-specific pagination functionality
	t.Log("Testing endpoint-specific pagination configuration...")

	// Create some test endpoint rules
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/v1/users",
			Method:     "GET",
			Pagination: "cursor",
		},
		{
			Endpoint:   "/api/v1/posts/*",
			Method:     "POST",
			Pagination: "checkpoint",
		},
	}

	// Test options with global priority and endpoint rules
	opts := Options{
		Priority:      []string{"offset", "page", "none"},
		EndpointRules: rules,
	}

	// Test cases
	testCases := []struct {
		name     string
		endpoint string
		method   string
		expected string
	}{
		{"exact match", "/api/v1/users", "GET", "cursor"},
		{"wildcard match", "/api/v1/posts/123", "POST", "checkpoint"},
		{"fallback to global priority", "/api/v1/comments", "GET", "offset"},
		{"method mismatch", "/api/v1/users", "POST", "offset"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := opts.GetPaginationStrategy(tc.endpoint, tc.method)
			if len(result) == 0 {
				t.Errorf("Expected pagination strategy, got empty result")
				return
			}

			actual := result[0]
			if actual != tc.expected {
				t.Errorf("Expected %s, got %s for %s %s", tc.expected, actual, tc.method, tc.endpoint)
			}
		})
	}
}

func TestPatternMatching(t *testing.T) {
	// Test pattern matching functionality
	testCases := []struct {
		name     string
		endpoint string
		pattern  string
		expected bool
	}{
		{"exact match", "/api/v1/users", "/api/v1/users", true},
		{"wildcard match", "/api/v1/users/123", "/api/v1/users/*", true},
		{"no match", "/api/v1/posts", "/api/v1/users/*", false},
		{"base path with wildcard", "/api/v1/users", "/api/v1/users/*", true},
		{"complex path match", "/api/v1/users/123/posts", "/api/v1/users/*", true},
		{"wildcard without slash", "/api/v1/users123", "/api/v1/users*", true},
		{"partial match fails", "/api/v1/user", "/api/v1/users/*", false},
		{"root wildcard", "/", "/*", true},
		{"deep nested match", "/api/v1/users/123/posts/456/comments", "/api/v1/users/*", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test through GetPaginationStrategy since matchesEndpointPattern is not exported
			rules := []EndpointPaginationRule{
				{
					Endpoint:   tc.pattern,
					Method:     "GET",
					Pagination: "test",
				},
			}
			opts := Options{
				Priority:      []string{"default"},
				EndpointRules: rules,
			}

			result := opts.GetPaginationStrategy(tc.endpoint, "GET")
			matches := len(result) == 1 && result[0] == "test"

			if matches != tc.expected {
				t.Errorf("Pattern '%s' should match '%s': expected %v, got %v",
					tc.pattern, tc.endpoint, tc.expected, matches)
			}
		})
	}
}

func TestFallbackBehavior(t *testing.T) {
	// Test that when no endpoint rules match, it falls back to global priority
	opts := Options{
		Priority: []string{"checkpoint", "offset", "page"},
		EndpointRules: []EndpointPaginationRule{
			{
				Endpoint:   "/specific/endpoint",
				Method:     "GET",
				Pagination: "cursor",
			},
		},
	}

	// Test fallback
	result := opts.GetPaginationStrategy("/different/endpoint", "GET")
	expected := []string{"checkpoint", "offset", "page"}

	if len(result) != len(expected) {
		t.Errorf("Expected %d strategies, got %d", len(expected), len(result))
		return
	}

	for i, strategy := range expected {
		if result[i] != strategy {
			t.Errorf("Expected strategy %d to be %s, got %s", i, strategy, result[i])
		}
	}
}

func TestMethodCaseSensitivity(t *testing.T) {
	// Test that HTTP methods are case insensitive
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/users",
			Method:     "GET",
			Pagination: "cursor",
		},
	}

	opts := Options{
		Priority:      []string{"offset"},
		EndpointRules: rules,
	}

	testCases := []struct {
		method   string
		expected string
	}{
		{"GET", "cursor"},
		{"get", "cursor"},
		{"Get", "cursor"},
		{"POST", "offset"}, // fallback to global
	}

	for _, tc := range testCases {
		t.Run("method_"+tc.method, func(t *testing.T) {
			result := opts.GetPaginationStrategy("/api/users", tc.method)
			if len(result) == 0 {
				t.Errorf("Expected pagination strategy, got empty result")
				return
			}

			actual := result[0]
			if actual != tc.expected {
				t.Errorf("Expected %s, got %s for method %s", tc.expected, actual, tc.method)
			}
		})
	}
}

func TestMultipleRuleOrdering(t *testing.T) {
	// Test that the first matching rule wins
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/*",
			Method:     "GET",
			Pagination: "cursor",
		},
		{
			Endpoint:   "/api/users",
			Method:     "GET",
			Pagination: "checkpoint",
		},
	}

	opts := Options{
		Priority:      []string{"offset"},
		EndpointRules: rules,
	}

	// The first rule should match because it comes first in the slice
	result := opts.GetPaginationStrategy("/api/users", "GET")
	if len(result) == 0 {
		t.Errorf("Expected pagination strategy, got empty result")
		return
	}

	// Should match the first rule (cursor) even though the second rule is more specific
	expected := "cursor"
	actual := result[0]
	if actual != expected {
		t.Errorf("Expected %s (first rule), got %s", expected, actual)
	}
}

func TestEmptyRulesAndPriority(t *testing.T) {
	// Test behavior with empty rules and priority
	opts := Options{
		Priority:      []string{},
		EndpointRules: []EndpointPaginationRule{},
	}

	result := opts.GetPaginationStrategy("/api/users", "GET")

	// Should return empty slice when no priority is set
	if len(result) != 0 {
		t.Errorf("Expected empty result when no priority set, got %v", result)
	}
}

func TestOnlyEndpointRulesNoPriority(t *testing.T) {
	// Test with endpoint rules but no global priority
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/users",
			Method:     "GET",
			Pagination: "cursor",
		},
	}

	opts := Options{
		Priority:      []string{}, // No global priority
		EndpointRules: rules,
	}

	testCases := []struct {
		name     string
		endpoint string
		method   string
		expected int // expected length of result
	}{
		{"matching rule", "/api/users", "GET", 1},
		{"no matching rule", "/api/posts", "GET", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := opts.GetPaginationStrategy(tc.endpoint, tc.method)
			if len(result) != tc.expected {
				t.Errorf("Expected %d strategies, got %d: %v", tc.expected, len(result), result)
			}

			if tc.expected == 1 {
				if result[0] != "cursor" {
					t.Errorf("Expected cursor, got %s", result[0])
				}
			}
		})
	}
}

func TestComplexWildcardPatterns(t *testing.T) {
	// Test complex wildcard scenarios
	testCases := []struct {
		name     string
		pattern  string
		endpoint string
		expected bool
	}{
		{"root wildcard matches all", "/*", "/api/users", true},
		{"root wildcard matches root", "/*", "/", true},
		{"versioned API wildcard", "/api/v1/*", "/api/v1/users", true},
		{"versioned API wildcard exact", "/api/v1/*", "/api/v1", true},
		{"no match different version", "/api/v1/*", "/api/v2/users", false},
		{"deep nesting", "/api/v1/users/*", "/api/v1/users/123/posts/456", true},
		{"empty pattern", "", "/api/users", false},
		{"empty endpoint", "", "", true},
		{"trailing slash pattern", "/api/", "/api", false},
		{"trailing slash wildcard", "/api/*", "/api/", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rules := []EndpointPaginationRule{
				{
					Endpoint:   tc.pattern,
					Method:     "GET",
					Pagination: "test",
				},
			}
			opts := Options{
				Priority:      []string{"default"},
				EndpointRules: rules,
			}

			result := opts.GetPaginationStrategy(tc.endpoint, "GET")
			matches := len(result) == 1 && result[0] == "test"

			if matches != tc.expected {
				t.Errorf("Pattern '%s' matching '%s': expected %v, got %v",
					tc.pattern, tc.endpoint, tc.expected, matches)
			}
		})
	}
}

func TestEndToEndEndpointSpecificProcessing(t *testing.T) {
	// Test end-to-end processing with endpoint-specific rules
	operationYAML := `
parameters:
- name: offset
  in: query
  schema:
    type: integer
- name: limit
  in: query
  schema:
    type: integer
- name: cursor
  in: query
  schema:
    type: string
- name: size
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
            total:
              type: integer
            next_cursor:
              type: string
            users:
              type: array
              items:
                type: object
`

	// Parse once for endpoint-specific rule test
	var opNode1 yaml.Node
	err := yaml.Unmarshal([]byte(operationYAML), &opNode1)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML: %v", err)
	}

	opContentNode1 := &opNode1
	if opNode1.Kind == yaml.DocumentNode && len(opNode1.Content) > 0 {
		opContentNode1 = opNode1.Content[0]
	}

	// Test with endpoint-specific rule overriding global priority
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/users",
			Method:     "GET",
			Pagination: "cursor", // Override to prefer cursor
		},
	}

	opts := Options{
		Priority:      []string{"offset", "checkpoint"}, // Global priority prefers offset
		EndpointRules: rules,
	}

	// Test endpoint-specific processing
	result, err := ProcessEndpointWithPathAndMethod(opContentNode1, nil, "/api/users", "GET", opts)
	if err != nil {
		t.Fatalf("ProcessEndpointWithPathAndMethod failed: %v", err)
	}

	// Should remove offset/limit parameters and keep cursor/size
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

	// Cursor and size should NOT be removed
	for _, removed := range result.RemovedParams {
		if removed == "cursor" || removed == "size" {
			t.Errorf("Parameter %s was incorrectly removed - it should be kept for cursor strategy", removed)
		}
	}

	// Parse again for fallback test (to avoid interference from first test)
	var opNode2 yaml.Node
	err = yaml.Unmarshal([]byte(operationYAML), &opNode2)
	if err != nil {
		t.Fatalf("Failed to unmarshal operation YAML for fallback test: %v", err)
	}

	opContentNode2 := &opNode2
	if opNode2.Kind == yaml.DocumentNode && len(opNode2.Content) > 0 {
		opContentNode2 = opNode2.Content[0]
	}

	// Test fallback behavior for different endpoint
	resultFallback, err := ProcessEndpointWithPathAndMethod(opContentNode2, nil, "/api/posts", "GET", opts)
	if err != nil {
		t.Fatalf("ProcessEndpointWithPathAndMethod failed for fallback: %v", err)
	}

	// Should use global priority (offset), so cursor/size should be removed
	expectedRemovedFallback := map[string]bool{"cursor": false, "size": false}
	for _, removed := range resultFallback.RemovedParams {
		if _, exists := expectedRemovedFallback[removed]; exists {
			expectedRemovedFallback[removed] = true
		}
	}

	for param, found := range expectedRemovedFallback {
		if !found {
			t.Errorf("Expected parameter %s to be removed in fallback case but it wasn't", param)
		}
	}
}

func TestNoneStrategyWithEndpointRules(t *testing.T) {
	// Test that "none" strategy works correctly with endpoint rules
	operationYAML := `
parameters:
- name: offset
  in: query
  schema:
    type: integer
- name: limit
  in: query
  schema:
    type: integer
- name: api_key
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
            total:
              type: integer
            users:
              type: array
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

	// Endpoint rule specifies "none" strategy
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/legacy",
			Method:     "GET",
			Pagination: "none",
		},
	}

	opts := Options{
		Priority:      []string{"offset"}, // Global priority
		EndpointRules: rules,
	}

	// Test "none" strategy removes all pagination parameters
	result, err := ProcessEndpointWithPathAndMethod(opContentNode, nil, "/api/legacy", "GET", opts)
	if err != nil {
		t.Fatalf("ProcessEndpointWithPathAndMethod failed: %v", err)
	}

	// All pagination parameters should be removed
	expectedRemoved := map[string]bool{"offset": false, "limit": false}
	for _, removed := range result.RemovedParams {
		if _, exists := expectedRemoved[removed]; exists {
			expectedRemoved[removed] = true
		}
	}

	for param, found := range expectedRemoved {
		if !found {
			t.Errorf("Expected parameter %s to be removed with 'none' strategy but it wasn't", param)
		}
	}

	// Non-pagination parameters should NOT be removed
	for _, removed := range result.RemovedParams {
		if removed == "api_key" {
			t.Errorf("Non-pagination parameter %s was incorrectly removed", removed)
		}
	}
}

func TestWildcardPrecedenceRules(t *testing.T) {
	// Test that more specific patterns should be placed first for expected behavior
	// This test demonstrates the current behavior where first match wins

	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/*", // Broad wildcard
			Method:     "GET",
			Pagination: "offset",
		},
		{
			Endpoint:   "/api/v1/*", // More specific
			Method:     "GET",
			Pagination: "cursor",
		},
		{
			Endpoint:   "/api/v1/users", // Most specific
			Method:     "GET",
			Pagination: "checkpoint",
		},
	}

	opts := Options{
		Priority:      []string{"page"},
		EndpointRules: rules,
	}

	testCases := []struct {
		endpoint string
		expected string
		note     string
	}{
		{"/api/v1/users", "offset", "First rule matches (broad wildcard)"},
		{"/api/v1/posts", "offset", "First rule matches (broad wildcard)"},
		{"/api/v2/users", "offset", "First rule matches (broad wildcard)"},
	}

	for _, tc := range testCases {
		t.Run("endpoint_"+strings.ReplaceAll(tc.endpoint, "/", "_"), func(t *testing.T) {
			result := opts.GetPaginationStrategy(tc.endpoint, "GET")
			if len(result) == 0 {
				t.Errorf("Expected pagination strategy, got empty result")
				return
			}

			actual := result[0]
			if actual != tc.expected {
				t.Errorf("Expected %s, got %s for %s (%s)", tc.expected, actual, tc.endpoint, tc.note)
			}
		})
	}

	// Test with rules in reverse order (more specific first)
	rulesReversed := []EndpointPaginationRule{
		{
			Endpoint:   "/api/v1/users", // Most specific first
			Method:     "GET",
			Pagination: "checkpoint",
		},
		{
			Endpoint:   "/api/v1/*", // More specific
			Method:     "GET",
			Pagination: "cursor",
		},
		{
			Endpoint:   "/api/*", // Broad wildcard last
			Method:     "GET",
			Pagination: "offset",
		},
	}

	optsReversed := Options{
		Priority:      []string{"page"},
		EndpointRules: rulesReversed,
	}

	// Now the more specific rules should match first
	reversedCases := []struct {
		endpoint string
		expected string
		note     string
	}{
		{"/api/v1/users", "checkpoint", "Most specific rule matches"},
		{"/api/v1/posts", "cursor", "Second rule matches"},
		{"/api/v2/users", "offset", "Third rule matches"},
	}

	for _, tc := range reversedCases {
		t.Run("reversed_"+strings.ReplaceAll(tc.endpoint, "/", "_"), func(t *testing.T) {
			result := optsReversed.GetPaginationStrategy(tc.endpoint, "GET")
			if len(result) == 0 {
				t.Errorf("Expected pagination strategy, got empty result")
				return
			}

			actual := result[0]
			if actual != tc.expected {
				t.Errorf("Expected %s, got %s for %s (%s)", tc.expected, actual, tc.endpoint, tc.note)
			}
		})
	}
}

func TestHTTPMethodMatching(t *testing.T) {
	// Test HTTP method matching with various cases
	rules := []EndpointPaginationRule{
		{
			Endpoint:   "/api/users",
			Method:     "GET",
			Pagination: "cursor",
		},
		{
			Endpoint:   "/api/users",
			Method:     "POST",
			Pagination: "offset",
		},
		{
			Endpoint:   "/api/*",
			Method:     "*", // Wildcard method
			Pagination: "page",
		},
	}

	opts := Options{
		Priority:      []string{"checkpoint"},
		EndpointRules: rules,
	}

	testCases := []struct {
		endpoint string
		method   string
		expected string
		note     string
	}{
		{"/api/users", "GET", "cursor", "Exact method match"},
		{"/api/users", "get", "cursor", "Case insensitive match"},
		{"/api/users", "POST", "offset", "Different method"},
		{"/api/users", "PUT", "page", "Wildcard method match"},
		{"/api/posts", "DELETE", "page", "Wildcard endpoint and method"},
		{"/other/endpoint", "GET", "checkpoint", "No match - fallback to global"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+"_"+strings.ReplaceAll(tc.endpoint, "/", "_"), func(t *testing.T) {
			result := opts.GetPaginationStrategy(tc.endpoint, tc.method)
			if len(result) == 0 {
				t.Errorf("Expected pagination strategy, got empty result")
				return
			}

			actual := result[0]
			if actual != tc.expected {
				t.Errorf("Expected %s, got %s for %s %s (%s)", tc.expected, actual, tc.method, tc.endpoint, tc.note)
			}
		})
	}
}

func TestComplexRealWorldScenarios(t *testing.T) {
	// Test realistic configuration scenarios
	rules := []EndpointPaginationRule{
		// Legacy API v1 - all endpoints use offset
		{
			Endpoint:   "/api/v1/*",
			Method:     "GET",
			Pagination: "offset",
		},
		// Analytics - high performance cursor pagination
		{
			Endpoint:   "/api/*/analytics",
			Method:     "GET",
			Pagination: "cursor",
		},
		// Search - user-friendly page-based
		{
			Endpoint:   "/api/*/search",
			Method:     "GET",
			Pagination: "page",
		},
		// Admin endpoints - no pagination
		{
			Endpoint:   "/api/admin/*",
			Method:     "*",
			Pagination: "none",
		},
		// Specific high-traffic endpoint
		{
			Endpoint:   "/api/v2/users/feed",
			Method:     "GET",
			Pagination: "cursor",
		},
	}

	opts := Options{
		Priority:      []string{"checkpoint", "offset", "page"},
		EndpointRules: rules,
	}

	testCases := []struct {
		endpoint string
		method   string
		expected string
		scenario string
	}{
		{"/api/v1/users", "GET", "offset", "Legacy API"},
		{"/api/v1/analytics", "GET", "offset", "Legacy analytics (first rule wins)"},
		{"/api/v2/analytics", "GET", "cursor", "Modern analytics"},
		{"/api/v2/search", "GET", "page", "Search endpoint"},
		{"/api/admin/users", "GET", "none", "Admin endpoint"},
		{"/api/admin/config", "POST", "none", "Admin POST"},
		{"/api/v2/users/feed", "GET", "cursor", "High-traffic feed"},
		{"/api/v2/users", "GET", "checkpoint", "Fallback to global priority"},
		{"/api/v3/posts", "GET", "checkpoint", "New API version"},
	}

	for _, tc := range testCases {
		t.Run(tc.scenario, func(t *testing.T) {
			result := opts.GetPaginationStrategy(tc.endpoint, tc.method)
			if len(result) == 0 {
				t.Errorf("Expected pagination strategy, got empty result")
				return
			}

			actual := result[0]
			if actual != tc.expected {
				t.Errorf("Scenario '%s': Expected %s, got %s for %s %s",
					tc.scenario, tc.expected, actual, tc.method, tc.endpoint)
			}
		})
	}
}

func TestErrorConditionsAndEdgeCases(t *testing.T) {
	// Test various error conditions and edge cases

	t.Run("empty_endpoint_pattern", func(t *testing.T) {
		rules := []EndpointPaginationRule{
			{
				Endpoint:   "",
				Method:     "GET",
				Pagination: "cursor",
			},
		}
		opts := Options{
			Priority:      []string{"offset"},
			EndpointRules: rules,
		}

		// Empty pattern should only match empty endpoint
		result := opts.GetPaginationStrategy("", "GET")
		if len(result) != 1 || result[0] != "cursor" {
			t.Errorf("Expected cursor for empty endpoint, got %v", result)
		}

		result = opts.GetPaginationStrategy("/api/users", "GET")
		if len(result) != 1 || result[0] != "offset" {
			t.Errorf("Expected fallback to offset, got %v", result)
		}
	})

	t.Run("special_characters_in_endpoints", func(t *testing.T) {
		rules := []EndpointPaginationRule{
			{
				Endpoint:   "/api/users-test",
				Method:     "GET",
				Pagination: "cursor",
			},
			{
				Endpoint:   "/api/users_test",
				Method:     "GET",
				Pagination: "offset",
			},
			{
				Endpoint:   "/api/users.test",
				Method:     "GET",
				Pagination: "page",
			},
		}
		opts := Options{
			Priority:      []string{"checkpoint"},
			EndpointRules: rules,
		}

		testCases := []struct {
			endpoint string
			expected string
		}{
			{"/api/users-test", "cursor"},
			{"/api/users_test", "offset"},
			{"/api/users.test", "page"},
			{"/api/users", "checkpoint"}, // No match
		}

		for _, tc := range testCases {
			result := opts.GetPaginationStrategy(tc.endpoint, "GET")
			if len(result) != 1 || result[0] != tc.expected {
				t.Errorf("Endpoint %s: expected %s, got %v", tc.endpoint, tc.expected, result)
			}
		}
	})

	t.Run("case_sensitivity", func(t *testing.T) {
		rules := []EndpointPaginationRule{
			{
				Endpoint:   "/API/Users", // Mixed case in endpoint
				Method:     "get",        // Lowercase method
				Pagination: "cursor",
			},
		}
		opts := Options{
			Priority:      []string{"offset"},
			EndpointRules: rules,
		}

		// Endpoint patterns should be case sensitive, methods should not
		result := opts.GetPaginationStrategy("/API/Users", "GET")
		if len(result) != 1 || result[0] != "cursor" {
			t.Errorf("Expected cursor for exact endpoint match, got %v", result)
		}

		result = opts.GetPaginationStrategy("/api/users", "GET")
		if len(result) != 1 || result[0] != "offset" {
			t.Errorf("Expected fallback for case mismatch, got %v", result)
		}
	})
}

func TestAdvancedPatternMatching(t *testing.T) {
	// Test advanced pattern matching with multiple wildcards
	testCases := []struct {
		endpoint string
		pattern  string
		expected bool
		note     string
	}{
		// Multiple wildcards
		{"/api/v1/users/123/posts", "/api/*/users/*/posts", true, "Multiple wildcards"},
		{"/api/v2/users/abc/posts", "/api/*/users/*/posts", true, "Multiple wildcards different version"},
		{"/api/v1/users/posts", "/api/*/users/*/posts", false, "Missing middle segment"},

		// Complex patterns
		{"/tenant/abc/api/v1/reports", "/tenant/*/api/v*/reports", true, "Complex pattern"},
		{"/tenant/xyz/api/v2/reports", "/tenant/*/api/v*/reports", true, "Complex pattern v2"},
		{"/tenant/abc/api/reports", "/tenant/*/api/v*/reports", false, "Missing version"},

		// Edge cases with wildcards
		{"/a/b/c", "/*/b/*", true, "Wildcard at start and end"},
		{"/a/b/c", "/*/*/*", true, "All wildcards"},
		{"/a/b", "/*/*/*", false, "Not enough segments"},
		{"/a/b/c/d", "/*/*/*", false, "Too many segments"},

		// Mixed with existing patterns
		{"/api/v1/analytics", "/api/*/analytics", true, "Middle wildcard"},
		{"/api/legacy/users", "/api/legacy/*", true, "Suffix wildcard"},
		{"/api/legacy/users/123", "/api/legacy/*", true, "Suffix wildcard with path"},
	}

	for _, tc := range testCases {
		t.Run(tc.note+"_"+strings.ReplaceAll(tc.endpoint, "/", "_"), func(t *testing.T) {
			rules := []EndpointPaginationRule{
				{
					Endpoint:   tc.pattern,
					Method:     "GET",
					Pagination: "test",
				},
			}
			opts := Options{
				Priority:      []string{"default"},
				EndpointRules: rules,
			}

			result := opts.GetPaginationStrategy(tc.endpoint, "GET")
			matches := len(result) == 1 && result[0] == "test"

			if matches != tc.expected {
				t.Errorf("Pattern '%s' matching '%s': expected %v, got %v (%s)",
					tc.pattern, tc.endpoint, tc.expected, matches, tc.note)
			}
		})
	}
}
