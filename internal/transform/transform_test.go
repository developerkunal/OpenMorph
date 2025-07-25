package transform

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestShouldExclude(t *testing.T) {
	if !shouldExclude("foo", []string{"foo", "bar"}) {
		t.Error("should exclude foo")
	}
	if shouldExclude("baz", []string{"foo", "bar"}) {
		t.Error("should not exclude baz")
	}
}

func TestIsYAMLJSON(t *testing.T) {
	if !IsYAML("foo.yaml") || !IsYAML("foo.yml") {
		t.Error("IsYAML failed")
	}
	if !IsJSON("foo.json") {
		t.Error("IsJSON failed")
	}
}

func TestEqualBytes(t *testing.T) {
	a := []byte("abc")
	b := []byte("abc")
	c := []byte("def")
	if !equalBytes(a, b) {
		t.Error("should be equal")
	}
	if equalBytes(a, c) {
		t.Error("should not be equal")
	}
}

func TestTransformFileDryRun(t *testing.T) {
	f := "test.json"
	input := `{"x-a": 1}`
	if err := os.WriteFile(f, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	opts := Options{Mappings: map[string]string{"x-a": "x-z"}, DryRun: true}
	changed, err := File(f, opts)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !changed {
		t.Errorf("expected dry-run to detect change, got %v", changed)
	}
}

func TestTransformFileBackup(t *testing.T) {
	f := "test.yaml"
	if err := os.WriteFile(f, []byte("x-a: 1\n"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	defer os.Remove(f + ".bak")
	opts := Options{Mappings: map[string]string{"x-a": "x-z"}, Backup: true}
	changed, err := File(f, opts)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if changed {
		if _, err := os.Stat(f + ".bak"); err != nil {
			t.Errorf("expected backup file, got %v", err)
		}
	}
}

func TestTransformFileJSON(t *testing.T) {
	f := "test.json"
	input := `{"x-a": 1, "x-b": {"x-c": 2}}`
	expected := `{"x-z": 1, "x-b": {"x-y": 2}}`
	if err := os.WriteFile(f, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	opts := Options{Mappings: map[string]string{"x-a": "x-z", "x-c": "x-y"}}
	_, err := File(f, opts)
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}
	actual, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	var act, exp interface{}
	if err := json.Unmarshal(actual, &act); err != nil {
		t.Fatalf("unmarshal actual: %v", err)
	}
	if err := json.Unmarshal([]byte(expected), &exp); err != nil {
		t.Fatalf("unmarshal expected: %v", err)
	}
	if !reflect.DeepEqual(act, exp) {
		t.Errorf("json transform mismatch:\nactual:   %#v\nexpected: %#v", act, exp)
	}
}

// In tests that check output or file content, for JSON files, unmarshal both expected and actual output to interface{} and compare via reflect.DeepEqual to avoid formatting/sorting issues.
// For YAML, compare as before.

func TestTransformFileWithOutput(t *testing.T) {
	// Test YAML transformation with output file
	inputFile := "test_input.yaml"
	outputFile := "test_output.yaml"
	input := `openapi: 3.0.0
info:
  title: Test API
paths:
  /users:
    get:
      x-operation-group-name: users
      x-tag: user-ops
`
	expected := `openapi: 3.0.0
info:
  title: Test API
paths:
  /users:
    get:
      x-fern-sdk-group-name: users
      x-fern-tag: user-ops
`

	// Write input file
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)

	// Transform with output file
	opts := Options{
		Mappings: map[string]string{
			"x-operation-group-name": "x-fern-sdk-group-name",
			"x-tag":                  "x-fern-tag",
		},
		OutputFile: outputFile,
	}

	changed, err := File(inputFile, opts)
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}
	if !changed {
		t.Error("expected file to be transformed")
	}

	// Check that input file is unchanged
	inputContent, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("failed to read input file: %v", err)
	}
	if string(inputContent) != input {
		t.Error("input file should not be modified when using output file")
	}

	// Check output file content
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Normalize whitespace for comparison (YAML can have different formatting)
	// Parse both as YAML and compare the parsed content instead of raw strings
	var actualData, expectedData interface{}
	if err := yaml.Unmarshal(outputContent, &actualData); err != nil {
		t.Fatalf("failed to parse actual output YAML: %v", err)
	}
	if err := yaml.Unmarshal([]byte(expected), &expectedData); err != nil {
		t.Fatalf("failed to parse expected YAML: %v", err)
	}

	if !reflect.DeepEqual(actualData, expectedData) {
		t.Errorf("output file content mismatch:\nactual:\n%s\nexpected:\n%s", string(outputContent), expected)
	}
}

func TestTransformFileWithOutputJSON(t *testing.T) {
	// Test JSON transformation with output file
	inputFile := "test_input.json"
	outputFile := "test_output.json"
	input := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API"
  },
  "paths": {
    "/users": {
      "get": {
        "x-operation-group-name": "users",
        "x-tag": "user-ops"
      }
    }
  }
}`

	// Write input file
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)

	// Transform with output file
	opts := Options{
		Mappings: map[string]string{
			"x-operation-group-name": "x-fern-sdk-group-name",
			"x-tag":                  "x-fern-tag",
		},
		OutputFile: outputFile,
	}

	changed, err := File(inputFile, opts)
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}
	if !changed {
		t.Error("expected file to be transformed")
	}

	// Check that input file is unchanged
	inputContent, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("failed to read input file: %v", err)
	}
	if string(inputContent) != input {
		t.Error("input file should not be modified when using output file")
	}

	// Check output file content using JSON unmarshaling for robust comparison
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var outputData, expectedData interface{}
	if err := json.Unmarshal(outputContent, &outputData); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	expectedJSON := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API"
  },
  "paths": {
    "/users": {
      "get": {
        "x-fern-sdk-group-name": "users",
        "x-fern-tag": "user-ops"
      }
    }
  }
}`

	if err := json.Unmarshal([]byte(expectedJSON), &expectedData); err != nil {
		t.Fatalf("failed to unmarshal expected: %v", err)
	}

	if !reflect.DeepEqual(outputData, expectedData) {
		t.Errorf("output JSON mismatch:\nactual:   %#v\nexpected: %#v", outputData, expectedData)
	}
}

func TestTransformFileWithOutputNoBackup(t *testing.T) {
	// Test that backup is not created when using output file
	inputFile := "test_input.yaml"
	outputFile := "test_output.yaml"
	input := `x-test: value`

	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)
	defer os.Remove(inputFile + ".bak") // Cleanup backup if it was mistakenly created

	opts := Options{
		Mappings:   map[string]string{"x-test": "x-transformed"},
		OutputFile: outputFile,
		Backup:     true, // Backup enabled but should not create backup for input when using output file
	}

	_, err := File(inputFile, opts)
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}

	// Verify no backup was created
	if _, err := os.Stat(inputFile + ".bak"); err == nil {
		t.Error("backup file should not be created when using output file")
	}
}

func TestTransformFileWithOutputDryRun(t *testing.T) {
	// Test dry run with output file
	inputFile := "test_input.yaml"
	outputFile := "test_output.yaml"
	input := `x-test: value`

	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}
	defer os.Remove(inputFile)
	defer os.Remove(outputFile) // Cleanup if accidentally created

	opts := Options{
		Mappings:   map[string]string{"x-test": "x-transformed"},
		OutputFile: outputFile,
		DryRun:     true,
	}

	changed, err := File(inputFile, opts)
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}
	if !changed {
		t.Error("dry run should detect changes")
	}

	// Verify output file was not created in dry run
	if _, err := os.Stat(outputFile); err == nil {
		t.Error("output file should not be created in dry run mode")
	}
}
