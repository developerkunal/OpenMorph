package transform

import (
	"fmt"
	"os"

	"github.com/developerkunal/OpenMorph/internal/config"
)

// TransformationPipeline represents the complete transformation pipeline
type TransformationPipeline struct {
	Config          *config.Config
	VendorProviders []string
	DryRun          bool
	Backup          bool
	OutputFile      string
}

// TransformationResults aggregates results from all transformation steps
type TransformationResults struct {
	Changed            []string
	PaginationResult   *PaginationResult
	FlattenResult      *FlattenResult
	VendorResult       *VendorExtensionResult
	DefaultsResult     *DefaultsResult
	AnyTransformations bool
}

// normalizeResultPaths normalizes file paths in result structures to show the original input path
func normalizeResultPaths(inputPath string, files []string) []string {
	normalized := make([]string, len(files))
	for i := range files {
		normalized[i] = inputPath
	}
	return normalized
}

// normalizeMapKeys normalizes map keys to use the original input path
func normalizeMapKeys(inputPath string, originalMap map[string][]string) map[string][]string {
	if len(originalMap) == 0 {
		return originalMap
	}

	normalized := make(map[string][]string)
	for _, values := range originalMap {
		normalized[inputPath] = values
		break // Use only the first entry's values since all should be the same for single file
	}
	return normalized
}

// ExecuteFullPipeline runs the complete transformation pipeline in the correct order
func (tp *TransformationPipeline) ExecuteFullPipeline(inputPath string) (*TransformationResults, error) {
	// Determine if we're processing a single file or directory
	isOutputMode := tp.OutputFile != ""

	// For single file with output, use the enhanced single file processor
	if isOutputMode {
		return tp.executeSingleFileWithOutput(inputPath)
	}

	// For directory processing, execute each step in sequence
	return tp.executeDirectoryPipeline(inputPath)
}

// executeSingleFileWithOutput handles single file transformation with output file
// This method now returns detailed results like directory processing
func (tp *TransformationPipeline) executeSingleFileWithOutput(inputPath string) (*TransformationResults, error) {
	results := &TransformationResults{
		Changed: []string{},
	}

	// Setup temporary processing environment
	tempDir, tempFilePath, cleanup, err := tp.setupTempProcessing(inputPath)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Apply all transformations using the same pipeline as directory processing
	opts := Options{
		Mappings: tp.Config.Mappings,
		Exclude:  tp.Config.Exclude,
		DryRun:   false, // Process the temp file, not dry run
		Backup:   false, // No backup for temp files
	}

	anyChanges, err := tp.applySingleFileTransformations(inputPath, tempDir, tempFilePath, opts, results)
	if err != nil {
		return nil, err
	}

	// Copy result to output file if changes were made
	if anyChanges {
		results.Changed = append(results.Changed, inputPath)
		results.AnyTransformations = true

		if !tp.DryRun && tp.OutputFile != "" {
			transformedData, err := os.ReadFile(tempFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read transformed file: %v", err)
			}
			if err := os.WriteFile(tp.OutputFile, transformedData, 0600); err != nil {
				return nil, fmt.Errorf("failed to write output file: %v", err)
			}
		}
	}

	return results, nil
}

// setupTempProcessing creates temporary directory and file for processing
func (*TransformationPipeline) setupTempProcessing(inputPath string) (string, string, func(), error) {
	// Read the original file
	originalData, err := os.ReadFile(inputPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read input file: %v", err)
	}

	// Create a temporary directory for processing
	tempDir, err := os.MkdirTemp("", "openmorph_temp_*")
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create temp directory: %v", err)
	}

	// Create a temporary copy of the input file
	tempFilePath := tempDir + "/temp_input" + getFileExtension(inputPath)
	if err := os.WriteFile(tempFilePath, originalData, 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", "", nil, fmt.Errorf("failed to write temp file: %v", err)
	}

	cleanup := func() { os.RemoveAll(tempDir) }
	return tempDir, tempFilePath, cleanup, nil
}

// applySingleFileTransformations applies all transformation steps to a single file
func (tp *TransformationPipeline) applySingleFileTransformations(inputPath, tempDir, tempFilePath string, opts Options, results *TransformationResults) (bool, error) {
	var anyChanges bool

	// Step 1: Apply basic key mappings
	if len(tp.Config.Mappings) > 0 {
		fileChanged, err := FileWithChanges(tempFilePath, opts, nil)
		if err != nil {
			return false, fmt.Errorf("failed to apply mappings: %v", err)
		}
		if fileChanged {
			anyChanges = true
		}
	}

	// Apply remaining transformations using helper functions
	steps := []func(string, string, Options, *TransformationResults) (bool, error){
		tp.applySingleFilePagination,
		tp.applySingleFileFlattening,
		tp.applySingleFileVendorExtensions,
		tp.applySingleFileDefaults,
	}

	for _, step := range steps {
		changed, err := step(inputPath, tempDir, opts, results)
		if err != nil {
			return false, err
		}
		if changed {
			anyChanges = true
		}
	}

	return anyChanges, nil
}

// applySingleFilePagination applies pagination transformations to a single file
func (tp *TransformationPipeline) applySingleFilePagination(inputPath, tempDir string, opts Options, results *TransformationResults) (bool, error) {
	if len(tp.Config.PaginationPriority) == 0 {
		return false, nil
	}

	paginationOpts := PaginationOptions{
		Options:            opts,
		PaginationPriority: tp.Config.PaginationPriority,
		EndpointRules:      tp.Config.EndpointPagination,
	}
	paginationResult, err := ProcessPaginationInDir(tempDir, paginationOpts)
	if err != nil {
		return false, fmt.Errorf("failed to apply pagination: %v", err)
	}

	if paginationResult != nil {
		paginationResult.ProcessedFiles = normalizeResultPaths(inputPath, paginationResult.ProcessedFiles)
	}
	results.PaginationResult = paginationResult
	return paginationResult != nil && paginationResult.Changed, nil
}

// applySingleFileFlattening applies flattening transformations to a single file
func (tp *TransformationPipeline) applySingleFileFlattening(inputPath, tempDir string, opts Options, results *TransformationResults) (bool, error) {
	if !tp.Config.FlattenResponses {
		return false, nil
	}

	flattenOpts := FlattenOptions{
		Options:          opts,
		FlattenResponses: tp.Config.FlattenResponses,
	}
	flattenResult, err := ProcessFlatteningInDir(tempDir, flattenOpts)
	if err != nil {
		return false, fmt.Errorf("failed to apply flattening: %v", err)
	}

	if flattenResult != nil {
		flattenResult.ProcessedFiles = normalizeResultPaths(inputPath, flattenResult.ProcessedFiles)
		flattenResult.FlattenedRefs = normalizeMapKeys(inputPath, flattenResult.FlattenedRefs)
		flattenResult.RemovedComponents = normalizeMapKeys(inputPath, flattenResult.RemovedComponents)
	}
	results.FlattenResult = flattenResult
	return flattenResult != nil && flattenResult.Changed, nil
}

// applySingleFileVendorExtensions applies vendor extension transformations to a single file
func (tp *TransformationPipeline) applySingleFileVendorExtensions(inputPath, tempDir string, opts Options, results *TransformationResults) (bool, error) {
	if !tp.Config.VendorExtensions.Enabled || len(tp.Config.VendorExtensions.Providers) == 0 {
		return false, nil
	}

	vendorOpts := VendorExtensionOptions{
		Options:          opts,
		VendorExtensions: tp.Config.VendorExtensions,
		EnabledProviders: tp.VendorProviders,
	}
	vendorResult, err := ProcessVendorExtensionsInDir(tempDir, vendorOpts)
	if err != nil {
		return false, fmt.Errorf("failed to apply vendor extensions: %v", err)
	}

	if vendorResult != nil {
		vendorResult.ProcessedFiles = normalizeResultPaths(inputPath, vendorResult.ProcessedFiles)
		vendorResult.AddedExtensions = normalizeMapKeys(inputPath, vendorResult.AddedExtensions)
		vendorResult.SkippedOperations = normalizeMapKeys(inputPath, vendorResult.SkippedOperations)
	}
	results.VendorResult = vendorResult
	return vendorResult != nil && vendorResult.Changed, nil
}

// applySingleFileDefaults applies default values transformations to a single file
func (tp *TransformationPipeline) applySingleFileDefaults(inputPath, tempDir string, opts Options, results *TransformationResults) (bool, error) {
	if !tp.Config.DefaultValues.Enabled {
		return false, nil
	}

	defaultsOpts := DefaultsOptions{
		Options:       opts,
		DefaultValues: tp.Config.DefaultValues,
	}
	defaultsResult, err := ProcessDefaultsInDir(tempDir, defaultsOpts)
	if err != nil {
		return false, fmt.Errorf("failed to apply defaults: %v", err)
	}

	if defaultsResult != nil {
		defaultsResult.ProcessedFiles = normalizeResultPaths(inputPath, defaultsResult.ProcessedFiles)
		defaultsResult.AppliedDefaults = normalizeMapKeys(inputPath, defaultsResult.AppliedDefaults)
		defaultsResult.SkippedTargets = normalizeMapKeys(inputPath, defaultsResult.SkippedTargets)
	}
	results.DefaultsResult = defaultsResult
	return defaultsResult != nil && defaultsResult.Changed, nil
}

// executeDirectoryPipeline handles directory-based transformations
func (tp *TransformationPipeline) executeDirectoryPipeline(inputPath string) (*TransformationResults, error) {
	results := &TransformationResults{
		Changed: []string{},
	}

	// Step 1: Apply basic key mappings
	opts := Options{
		Mappings:   tp.Config.Mappings,
		Exclude:    tp.Config.Exclude,
		DryRun:     tp.DryRun,
		Backup:     tp.Backup,
		OutputFile: tp.OutputFile,
	}

	changed, err := Dir(inputPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply basic mappings: %v", err)
	}
	results.Changed = changed
	if len(changed) > 0 {
		results.AnyTransformations = true
	}

	// Step 2: Apply pagination transformations
	if err := tp.applyPaginationStep(inputPath, opts, results); err != nil {
		return nil, err
	}

	// Step 3: Apply response flattening
	if err := tp.applyFlatteningStep(inputPath, opts, results); err != nil {
		return nil, err
	}

	// Step 4: Apply vendor extensions
	if err := tp.applyVendorExtensionsStep(inputPath, opts, results); err != nil {
		return nil, err
	}

	// Step 5: Apply default values
	if err := tp.applyDefaultsStep(inputPath, opts, results); err != nil {
		return nil, err
	}

	return results, nil
}

// NewTransformationPipeline creates a new transformation pipeline
func NewTransformationPipeline(cfg *config.Config, vendorProviders []string, dryRun bool, backup bool, outputFile string) *TransformationPipeline {
	return &TransformationPipeline{
		Config:          cfg,
		VendorProviders: vendorProviders,
		DryRun:          dryRun,
		Backup:          backup,
		OutputFile:      outputFile,
	}
}

// applyPaginationStep applies pagination transformations
func (tp *TransformationPipeline) applyPaginationStep(inputPath string, opts Options, results *TransformationResults) error {
	if len(tp.Config.PaginationPriority) == 0 {
		return nil
	}

	paginationOpts := PaginationOptions{
		Options:            opts,
		PaginationPriority: tp.Config.PaginationPriority,
		EndpointRules:      tp.Config.EndpointPagination,
	}
	paginationResult, err := ProcessPaginationInDir(inputPath, paginationOpts)
	if err != nil {
		return fmt.Errorf("failed to apply pagination: %v", err)
	}
	results.PaginationResult = paginationResult
	if paginationResult.Changed {
		results.AnyTransformations = true
	}
	return nil
}

// applyFlatteningStep applies response flattening transformations
func (tp *TransformationPipeline) applyFlatteningStep(inputPath string, opts Options, results *TransformationResults) error {
	if !tp.Config.FlattenResponses {
		return nil
	}

	flattenOpts := FlattenOptions{
		Options:          opts,
		FlattenResponses: tp.Config.FlattenResponses,
	}
	flattenResult, err := ProcessFlatteningInDir(inputPath, flattenOpts)
	if err != nil {
		return fmt.Errorf("failed to apply flattening: %v", err)
	}
	results.FlattenResult = flattenResult
	if flattenResult.Changed {
		results.AnyTransformations = true
	}
	return nil
}

// applyVendorExtensionsStep applies vendor extension transformations
func (tp *TransformationPipeline) applyVendorExtensionsStep(inputPath string, opts Options, results *TransformationResults) error {
	if !tp.Config.VendorExtensions.Enabled {
		return nil
	}

	vendorOpts := VendorExtensionOptions{
		Options:          opts,
		VendorExtensions: tp.Config.VendorExtensions,
		EnabledProviders: tp.VendorProviders,
	}
	vendorResult, err := ProcessVendorExtensionsInDir(inputPath, vendorOpts)
	if err != nil {
		return fmt.Errorf("failed to apply vendor extensions: %v", err)
	}
	results.VendorResult = vendorResult
	if vendorResult.Changed {
		results.AnyTransformations = true
	}
	return nil
}

// applyDefaultsStep applies default values transformations
func (tp *TransformationPipeline) applyDefaultsStep(inputPath string, opts Options, results *TransformationResults) error {
	if !tp.Config.DefaultValues.Enabled {
		return nil
	}

	defaultsOpts := DefaultsOptions{
		Options:       opts,
		DefaultValues: tp.Config.DefaultValues,
	}
	defaultsResult, err := ProcessDefaultsInDir(inputPath, defaultsOpts)
	if err != nil {
		return fmt.Errorf("failed to apply defaults: %v", err)
	}
	results.DefaultsResult = defaultsResult
	if defaultsResult.Changed {
		results.AnyTransformations = true
	}
	return nil
}
