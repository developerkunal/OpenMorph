package transform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/developerkunal/OpenMorph/internal/config"
)

func TestNewTransformationPipeline(t *testing.T) {
	cfg := &config.Config{
		Mappings: map[string]string{"x-test": "x-new"},
	}
	vendorProviders := []string{"fern", "speakeasy"}

	pipeline := NewTransformationPipeline(cfg, vendorProviders, true, false, "output.yaml")

	if pipeline.Config != cfg {
		t.Error("Expected config to be set correctly")
	}
	if len(pipeline.VendorProviders) != 2 {
		t.Errorf("Expected 2 vendor providers, got %d", len(pipeline.VendorProviders))
	}
	if !pipeline.DryRun {
		t.Error("Expected DryRun to be true")
	}
	if pipeline.Backup {
		t.Error("Expected Backup to be false")
	}
	if pipeline.OutputFile != "output.yaml" {
		t.Errorf("Expected OutputFile to be 'output.yaml', got '%s'", pipeline.OutputFile)
	}
}

func TestNormalizeResultPaths(t *testing.T) {
	inputPath := "/original/input.yaml"
	files := []string{"/temp/temp1.yaml", "/temp/temp2.yaml"}

	normalized := normalizeResultPaths(inputPath, files)

	if len(normalized) != 2 {
		t.Errorf("Expected 2 normalized paths, got %d", len(normalized))
	}
	for _, path := range normalized {
		if path != inputPath {
			t.Errorf("Expected normalized path to be '%s', got '%s'", inputPath, path)
		}
	}
}

func TestNormalizeMapKeys(t *testing.T) {
	inputPath := "/original/input.yaml"

	t.Run("empty map", func(t *testing.T) {
		originalMap := make(map[string][]string)
		normalized := normalizeMapKeys(inputPath, originalMap)

		if len(normalized) != 0 {
			t.Errorf("Expected empty map, got %v", normalized)
		}
	})

	t.Run("map with entries", func(t *testing.T) {
		originalMap := map[string][]string{
			"/temp/temp1.yaml": {"ref1", "ref2"},
			"/temp/temp2.yaml": {"ref3", "ref4"},
		}
		normalized := normalizeMapKeys(inputPath, originalMap)

		if len(normalized) != 1 {
			t.Errorf("Expected 1 entry in normalized map, got %d", len(normalized))
		}
		if refs, exists := normalized[inputPath]; !exists {
			t.Error("Expected normalized map to contain input path")
		} else if len(refs) == 0 {
			t.Error("Expected normalized map to contain references")
		}
	})
}

func TestExecuteFullPipeline_OutputMode(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cfg := &config.Config{
		Mappings: map[string]string{"x-operation-group-name": "x-fern-sdk-group-name"},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, outputFile)

	results, err := pipeline.ExecuteFullPipeline(inputFile)
	if err != nil {
		t.Fatalf("ExecuteFullPipeline failed: %v", err)
	}

	if !results.AnyTransformations {
		t.Error("Expected transformations to be detected")
	}

	if len(results.Changed) != 1 {
		t.Errorf("Expected 1 changed file, got %d", len(results.Changed))
	}

	// Check that output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Expected output file to be created")
	}
}

func TestExecuteFullPipeline_DirectoryMode(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cfg := &config.Config{
		Mappings: map[string]string{"x-operation-group-name": "x-fern-sdk-group-name"},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, "")

	results, err := pipeline.ExecuteFullPipeline(tempDir)
	if err != nil {
		t.Fatalf("ExecuteFullPipeline failed: %v", err)
	}

	if !results.AnyTransformations {
		t.Error("Expected transformations to be detected")
	}

	if len(results.Changed) == 0 {
		t.Error("Expected at least one changed file")
	}
}

func TestExecuteSingleFileWithOutput(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cfg := &config.Config{
		Mappings: map[string]string{"x-operation-group-name": "x-fern-sdk-group-name"},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, outputFile)

	results, err := pipeline.executeSingleFileWithOutput(inputFile)
	if err != nil {
		t.Fatalf("executeSingleFileWithOutput failed: %v", err)
	}

	if !results.AnyTransformations {
		t.Error("Expected transformations to be detected")
	}

	// Check that output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Expected output file to be created")
	}

	// Verify the transformation was applied
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(outputContent)
	if !strings.Contains(outputStr, "x-fern-sdk-group-name") {
		t.Error("Expected transformed key in output file")
	}
	if strings.Contains(outputStr, "x-operation-group-name") {
		t.Error("Expected original key to be replaced in output file")
	}
}

func TestExecuteDirectoryPipeline(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cfg := &config.Config{
		Mappings: map[string]string{"x-operation-group-name": "x-fern-sdk-group-name"},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, "")

	results, err := pipeline.executeDirectoryPipeline(tempDir)
	if err != nil {
		t.Fatalf("executeDirectoryPipeline failed: %v", err)
	}

	if !results.AnyTransformations {
		t.Error("Expected transformations to be detected")
	}

	if len(results.Changed) == 0 {
		t.Error("Expected at least one changed file")
	}
}

func TestSetupTempProcessing(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")

	// Create test input file
	inputContent := "test: content"
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	pipeline := &TransformationPipeline{}

	tempProcDir, tempFilePath, cleanup, err := pipeline.setupTempProcessing(inputFile)
	if err != nil {
		t.Fatalf("setupTempProcessing failed: %v", err)
	}
	defer cleanup()

	// Check that temporary directory was created
	if _, err := os.Stat(tempProcDir); os.IsNotExist(err) {
		t.Error("Expected temporary directory to be created")
	}

	// Check that temporary file was created
	if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
		t.Error("Expected temporary file to be created")
	}

	// Check that temporary file has correct content
	tempContent, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read temporary file: %v", err)
	}

	if string(tempContent) != inputContent {
		t.Errorf("Expected temporary file content '%s', got '%s'", inputContent, string(tempContent))
	}

	// Test cleanup
	cleanup()
	if _, err := os.Stat(tempProcDir); !os.IsNotExist(err) {
		t.Error("Expected temporary directory to be cleaned up")
	}
}

func TestApplySingleFileTransformations(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")
	tempProcessDir := filepath.Join(tempDir, "temp")
	tempFile := filepath.Join(tempProcessDir, "temp_input.yaml")

	// Create temp processing directory and file
	if err := os.MkdirAll(tempProcessDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(tempFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	cfg := &config.Config{
		Mappings: map[string]string{"x-operation-group-name": "x-fern-sdk-group-name"},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, "")

	opts := Options{
		Mappings: cfg.Mappings,
		Exclude:  []string{},
		DryRun:   false,
		Backup:   false,
	}

	results := &TransformationResults{
		Changed: []string{},
	}

	anyChanges, err := pipeline.applySingleFileTransformations(inputFile, tempProcessDir, tempFile, opts, results)
	if err != nil {
		t.Fatalf("applySingleFileTransformations failed: %v", err)
	}

	if !anyChanges {
		t.Error("Expected changes to be detected")
	}
}

func TestPipelineStepMethods(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
      parameters:
        - name: page
          in: query
          schema:
            type: integer
        - name: limit
          in: query
          schema:
            type: integer
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cfg := &config.Config{
		PaginationPriority: []string{"page", "offset", "cursor"},
		FlattenResponses:   true,
		VendorExtensions: config.VendorExtensions{
			Enabled: true,
			Providers: map[string]config.ProviderConfig{
				"fern": {
					ExtensionName: "x-fern-pagination",
					TargetLevel:   "operation",
				},
			},
		},
		DefaultValues: config.DefaultValues{
			Enabled: true,
		},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, "")

	opts := Options{
		Mappings: map[string]string{},
		Exclude:  []string{},
		DryRun:   false,
		Backup:   false,
	}

	results := &TransformationResults{
		Changed: []string{},
	}

	// Test individual step methods
	t.Run("applyPaginationStep", func(t *testing.T) {
		err := pipeline.applyPaginationStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyPaginationStep failed: %v", err)
		}
	})

	t.Run("applyFlatteningStep", func(t *testing.T) {
		err := pipeline.applyFlatteningStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyFlatteningStep failed: %v", err)
		}
	})

	t.Run("applyVendorExtensionsStep", func(t *testing.T) {
		err := pipeline.applyVendorExtensionsStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyVendorExtensionsStep failed: %v", err)
		}
	})

	t.Run("applyDefaultsStep", func(t *testing.T) {
		err := pipeline.applyDefaultsStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyDefaultsStep failed: %v", err)
		}
	})
}

func TestSingleFileStepMethods(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")
	tempProcessDir := filepath.Join(tempDir, "temp")

	// Create temp processing directory
	if err := os.MkdirAll(tempProcessDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
      parameters:
        - name: page
          in: query
          schema:
            type: integer
`
	// Create temp file with content
	tempFile := filepath.Join(tempProcessDir, "temp_input.yaml")
	if err := os.WriteFile(tempFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	cfg := &config.Config{
		PaginationPriority: []string{"page", "offset", "cursor"},
		FlattenResponses:   true,
		VendorExtensions: config.VendorExtensions{
			Enabled: true,
			Providers: map[string]config.ProviderConfig{
				"fern": {
					ExtensionName: "x-fern-pagination",
					TargetLevel:   "operation",
				},
			},
		},
		DefaultValues: config.DefaultValues{
			Enabled: true,
		},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, "")

	opts := Options{
		Mappings: map[string]string{},
		Exclude:  []string{},
		DryRun:   false,
		Backup:   false,
	}

	results := &TransformationResults{
		Changed: []string{},
	}

	// Test individual single file step methods
	t.Run("applySingleFilePagination", func(t *testing.T) {
		changed, err := pipeline.applySingleFilePagination(inputFile, tempProcessDir, opts, results)
		if err != nil {
			t.Errorf("applySingleFilePagination failed: %v", err)
		}
		// Don't require changes as it depends on specific pagination detection
		_ = changed
	})

	t.Run("applySingleFileFlattening", func(t *testing.T) {
		changed, err := pipeline.applySingleFileFlattening(inputFile, tempProcessDir, opts, results)
		if err != nil {
			t.Errorf("applySingleFileFlattening failed: %v", err)
		}
		_ = changed
	})

	t.Run("applySingleFileVendorExtensions", func(t *testing.T) {
		changed, err := pipeline.applySingleFileVendorExtensions(inputFile, tempProcessDir, opts, results)
		if err != nil {
			t.Errorf("applySingleFileVendorExtensions failed: %v", err)
		}
		_ = changed
	})

	t.Run("applySingleFileDefaults", func(t *testing.T) {
		changed, err := pipeline.applySingleFileDefaults(inputFile, tempProcessDir, opts, results)
		if err != nil {
			t.Errorf("applySingleFileDefaults failed: %v", err)
		}
		_ = changed
	})
}

func TestPipelineWithDryRun(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cfg := &config.Config{
		Mappings: map[string]string{"x-operation-group-name": "x-fern-sdk-group-name"},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, true, false, outputFile)

	results, err := pipeline.ExecuteFullPipeline(inputFile)
	if err != nil {
		t.Fatalf("ExecuteFullPipeline with dry-run failed: %v", err)
	}

	// In dry-run mode, output file should not be created
	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Error("Expected output file not to be created in dry-run mode")
	}

	// Results should still indicate changes would be made
	if !results.AnyTransformations {
		t.Error("Expected transformations to be detected in dry-run mode")
	}
}

func TestPipelineDisabledFeatures(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.yaml")

	// Create test input file
	inputContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      x-operation-group-name: users
`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Config with all features disabled
	cfg := &config.Config{
		PaginationPriority: []string{}, // Empty = disabled
		FlattenResponses:   false,
		VendorExtensions: config.VendorExtensions{
			Enabled: false,
		},
		DefaultValues: config.DefaultValues{
			Enabled: false,
		},
	}

	pipeline := NewTransformationPipeline(cfg, []string{}, false, false, "")

	opts := Options{
		Mappings: map[string]string{},
		Exclude:  []string{},
		DryRun:   false,
		Backup:   false,
	}

	results := &TransformationResults{
		Changed: []string{},
	}

	// Test that disabled features don't cause errors
	t.Run("disabled_pagination", func(t *testing.T) {
		err := pipeline.applyPaginationStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyPaginationStep with disabled feature failed: %v", err)
		}
	})

	t.Run("disabled_flattening", func(t *testing.T) {
		err := pipeline.applyFlatteningStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyFlatteningStep with disabled feature failed: %v", err)
		}
	})

	t.Run("disabled_vendor_extensions", func(t *testing.T) {
		err := pipeline.applyVendorExtensionsStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyVendorExtensionsStep with disabled feature failed: %v", err)
		}
	})

	t.Run("disabled_defaults", func(t *testing.T) {
		err := pipeline.applyDefaultsStep(inputFile, opts, results)
		if err != nil {
			t.Errorf("applyDefaultsStep with disabled feature failed: %v", err)
		}
	})
}
