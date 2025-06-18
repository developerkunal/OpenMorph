package transform

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/developerkunal/OpenMorph/internal/config"
	"github.com/developerkunal/OpenMorph/internal/pagination"
)

// VendorExtensionOptions extends the regular Options with vendor extension settings
type VendorExtensionOptions struct {
	Options
	VendorExtensions config.VendorExtensions
	EnabledProviders []string // specific providers to apply, empty means all
}

// VendorExtensionResult represents the result of vendor extension processing
type VendorExtensionResult struct {
	Changed           bool
	ProcessedFiles    []string
	AddedExtensions   map[string][]string // file -> list of added extensions
	SkippedOperations map[string][]string // file -> list of skipped operations with reasons
}

// ProcessVendorExtensionsInDir processes vendor extensions in all OpenAPI files in a directory
func ProcessVendorExtensionsInDir(dir string, opts VendorExtensionOptions) (*VendorExtensionResult, error) {
	result := &VendorExtensionResult{
		ProcessedFiles:    []string{},
		AddedExtensions:   make(map[string][]string),
		SkippedOperations: make(map[string][]string),
	}

	if !opts.VendorExtensions.Enabled {
		return result, nil // Feature not enabled
	}

	if len(opts.VendorExtensions.Providers) == 0 {
		return result, nil // No providers configured
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if IsYAML(path) || IsJSON(path) {
			changed, err := processVendorExtensionsInFile(path, opts, result)
			if err != nil {
				return fmt.Errorf("error processing %s: %w", path, err)
			}
			if changed {
				result.Changed = true
				result.ProcessedFiles = append(result.ProcessedFiles, path)
			}
		}
		return nil
	})

	return result, err
}

// processVendorExtensionsInFile processes vendor extensions in a single file
func processVendorExtensionsInFile(path string, opts VendorExtensionOptions, result *VendorExtensionResult) (bool, error) {
	doc, err := loadAndParseDocument(path)
	if err != nil {
		return false, err
	}

	root := getRootNode(doc)

	if !isOpenAPIDocument(root) {
		return false, nil // Skip non-OpenAPI files
	}

	return processDocumentVendorExtensions(doc, root, path, opts, result)
}

// processDocumentVendorExtensions processes vendor extensions in a document
func processDocumentVendorExtensions(doc, root *yaml.Node, path string, opts VendorExtensionOptions, result *VendorExtensionResult) (bool, error) {
	changed := processVendorExtensionsInPaths(root, opts, path, result)

	if changed {
		return writeVendorExtensionsDocument(doc, path, opts.DryRun)
	}

	return false, nil
}

// processVendorExtensionsInPaths processes vendor extensions in the paths section
func processVendorExtensionsInPaths(root *yaml.Node, opts VendorExtensionOptions, filePath string, result *VendorExtensionResult) bool {
	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	changed := false

	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if pathNode.Kind != yaml.MappingNode {
			continue
		}

		if processVendorOperationsInPath(pathNode, pathName, opts, root, filePath, result) {
			changed = true
		}
	}

	return changed
}

// processVendorOperationsInPath processes all operations in a single path
func processVendorOperationsInPath(pathNode *yaml.Node, pathName string, opts VendorExtensionOptions, root *yaml.Node, filePath string, result *VendorExtensionResult) bool {
	changed := false

	for j := 0; j < len(pathNode.Content); j += 2 {
		operation := pathNode.Content[j].Value
		operationNode := pathNode.Content[j+1]

		if !isHTTPMethod(operation) {
			continue
		}

		if processVendorOperation(operation, operationNode, pathName, opts, root, filePath, result) {
			changed = true
		}
	}

	return changed
}

// processVendorOperation processes a single operation for vendor extensions
func processVendorOperation(operation string, operationNode *yaml.Node, pathName string, opts VendorExtensionOptions, root *yaml.Node, filePath string, result *VendorExtensionResult) bool {
	changed := false
	operationKey := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)

	// Process each enabled provider
	for providerName, providerConfig := range opts.VendorExtensions.Providers {
		// Skip if specific providers are requested and this isn't one of them
		if len(opts.EnabledProviders) > 0 && !contains(opts.EnabledProviders, providerName) {
			continue
		}

		// Check if operation matches provider criteria
		if !operationMatchesProvider(operation, pathName, providerConfig) {
			addSkippedOperation(result, filePath, operationKey, fmt.Sprintf("doesn't match %s provider criteria", providerName))
			continue
		}

		// Detect pagination in this operation
		params := getVendorNodeValue(operationNode, "parameters")
		responses := getVendorNodeValue(operationNode, "responses")

		detected := pagination.DetectPaginationInParamsWithDoc(params, root)
		if len(detected) == 0 {
			addSkippedOperation(result, filePath, operationKey, fmt.Sprintf("no pagination detected for %s", providerName))
			continue
		}

		// Try to add vendor extension for each detected strategy
		for _, paginationInfo := range detected {
			if addVendorExtension(operationNode, paginationInfo, providerConfig, params, responses, root) {
				changed = true
				addProcessedExtension(result, filePath, fmt.Sprintf("%s: %s (%s strategy)", operationKey, providerConfig.ExtensionName, paginationInfo.Strategy))
			}
		}
	}

	return changed
}

// operationMatchesProvider checks if an operation matches provider criteria
func operationMatchesProvider(operation, pathName string, config config.ProviderConfig) bool {
	// Check HTTP methods
	if len(config.Methods) > 0 {
		methodMatch := false
		for _, method := range config.Methods {
			if strings.EqualFold(operation, method) {
				methodMatch = true
				break
			}
		}
		if !methodMatch {
			return false
		}
	}

	// Check path patterns
	if len(config.PathPatterns) > 0 {
		pathMatch := false
		for _, pattern := range config.PathPatterns {
			if matched, _ := filepath.Match(pattern, pathName); matched {
				pathMatch = true
				break
			}
			// Also try glob-style matching
			if globMatch(pathName, pattern) {
				pathMatch = true
				break
			}
		}
		if !pathMatch {
			return false
		}
	}

	return true
}

// addVendorExtension adds a vendor extension to an operation
func addVendorExtension(operationNode *yaml.Node, paginationInfo pagination.DetectedPagination, config config.ProviderConfig, params, responses *yaml.Node, root *yaml.Node) bool {
	strategyConfig, exists := config.Strategies[paginationInfo.Strategy]
	if !exists {
		return false
	}

	// Build template context
	context := buildTemplateContext(paginationInfo, config, params, responses, root)

	// Check if we have required fields
	if !hasRequiredFields(context, strategyConfig.RequiredFields) {
		return false
	}

	// Process template with context
	processedTemplate := processTemplate(strategyConfig.Template, context)

	// Add the vendor extension to the operation
	return addExtensionToOperation(operationNode, config.ExtensionName, processedTemplate)
}

// buildTemplateContext builds the context for template processing
func buildTemplateContext(paginationInfo pagination.DetectedPagination, config config.ProviderConfig, params, responses *yaml.Node, root *yaml.Node) map[string]string {
	context := make(map[string]string)

	// Map request parameters
	if params != nil {
		paramNames := extractParameterNames(params, root)
		for contextKey, possibleParams := range config.FieldMapping.RequestParams {
			for _, paramName := range paramNames {
				if contains(possibleParams, paramName) {
					context[contextKey+"_param"] = paramName
					break
				}
			}
		}
	}

	// Map response fields - first try from config, then auto-detect
	if responses != nil {
		responseFields := extractResponseFields(responses, root)

		// Try config-based mapping first
		for contextKey, possibleFields := range config.FieldMapping.ResponseFields {
			for _, fieldName := range responseFields {
				if contains(possibleFields, fieldName) {
					context[contextKey+"_field"] = fieldName
					break
				}
			}
		}

		// Auto-detect results fields if not found in config
		if _, hasResults := context["results_field"]; !hasResults {
			// Look for array fields in response schemas
			arrayFields := extractArrayFieldsFromResponses(responses, root)
			if len(arrayFields) > 0 {
				// Use the first array field found as the results field
				context["results_field"] = arrayFields[0]
			}
		}
	}

	return context
}

// processTemplate processes a template with the given context
func processTemplate(template map[string]interface{}, context map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range template {
		if strValue, ok := value.(string); ok {
			result[key] = substituteTemplate(strValue, context)
		} else {
			result[key] = value
		}
	}

	return result
}

// substituteTemplate substitutes template variables like $request.{cursor_param}
func substituteTemplate(template string, context map[string]string) string {
	// Replace $request.{param_name} and $response.{field_name}
	re := regexp.MustCompile(`\$(request|response)\.{([^}]+)}`)

	return re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract the prefix (request/response) and variable name from {variable_name}
		parts := re.FindStringSubmatch(match)
		if len(parts) > 2 {
			prefix := parts[1]  // "request" or "response"
			varName := parts[2] // "cursor_param", "results_field", etc.
			if value, exists := context[varName]; exists {
				return fmt.Sprintf("$%s.%s", prefix, value)
			}
		}
		return match // Return unchanged if no substitution found
	})
}

// addExtensionToOperation adds a vendor extension to an operation node
func addExtensionToOperation(operationNode *yaml.Node, extensionName string, extensionValue map[string]interface{}) bool {
	if operationNode.Kind != yaml.MappingNode {
		return false
	}

	// Check if extension already exists
	for i := 0; i < len(operationNode.Content); i += 2 {
		if operationNode.Content[i].Value == extensionName {
			// Extension already exists, skip
			return false
		}
	}

	// Create extension key node
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: extensionName,
	}

	// Create extension value node
	valueNode := createYAMLNodeFromMap(extensionValue)

	// Add to operation
	operationNode.Content = append(operationNode.Content, keyNode, valueNode)

	return true
}

// createYAMLNodeFromMap creates a YAML node from a map
func createYAMLNodeFromMap(data map[string]interface{}) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}

	for key, value := range data {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}

		var valueNode *yaml.Node
		if strValue, ok := value.(string); ok {
			valueNode = &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: strValue,
			}
		} else {
			// For more complex values, marshal and unmarshal
			valueNode = &yaml.Node{}
			_ = valueNode.Encode(value)
		}

		node.Content = append(node.Content, keyNode, valueNode)
	}

	return node
}

// Helper functions

func extractParameterNames(params *yaml.Node, root *yaml.Node) []string {
	var names []string

	if params == nil || params.Kind != yaml.SequenceNode {
		return names
	}

	for _, param := range params.Content {
		if param.Kind != yaml.MappingNode {
			continue
		}

		// Handle $ref
		if ref := getVendorNodeValue(param, "$ref"); ref != nil && root != nil {
			resolvedParam := resolveVendorRef(ref.Value, root)
			if resolvedParam != nil {
				if name := getVendorStringValue(resolvedParam, "name"); name != "" {
					names = append(names, name)
				}
			}
		} else {
			if name := getVendorStringValue(param, "name"); name != "" {
				names = append(names, name)
			}
		}
	}

	return names
}

func extractResponseFields(responses *yaml.Node, root *yaml.Node) []string {
	var fields []string

	if responses == nil || responses.Kind != yaml.MappingNode {
		return fields
	}

	// Look through success responses
	for i := 0; i < len(responses.Content); i += 2 {
		responseCode := responses.Content[i].Value
		responseNode := responses.Content[i+1]

		if isSuccessResponse(responseCode) {
			responseFields := extractFieldsFromResponseWithDoc(responseNode, root)
			fields = append(fields, responseFields...)
		}
	}

	return fields
}

func hasRequiredFields(context map[string]string, requiredFields []string) bool {
	for _, field := range requiredFields {
		if _, exists := context[field]; !exists {
			return false
		}
	}
	return true
}

func addProcessedExtension(result *VendorExtensionResult, filePath, extension string) {
	if result.AddedExtensions[filePath] == nil {
		result.AddedExtensions[filePath] = []string{}
	}
	result.AddedExtensions[filePath] = append(result.AddedExtensions[filePath], extension)
}

func addSkippedOperation(result *VendorExtensionResult, filePath, operation, reason string) {
	if result.SkippedOperations[filePath] == nil {
		result.SkippedOperations[filePath] = []string{}
	}
	result.SkippedOperations[filePath] = append(result.SkippedOperations[filePath], fmt.Sprintf("%s: %s", operation, reason))
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func globMatch(path, pattern string) bool {
	// Simple glob matching for patterns like "/api/v1/*"
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}

func writeVendorExtensionsDocument(doc *yaml.Node, path string, dryRun bool) (bool, error) {
	if dryRun {
		return true, nil // Return true to indicate changes were detected, but don't write
	}

	return writeModifiedDocument(doc, path)
}

// Reuse existing helper functions from pagination.go
func getVendorNodeValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func getVendorStringValue(node *yaml.Node, key string) string {
	valueNode := getVendorNodeValue(node, key)
	if valueNode != nil {
		return valueNode.Value
	}
	return ""
}

func isSuccessResponse(code string) bool {
	if matched, _ := regexp.MatchString(`^[23]\d\d$`, code); matched {
		return true
	}
	return code == "default"
}

func resolveVendorRef(refPath string, doc *yaml.Node) *yaml.Node {
	// Simple $ref resolution for #/components/...
	if !strings.HasPrefix(refPath, "#/") {
		return nil
	}

	path := strings.TrimPrefix(refPath, "#/")
	parts := strings.Split(path, "/")

	current := doc
	for _, part := range parts {
		current = getVendorNodeValue(current, part)
		if current == nil {
			return nil
		}
	}

	return current
}

func extractFieldsFromResponseWithDoc(response *yaml.Node, doc *yaml.Node) []string {
	var fields []string

	content := getVendorNodeValue(response, "content")
	if content == nil {
		return fields
	}

	// Walk through content types
	if content.Kind == yaml.MappingNode {
		for i := 1; i < len(content.Content); i += 2 {
			mediaTypeNode := content.Content[i]
			schema := getVendorNodeValue(mediaTypeNode, "schema")
			if schema != nil {
				fields = append(fields, extractFieldsFromSchemaWithDoc(schema, doc)...)
			}
		}
	}

	return fields
}

func extractFieldsFromSchemaWithDoc(schema *yaml.Node, doc *yaml.Node) []string {
	var fields []string

	if schema == nil || schema.Kind != yaml.MappingNode {
		return fields
	}

	// Handle $ref by resolving it
	if ref := getVendorNodeValue(schema, "$ref"); ref != nil {
		resolvedSchema := resolveVendorRef(ref.Value, doc)
		if resolvedSchema != nil {
			return extractFieldsFromSchemaWithDoc(resolvedSchema, doc)
		}
		return fields
	}

	// Handle direct properties
	if properties := getVendorNodeValue(schema, "properties"); properties != nil {
		fields = append(fields, extractFieldsFromProperties(properties)...)
	}

	// Handle oneOf, anyOf, allOf
	compositions := []string{"oneOf", "anyOf", "allOf"}
	for _, comp := range compositions {
		if composition := getVendorNodeValue(schema, comp); composition != nil {
			fields = append(fields, extractFieldsFromCompositionWithDoc(composition, doc)...)
		}
	}

	return fields
}

func extractFieldsFromProperties(properties *yaml.Node) []string {
	var fields []string

	if properties == nil || properties.Kind != yaml.MappingNode {
		return fields
	}

	for i := 0; i < len(properties.Content); i += 2 {
		fields = append(fields, properties.Content[i].Value)
	}

	return fields
}

func extractFieldsFromCompositionWithDoc(composition *yaml.Node, doc *yaml.Node) []string {
	var fields []string

	if composition == nil || composition.Kind != yaml.SequenceNode {
		return fields
	}

	for _, item := range composition.Content {
		fields = append(fields, extractFieldsFromSchemaWithDoc(item, doc)...)
	}

	return fields
}

// extractArrayFieldsFromResponses extracts all array fields from response schemas
func extractArrayFieldsFromResponses(responses *yaml.Node, root *yaml.Node) []string {
	var arrayFields []string

	if responses == nil || responses.Kind != yaml.MappingNode {
		return arrayFields
	}

	// Look through success responses
	for i := 0; i < len(responses.Content); i += 2 {
		responseCode := responses.Content[i].Value
		responseNode := responses.Content[i+1]

		if isSuccessResponse(responseCode) {
			fields := extractArrayFieldsFromResponseWithDoc(responseNode, root)
			arrayFields = append(arrayFields, fields...)
		}
	}

	return arrayFields
}

// extractArrayFieldsFromResponseWithDoc extracts array fields from a response node
func extractArrayFieldsFromResponseWithDoc(response *yaml.Node, doc *yaml.Node) []string {
	var arrayFields []string

	content := getVendorNodeValue(response, "content")
	if content == nil {
		return arrayFields
	}

	// Walk through content types
	if content.Kind == yaml.MappingNode {
		for i := 1; i < len(content.Content); i += 2 {
			mediaTypeNode := content.Content[i]
			schema := getVendorNodeValue(mediaTypeNode, "schema")
			if schema != nil {
				arrayFields = append(arrayFields, extractArrayFieldsFromSchemaWithDoc(schema, doc)...)
			}
		}
	}

	return arrayFields
}

// extractArrayFieldsFromSchemaWithDoc extracts array fields from a schema node
func extractArrayFieldsFromSchemaWithDoc(schema *yaml.Node, doc *yaml.Node) []string {
	var arrayFields []string

	if schema == nil || schema.Kind != yaml.MappingNode {
		return arrayFields
	}

	// Handle $ref by resolving it
	if ref := getVendorNodeValue(schema, "$ref"); ref != nil {
		resolvedSchema := resolveVendorRef(ref.Value, doc)
		if resolvedSchema != nil {
			return extractArrayFieldsFromSchemaWithDoc(resolvedSchema, doc)
		}
		return arrayFields
	}

	// Handle direct properties
	if properties := getVendorNodeValue(schema, "properties"); properties != nil {
		arrayFields = append(arrayFields, extractArrayFieldsFromProperties(properties, doc)...)
	}

	// Handle oneOf, anyOf, allOf
	compositions := []string{"oneOf", "anyOf", "allOf"}
	for _, comp := range compositions {
		if composition := getVendorNodeValue(schema, comp); composition != nil {
			arrayFields = append(arrayFields, extractArrayFieldsFromCompositionWithDoc(composition, doc)...)
		}
	}

	return arrayFields
}

// extractArrayFieldsFromProperties extracts array fields from a properties node
func extractArrayFieldsFromProperties(properties *yaml.Node, doc *yaml.Node) []string {
	var arrayFields []string

	if properties == nil || properties.Kind != yaml.MappingNode {
		return arrayFields
	}

	for i := 0; i < len(properties.Content); i += 2 {
		fieldName := properties.Content[i].Value
		fieldSchema := properties.Content[i+1]

		if isArrayField(fieldSchema, doc) {
			arrayFields = append(arrayFields, fieldName)
		}
	}

	return arrayFields
}

// extractArrayFieldsFromCompositionWithDoc extracts array fields from composition schemas (oneOf, anyOf, allOf)
func extractArrayFieldsFromCompositionWithDoc(composition *yaml.Node, doc *yaml.Node) []string {
	var arrayFields []string

	if composition == nil || composition.Kind != yaml.SequenceNode {
		return arrayFields
	}

	for _, item := range composition.Content {
		arrayFields = append(arrayFields, extractArrayFieldsFromSchemaWithDoc(item, doc)...)
	}

	return arrayFields
}

// isArrayField checks if a field schema defines an array type
func isArrayField(fieldSchema *yaml.Node, doc *yaml.Node) bool {
	if fieldSchema == nil || fieldSchema.Kind != yaml.MappingNode {
		return false
	}

	// Handle $ref by resolving it
	if ref := getVendorNodeValue(fieldSchema, "$ref"); ref != nil {
		resolvedSchema := resolveVendorRef(ref.Value, doc)
		if resolvedSchema != nil {
			return isArrayField(resolvedSchema, doc)
		}
		return false
	}

	// Check direct type
	if typeNode := getVendorNodeValue(fieldSchema, "type"); typeNode != nil {
		return typeNode.Value == "array"
	}

	// Check for array items property (implicit array type)
	if items := getVendorNodeValue(fieldSchema, "items"); items != nil {
		return true
	}

	return false
}
