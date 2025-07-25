package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Help(t *testing.T) {
	cmd := exec.Command("go", "run", "../main.go", "--help")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err != nil && !testing.Short() {
		t.Fatalf("help failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected help output")
	}
}

func TestCLI_OutputFlagPresent(t *testing.T) {
	cmd := exec.Command("go", "run", "../main.go", "--help")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err != nil && !testing.Short() {
		t.Fatalf("help failed: %v\n%s", err, out)
	}

	helpText := string(out)
	if !strings.Contains(helpText, "-o, --output string") {
		t.Error("expected --output flag to be present in help text")
	}
	if !strings.Contains(helpText, "Output file path (optional - if not provided, files are modified in place)") {
		t.Error("expected output flag description to be present in help text")
	}
}

func TestCLI_OutputWithSingleFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Create temporary directory
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Create test input file
	input := `openapi: 3.0.0
info:
  title: Test API
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run openmorph with output flag
	cmd := exec.Command("go", "run", "../main.go",
		"--input", inputFile,
		"--output", outputFile,
		"--map", "x-operation-group-name=x-fern-sdk-group-name",
		"--no-config")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("openmorph failed: %v\n%s", err, out)
	}

	// Verify output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("output file was not created")
	}

	// Verify transformation was applied
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(outputContent), "x-fern-sdk-group-name") {
		t.Error("expected transformation to be applied in output file")
	}

	// Verify input file was not modified
	inputContent, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("failed to read input file: %v", err)
	}

	if string(inputContent) != input {
		t.Error("input file should not be modified when using output file")
	}
}

func TestCLI_OutputWithJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Create temporary directory
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.json")
	outputFile := filepath.Join(tempDir, "output.json")

	// Create test input file
	input := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API"
  },
  "paths": {
    "/users": {
      "get": {
        "x-operation-group-name": "users"
      }
    }
  }
}`
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run openmorph with output flag
	cmd := exec.Command("go", "run", "../main.go",
		"--input", inputFile,
		"--output", outputFile,
		"--map", "x-operation-group-name=x-fern-sdk-group-name",
		"--no-config")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("openmorph failed: %v\n%s", err, out)
	}

	// Verify output file was created and transformed
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var outputData interface{}
	if err := json.Unmarshal(outputContent, &outputData); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}

	// Check that transformation was applied using JSON traversal
	dataMap, ok := outputData.(map[string]interface{})
	if !ok {
		t.Fatal("output data is not a map")
	}
	paths, ok := dataMap["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("paths field is not a map")
	}
	users, ok := paths["/users"].(map[string]interface{})
	if !ok {
		t.Fatal("/users path is not a map")
	}
	get, ok := users["get"].(map[string]interface{})
	if !ok {
		t.Fatal("get operation is not a map")
	}

	if _, exists := get["x-fern-sdk-group-name"]; !exists {
		t.Error("expected x-fern-sdk-group-name to be present in transformed JSON")
	}
	if _, exists := get["x-operation-group-name"]; exists {
		t.Error("expected x-operation-group-name to be transformed away")
	}
}

func TestCLI_OutputWithDirectoryError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Create temporary directory
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Try to use output flag with directory input (should fail)
	cmd := exec.Command("go", "run", "../main.go",
		"--input", tempDir,
		"--output", outputFile,
		"--no-config")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected command to fail when using --output with directory input")
	}

	if !strings.Contains(string(out), "--output flag can only be used with a single input file") {
		t.Errorf("expected specific error message, got: %s", string(out))
	}
}

func TestCLI_OutputWithInteractiveError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Create temporary directory and file
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Create test input file
	if err := os.WriteFile(inputFile, []byte("x-test: value"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Try to use output flag with interactive mode (should fail)
	cmd := exec.Command("go", "run", "../main.go",
		"--input", inputFile,
		"--output", outputFile,
		"--interactive",
		"--no-config")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected command to fail when using --output with --interactive")
	}

	if !strings.Contains(string(out), "--output flag cannot be used with --interactive mode") {
		t.Errorf("expected specific error message, got: %s", string(out))
	}
}

func TestCLI_OutputDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Create temporary directory
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Create test input file
	input := `x-test: value`
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run openmorph with output flag and dry-run
	cmd := exec.Command("go", "run", "../main.go",
		"--input", inputFile,
		"--output", outputFile,
		"--map", "x-test=x-transformed",
		"--dry-run",
		"--no-config")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("openmorph failed: %v\n%s", err, out)
	}

	// Verify output file was NOT created in dry-run mode
	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Error("output file should not be created in dry-run mode")
	}

	// Verify the dry-run output mentions the transformation
	outputText := string(out)
	if !strings.Contains(outputText, "DRY-RUN PREVIEW MODE") {
		t.Error("expected dry-run output to show preview mode header")
	}
}

// More CLI integration tests can be added for real-world scenarios.
