package transform

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFlattenSingleRefOneOf(t *testing.T) {
	// Test flattening oneOf with single $ref
	input := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    ListCustomDomainsResponseContent:
      oneOf:
        - $ref: "#/components/schemas/ListCustomDomainsPaginatedResponseContent"
    ListCustomDomainsPaginatedResponseContent:
      type: object
      properties:
        data:
          type: array
paths:
  /custom-domains:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ListCustomDomainsResponseContent"
`

	expected := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    ListCustomDomainsPaginatedResponseContent:
      type: object
      properties:
        data:
          type: array
paths:
  /custom-domains:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ListCustomDomainsPaginatedResponseContent"
`

	// Create temp file
	tmpFile := "test_flatten.yaml"
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

	var actual, expectedParsed interface{}
	if err := yaml.Unmarshal(actualBytes, &actual); err != nil {
		t.Fatalf("failed to parse actual YAML: %v", err)
	}
	if err := yaml.Unmarshal([]byte(expected), &expectedParsed); err != nil {
		t.Fatalf("failed to parse expected YAML: %v", err)
	}

	// Check that the reference was flattened directly to the final schema
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("actual YAML is not a map[string]interface{}")
	}

	components, ok := actualYaml["components"].(map[string]interface{})
	if !ok {
		t.Fatal("components is not a map[string]interface{}")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("schemas is not a map[string]interface{}")
	}

	// The intermediate schema should be removed
	if _, exists := schemas["ListCustomDomainsResponseContent"]; exists {
		t.Error("intermediate schema ListCustomDomainsResponseContent should have been removed")
	}

	// The final schema should still exist
	if _, exists := schemas["ListCustomDomainsPaginatedResponseContent"]; !exists {
		t.Error("final schema ListCustomDomainsPaginatedResponseContent should still exist")
	}

	// The path should reference the final schema directly
	paths, ok := actualYaml["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("paths is not a map[string]interface{}")
	}

	customDomains, ok := paths["/custom-domains"].(map[string]interface{})
	if !ok {
		t.Fatal("custom-domains path is not a map[string]interface{}")
	}

	get, ok := customDomains["get"].(map[string]interface{})
	if !ok {
		t.Fatal("get method is not a map[string]interface{}")
	}

	responses, ok := get["responses"].(map[string]interface{})
	if !ok {
		t.Fatal("responses is not a map[string]interface{}")
	}

	response200, ok := responses["200"].(map[string]interface{})
	if !ok {
		t.Fatal("200 response is not a map[string]interface{}")
	}

	content, ok := response200["content"].(map[string]interface{})
	if !ok {
		t.Fatal("content is not a map[string]interface{}")
	}

	applicationJSON, ok := content["application/json"].(map[string]interface{})
	if !ok {
		t.Fatal("application/json is not a map[string]interface{}")
	}

	schema, ok := applicationJSON["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("schema is not a map[string]interface{}")
	}

	if refValue, exists := schema["$ref"]; !exists {
		t.Error("expected $ref field in path schema")
	} else if refValue != "#/components/schemas/ListCustomDomainsPaginatedResponseContent" {
		t.Errorf("expected ref to be '#/components/schemas/ListCustomDomainsPaginatedResponseContent', got %v", refValue)
	}

	// Check that flattening was recorded
	if len(result.FlattenedRefs) == 0 {
		t.Error("expected flattened refs to be recorded")
	}
}

func TestFlattenSingleRefAnyOf(t *testing.T) {
	// Test flattening anyOf with single $ref
	input := `
openapi: 3.0.0
components:
  schemas:
    TestSchema:
      anyOf:
        - $ref: "#/components/schemas/ActualSchema"
`

	// Create temp file
	tmpFile := "test_flatten_anyof.yaml"
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

	// Check that anyOf was flattened to $ref
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("actual YAML is not a map[string]interface{}")
	}

	components, ok := actualYaml["components"].(map[string]interface{})
	if !ok {
		t.Fatal("components is not a map[string]interface{}")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("schemas is not a map[string]interface{}")
	}

	testSchema, ok := schemas["TestSchema"].(map[string]interface{})
	if !ok {
		t.Fatal("TestSchema is not a map[string]interface{}")
	}

	if refValue, exists := testSchema["$ref"]; !exists {
		t.Error("expected $ref field in flattened schema")
	} else if refValue != "#/components/schemas/ActualSchema" {
		t.Errorf("expected ref to be '#/components/schemas/ActualSchema', got %v", refValue)
	}

	if _, exists := testSchema["anyOf"]; exists {
		t.Error("anyOf field should have been removed")
	}
}

func TestFlattenMultipleRefs(t *testing.T) {
	// Test that oneOf with multiple refs is NOT flattened
	input := `
openapi: 3.0.0
components:
  schemas:
    TestSchema:
      oneOf:
        - $ref: "#/components/schemas/Schema1"
        - $ref: "#/components/schemas/Schema2"
`

	// Create temp file
	tmpFile := "test_no_flatten.yaml"
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

	if changed {
		t.Error("expected file NOT to be changed when oneOf has multiple refs")
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

	// Check that oneOf is still there
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("actual YAML is not a map[string]interface{}")
	}

	components, ok := actualYaml["components"].(map[string]interface{})
	if !ok {
		t.Fatal("components is not a map[string]interface{}")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("schemas is not a map[string]interface{}")
	}

	testSchema, ok := schemas["TestSchema"].(map[string]interface{})
	if !ok {
		t.Fatal("TestSchema is not a map[string]interface{}")
	}

	if _, exists := testSchema["oneOf"]; !exists {
		t.Error("oneOf field should still exist when there are multiple refs")
	}

	if _, exists := testSchema["$ref"]; exists {
		t.Error("$ref field should NOT exist when oneOf has multiple refs")
	}
}

func TestFlattenResponsesInPaths(t *testing.T) {
	// Test flattening in response schemas
	input := `
openapi: 3.0.0
paths:
  /test:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                oneOf:
                  - $ref: "#/components/schemas/SuccessResponse"
`

	// Create temp file
	tmpFile := "test_flatten_response.yaml"
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
		t.Fatal("actual YAML is not a map[string]interface{}")
	}

	paths, ok := actualYaml["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("paths is not a map[string]interface{}")
	}

	testPath, ok := paths["/test"].(map[string]interface{})
	if !ok {
		t.Fatal("test path is not a map[string]interface{}")
	}

	get, ok := testPath["get"].(map[string]interface{})
	if !ok {
		t.Fatal("get method is not a map[string]interface{}")
	}

	responses, ok := get["responses"].(map[string]interface{})
	if !ok {
		t.Fatal("responses is not a map[string]interface{}")
	}

	response200, ok := responses["200"].(map[string]interface{})
	if !ok {
		t.Fatal("200 response is not a map[string]interface{}")
	}

	content, ok := response200["content"].(map[string]interface{})
	if !ok {
		t.Fatal("content is not a map[string]interface{}")
	}

	appJSON, ok := content["application/json"].(map[string]interface{})
	if !ok {
		t.Fatal("application/json is not a map[string]interface{}")
	}

	schema, ok := appJSON["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("schema is not a map[string]interface{}")
	}

	// Check that oneOf was flattened to $ref
	if refValue, exists := schema["$ref"]; !exists {
		t.Error("expected $ref field in flattened response schema")
	} else if refValue != "#/components/schemas/SuccessResponse" {
		t.Errorf("expected ref to be '#/components/schemas/SuccessResponse', got %v", refValue)
	}

	if _, exists := schema["oneOf"]; exists {
		t.Error("oneOf field should have been removed from response schema")
	}
}

// TestFlattenEmptyComposition tests that empty compositions are properly removed
func TestFlattenEmptyComposition(t *testing.T) {
	input := `
openapi: 3.0.0
components:
  schemas:
    TestSchema:
      type: object
      properties:
        id:
          type: string
      oneOf: []
`

	// Create temp file
	tmpFile := "test_empty_composition.yaml"
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

	// Check that empty oneOf was removed
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("actual YAML is not a map[string]interface{}")
	}

	components, ok := actualYaml["components"].(map[string]interface{})
	if !ok {
		t.Fatal("components is not a map[string]interface{}")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("schemas is not a map[string]interface{}")
	}

	testSchema, ok := schemas["TestSchema"].(map[string]interface{})
	if !ok {
		t.Fatal("TestSchema is not a map[string]interface{}")
	}

	if _, exists := testSchema["oneOf"]; exists {
		t.Error("empty oneOf should have been removed")
	}

	// Verify other properties are preserved
	if _, exists := testSchema["type"]; !exists {
		t.Error("type property should be preserved")
	}

	if _, exists := testSchema["properties"]; !exists {
		t.Error("properties should be preserved")
	}
}

// TestFlattenMixedRefAndInlineSchemas tests mixed $ref and inline schemas
func TestFlattenMixedRefAndInlineSchemas(t *testing.T) {
	input := `
openapi: 3.0.0
components:
  schemas:
    TestSchema:
      oneOf:
        - $ref: "#/components/schemas/RefSchema"
        - type: object
          properties:
            name:
              type: string
    RefSchema:
      type: object
      properties:
        id:
          type: string
`

	// Create temp file
	tmpFile := "test_mixed_schemas.yaml"
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

	// Mixed compositions should NOT be flattened
	if changed {
		t.Error("mixed compositions should not be flattened")
	}
}

// TestFlattenDeeplyNestedCompositions tests deeply nested composition structures
func TestFlattenDeeplyNestedCompositions(t *testing.T) {
	input := `
openapi: 3.0.0
components:
  schemas:
    TestSchema:
      type: object
      properties:
        data:
          oneOf:
            - type: object
              properties:
                nested:
                  anyOf:
                    - type: string
`

	// Create temp file
	tmpFile := "test_nested_compositions.yaml"
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
		t.Error("expected nested compositions to be flattened")
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

	// Navigate to the nested structure and verify flattening
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse actual YAML as map")
	}
	components, ok := actualYaml["components"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse components as map")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse schemas as map")
	}
	testSchema, ok := schemas["TestSchema"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse TestSchema as map")
	}
	properties, ok := testSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse properties as map")
	}
	data, ok := properties["data"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse data as map")
	}

	// Should be flattened to just the object schema
	if _, exists := data["oneOf"]; exists {
		t.Error("oneOf should have been flattened")
	}

	if dataType, exists := data["type"]; !exists || dataType != "object" {
		t.Error("data should be flattened to object type")
	}
}

// TestFlattenInResponseContext tests flattening in response contexts
func TestFlattenInResponseContext(t *testing.T) {
	input := `
openapi: 3.0.0
paths:
  /test:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                anyOf:
                  - type: array
                    items:
                      type: string
`

	// Create temp file
	tmpFile := "test_response_context.yaml"
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
		t.Error("expected response schema to be flattened")
	}

	// Verify flattening was recorded
	if len(result.FlattenedRefs) == 0 {
		t.Error("expected flattened refs to be recorded")
	}
}

// TestFlattenComplexComposition tests complex composition with allOf
func TestFlattenComplexComposition(t *testing.T) {
	input := `
openapi: 3.0.0
components:
  schemas:
    TestSchema:
      allOf:
        - type: object
          properties:
            id:
              type: string
            name:
              type: string
`

	// Create temp file
	tmpFile := "test_complex_composition.yaml"
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
		t.Error("expected allOf with single item to be flattened")
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

	// Check that allOf was flattened
	actualYaml, ok := actual.(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse actual YAML as map")
	}
	components, ok := actualYaml["components"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse components as map")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse schemas as map")
	}
	testSchema, ok := schemas["TestSchema"].(map[string]interface{})
	if !ok {
		t.Fatal("failed to parse TestSchema as map")
	}

	if _, exists := testSchema["allOf"]; exists {
		t.Error("allOf should have been flattened")
	}

	if schemaType, exists := testSchema["type"]; !exists || schemaType != "object" {
		t.Error("schema should be flattened to object type")
	}

	if _, exists := testSchema["properties"]; !exists {
		t.Error("properties should be preserved after flattening")
	}
}
