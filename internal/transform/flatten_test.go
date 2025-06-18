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
