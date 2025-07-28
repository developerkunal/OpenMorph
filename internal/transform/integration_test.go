package transform

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestValidationIntegration tests the validation system integration
func TestValidationIntegration(t *testing.T) {
	input := `
openapi: 3.0.0
components:
  schemas:
    ValidSchema:
      oneOf:
        - $ref: "#/components/schemas/RefSchema1"
        - $ref: "#/components/schemas/RefSchema2"
    SingleItemSchema:
      anyOf:
        - type: string
    EmptySchema:
      allOf: []
    MalformedRefSchema:
      oneOf:
        - $ref: ""
        - type: object
          properties:
            name:
              type: string
    RefSchema1:
      type: object
      properties:
        id:
          type: string
    RefSchema2:
      type: object
      properties:
        name:
          type: string
`

	// Parse the YAML
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Get the root node correctly
	root := getRootNode(&doc)

	// Validate
	validationResult := ValidateCompositionStructures(root, "test.yaml")

	// Should find multiple validation issues
	if validationResult.Valid {
		t.Error("expected validation to fail")
	}

	if len(validationResult.Errors) == 0 {
		t.Error("expected validation errors")
	}

	// Check for specific error types
	errorMessages := make([]string, len(validationResult.Errors))
	for i, err := range validationResult.Errors {
		errorMessages[i] = err.Error()
	}

	// Should detect single item compositions
	hasFlattening := false
	hasEmpty := false
	hasMalformed := false

	for _, msg := range errorMessages {
		if strings.Contains(msg, "single item could be flattened") {
			hasFlattening = true
		}
		if strings.Contains(msg, "array is empty") {
			hasEmpty = true
		}
		if strings.Contains(msg, "non-empty string value") {
			hasMalformed = true
		}
	}

	if !hasFlattening {
		t.Error("expected to detect single item composition")
	}
	if !hasEmpty {
		t.Error("expected to detect empty composition")
	}
	if !hasMalformed {
		t.Error("expected to detect malformed $ref")
	}
}

// TestCompleteEdgeCaseHandling tests all edge cases together
func TestCompleteEdgeCaseHandling(t *testing.T) {
	input := `
openapi: 3.0.0
paths:
  /complex:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                type: object
                properties:
                  # This will be flattened
                  single_ref:
                    oneOf:
                      - $ref: "#/components/schemas/SimpleSchema"
                  # This will be flattened (inline)
                  single_inline:
                    anyOf:
                      - type: array
                        items:
                          type: string
                  # This will remain (multiple items)
                  multiple_items:
                    allOf:
                      - type: object
                        properties:
                          id:
                            type: string
                      - type: object
                        properties:
                          name:
                            type: string
                  # This will be removed (empty)
                  empty_composition:
                    oneOf: []
components:
  schemas:
    SimpleSchema:
      type: object
      properties:
        value:
          type: string
`

	// Create temp file
	tmpFile := "test_complete_edge_cases.yaml"
	if err := os.WriteFile(tmpFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Process flattening
	opts := FlattenOptions{
		Options: Options{
			DryRun: false,
			Backup: false,
		},
		FlattenResponses: true,
	}

	result := &FlattenResult{
		ProcessedFiles: []string{},
		FlattenedRefs:  make(map[string][]string),
	}

	changed, err := processFlatteningInFile(tmpFile, opts, result)
	if err != nil {
		t.Fatalf("processFlatteningInFile failed: %v", err)
	}

	if !changed {
		t.Error("expected file to be changed")
	}

	// Verify the result
	actualBytes, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read result file: %v", err)
	}

	var actual interface{}
	if err := yaml.Unmarshal(actualBytes, &actual); err != nil {
		t.Fatalf("failed to parse actual YAML: %v", err)
	}

	// Navigate to the response schema
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse actual YAML as map")
	}
	paths, ok := actualYaml["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse paths as map")
	}
	complex, ok := paths["/complex"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse complex path as map")
	}
	get, ok := complex["get"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse get method as map")
	}
	responses, ok := get["responses"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse responses as map")
	}
	response200, ok := responses["200"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse 200 response as map")
	}
	content, ok := response200["content"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse content as map")
	}
	appJSON, ok := content["application/json"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse application/json as map")
	}
	schema, ok := appJSON["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse schema as map")
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse properties as map")
	}

	// Check single_ref was flattened to $ref
	singleRef, ok := properties["single_ref"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse single_ref as map")
	}
	if _, exists := singleRef["oneOf"]; exists {
		t.Error("single_ref oneOf should have been flattened")
	}
	if refValue, exists := singleRef["$ref"]; !exists || refValue != "#/components/schemas/SimpleSchema" {
		t.Error("single_ref should be flattened to $ref")
	}

	// Check single_inline was flattened to inline schema
	singleInline, ok := properties["single_inline"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse single_inline as map")
	}
	if _, exists := singleInline["anyOf"]; exists {
		t.Error("single_inline anyOf should have been flattened")
	}
	if schemaType, exists := singleInline["type"]; !exists || schemaType != "array" {
		t.Error("single_inline should be flattened to array type")
	}

	// Check multiple_items was NOT flattened
	multipleItems, ok := properties["multiple_items"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse multiple_items as map")
	}
	if _, exists := multipleItems["allOf"]; !exists {
		t.Error("multiple_items allOf should NOT have been flattened")
	}

	// Check empty_composition was removed
	if _, exists := properties["empty_composition"]; exists {
		t.Error("empty_composition should have been removed")
	}

	// Verify flattening was recorded
	if len(result.FlattenedRefs) == 0 {
		t.Error("expected flattened refs to be recorded")
	}

	// Check specific flattening records
	flattenedList := result.FlattenedRefs[tmpFile]
	if len(flattenedList) < 3 { // Should have at least 3 flattened items
		t.Errorf("expected at least 3 flattening operations, got %d", len(flattenedList))
	}
}

// TestErrorReporting tests error reporting functionality
func TestErrorReporting(t *testing.T) {
	input := `
openapi: 3.0.0
components:
  schemas:
    ErrorSchema:
      oneOf: []
      anyOf:
        - $ref: ""
`

	// Parse the YAML
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Get the root node correctly
	root := getRootNode(&doc)

	// Test error reporting
	errorReport := ValidateAndReportCompositions(root, "error_test.yaml")

	if errorReport == "" {
		t.Error("expected error report to be generated")
	}

	if !strings.Contains(errorReport, "Validation Issues Found") {
		t.Error("expected error report header")
	}

	if !strings.Contains(errorReport, "Empty Compositions") {
		t.Error("expected empty array error")
	}

	if !strings.Contains(errorReport, "Malformed References") {
		t.Error("expected malformed ref error")
	}
}
