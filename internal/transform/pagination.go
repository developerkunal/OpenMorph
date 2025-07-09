package transform

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/developerkunal/OpenMorph/internal/config"
	"github.com/developerkunal/OpenMorph/internal/pagination"
)

// PaginationOptions extends the regular Options with pagination-specific settings
type PaginationOptions struct {
	Options
	PaginationPriority []string
	EndpointRules      []config.EndpointPaginationRule
}

// convertEndpointRules converts config.EndpointPaginationRule to pagination.EndpointPaginationRule
func convertEndpointRules(configRules []config.EndpointPaginationRule) []pagination.EndpointPaginationRule {
	var paginationRules []pagination.EndpointPaginationRule
	for _, rule := range configRules {
		paginationRules = append(paginationRules, pagination.EndpointPaginationRule{
			Endpoint:   rule.Endpoint,
			Method:     rule.Method,
			Pagination: rule.Pagination,
		})
	}
	return paginationRules
}

// PaginationResult represents the result of pagination processing
type PaginationResult struct {
	Changed          bool
	ProcessedFiles   []string
	RemovedParams    map[string][]string // file -> removed param names
	RemovedResponses map[string][]string // file -> removed response codes
	ModifiedSchemas  map[string][]string // file -> modified schema paths
	UnusedComponents []string            // components that became unused
}

// ProcessPaginationInDir processes pagination in all OpenAPI files in a directory
func ProcessPaginationInDir(dir string, opts PaginationOptions) (*PaginationResult, error) {
	result := &PaginationResult{
		ProcessedFiles:   []string{},
		RemovedParams:    make(map[string][]string),
		RemovedResponses: make(map[string][]string),
		ModifiedSchemas:  make(map[string][]string),
		UnusedComponents: []string{},
	}

	if len(opts.PaginationPriority) == 0 {
		return result, nil // No pagination priority configured
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if IsYAML(path) || IsJSON(path) {
			changed, err := processPaginationInFile(path, opts, result)
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

// processPaginationInFile processes pagination in a single file
func processPaginationInFile(path string, opts PaginationOptions, result *PaginationResult) (bool, error) {
	doc, err := loadAndParseDocument(path)
	if err != nil {
		return false, err
	}

	root := getRootNode(doc)

	if !isOpenAPIDocument(root) {
		return false, nil // Skip non-OpenAPI files
	}

	return processDocumentPagination(doc, root, path, opts, result)
}

// loadAndParseDocument loads and parses a YAML/JSON document
func loadAndParseDocument(path string) (*yaml.Node, error) {
	orig, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(orig, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML/JSON: %w", err)
	}

	return &doc, nil
}

// getRootNode extracts the root node from a document
func getRootNode(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

// processDocumentPagination processes pagination in a document
func processDocumentPagination(doc, root *yaml.Node, path string, opts PaginationOptions, result *PaginationResult) (bool, error) {
	componentsBefore := extractComponentRefs(root)

	changed := processPaginationInPaths(root, opts, path, result)

	if changed {
		return handleDocumentChanges(doc, root, path, componentsBefore, result, opts)
	}

	return false, nil
}

// handleDocumentChanges handles post-processing and file writing after pagination changes
func handleDocumentChanges(doc, root *yaml.Node, path string, componentsBefore map[string]bool, result *PaginationResult, opts PaginationOptions) (bool, error) {
	componentsAfter := extractComponentRefs(root)
	unused := findUnusedComponents(root, componentsBefore, componentsAfter)
	if len(unused) > 0 {
		removeUnusedComponents(root, unused)
		result.UnusedComponents = append(result.UnusedComponents, unused...)
	}

	// Only write to file if not in dry-run mode
	if opts.DryRun {
		return true, nil // Return true to indicate changes were detected, but don't write
	}

	return writeModifiedDocument(doc, path)
}

// writeModifiedDocument writes the modified document back to file
func writeModifiedDocument(doc *yaml.Node, path string) (bool, error) {
	var output []byte
	var err error

	if IsJSON(path) {
		output, err = formatAsJSON(doc)
	} else {
		output, err = formatAsYAML(doc)
	}

	if err != nil {
		return false, err
	}

	if err := os.WriteFile(path, output, 0600); err != nil {
		return false, fmt.Errorf("failed to write file: %w", err)
	}

	return true, nil
}

// formatAsJSON formats document as JSON
func formatAsJSON(doc *yaml.Node) ([]byte, error) {
	yamlOutput, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %w", err)
	}

	return yamlToFormattedJSON(yamlOutput)
}

// formatAsYAML formats document as YAML
func formatAsYAML(doc *yaml.Node) ([]byte, error) {
	return yaml.Marshal(doc)
}

// isOpenAPIDocument checks if the document is an OpenAPI specification
func isOpenAPIDocument(root *yaml.Node) bool {
	if root.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i].Value
		if key == "openapi" || key == "swagger" {
			return true
		}
	}
	return false
}

// processPaginationInPaths processes pagination in the paths section
func processPaginationInPaths(root *yaml.Node, opts PaginationOptions, _ string, result *PaginationResult) bool {
	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	changed := false
	paginationOpts := pagination.Options{
		Priority:      opts.PaginationPriority,
		EndpointRules: convertEndpointRules(opts.EndpointRules),
	}

	return processPathsAndOperations(paths, paginationOpts, root, result, &changed)
}

// processPathsAndOperations processes all paths and their operations
func processPathsAndOperations(paths *yaml.Node, paginationOpts pagination.Options, root *yaml.Node, result *PaginationResult, changed *bool) bool {
	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if pathNode.Kind != yaml.MappingNode {
			continue
		}

		processOperationsInPath(pathNode, pathName, paginationOpts, root, result, changed)
	}

	return *changed
}

// processOperationsInPath processes all operations in a single path
func processOperationsInPath(pathNode *yaml.Node, pathName string, paginationOpts pagination.Options, root *yaml.Node, result *PaginationResult, changed *bool) {
	for j := 0; j < len(pathNode.Content); j += 2 {
		operation := pathNode.Content[j].Value
		operationNode := pathNode.Content[j+1]

		if !isHTTPMethod(operation) {
			continue
		}

		processOperation(operation, operationNode, pathName, paginationOpts, root, result, changed)
	}
}

// processOperation processes a single operation
func processOperation(operation string, operationNode *yaml.Node, pathName string, paginationOpts pagination.Options, root *yaml.Node, result *PaginationResult, changed *bool) {
	operationResult, err := pagination.ProcessEndpointWithPathAndMethod(operationNode, root, pathName, operation, paginationOpts)
	if err != nil {
		fmt.Printf("Warning: failed to process %s %s: %v\n", operation, pathName, err)
		return
	}

	if operationResult.Changed {
		*changed = true
		recordOperationChanges(operation, pathName, operationResult, result)
	}
}

// recordOperationChanges records changes made to an operation
func recordOperationChanges(operation, pathName string, operationResult *pagination.ProcessResult, result *PaginationResult) {
	key := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)

	if len(operationResult.RemovedParams) > 0 {
		result.RemovedParams[key] = operationResult.RemovedParams
	}

	if len(operationResult.RemovedResponses) > 0 {
		result.RemovedResponses[key] = operationResult.RemovedResponses
	}

	if len(operationResult.ModifiedSchemas) > 0 {
		result.ModifiedSchemas[key] = operationResult.ModifiedSchemas
	}
}

// isHTTPMethod checks if a string is an HTTP method
func isHTTPMethod(method string) bool {
	methods := []string{"get", "post", "put", "delete", "patch", "head", "options", "trace"}
	method = strings.ToLower(method)
	for _, m := range methods {
		if method == m {
			return true
		}
	}
	return false
}

// extractComponentRefs extracts all component references from the document
func extractComponentRefs(root *yaml.Node) map[string]bool {
	refs := make(map[string]bool)
	extractRefsFromNode(root, refs)
	return refs
}

// extractRefsFromNode recursively extracts $ref values from a YAML node
func extractRefsFromNode(node *yaml.Node, refs map[string]bool) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		extractRefsFromMapping(node, refs)
	case yaml.SequenceNode:
		extractRefsFromSequence(node, refs)
	}
}

// extractRefsFromMapping extracts refs from mapping nodes
func extractRefsFromMapping(node *yaml.Node, refs map[string]bool) {
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]

		if key == "$ref" && value.Value != "" {
			refs[value.Value] = true
		} else {
			extractRefsFromNode(value, refs)
		}
	}
}

// extractRefsFromSequence extracts refs from sequence nodes
func extractRefsFromSequence(node *yaml.Node, refs map[string]bool) {
	for _, item := range node.Content {
		extractRefsFromNode(item, refs)
	}
}

// findUnusedComponents compares before and after component usage
func findUnusedComponents(root *yaml.Node, before, after map[string]bool) []string {
	var unused []string

	components := getNodeValue(root, "components")
	if components == nil {
		return unused
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return unused
	}

	return findUnusedSchemas(schemas, before, after)
}

// findUnusedSchemas finds schemas that are no longer used
func findUnusedSchemas(schemas *yaml.Node, before, after map[string]bool) []string {
	var unused []string

	for i := 0; i < len(schemas.Content); i += 2 {
		schemaName := schemas.Content[i].Value
		ref := "#/components/schemas/" + schemaName

		if before[ref] && !after[ref] {
			unused = append(unused, schemaName)
		}
	}

	return unused
}

// removeUnusedComponents removes unused component schemas
func removeUnusedComponents(root *yaml.Node, unused []string) {
	if len(unused) == 0 {
		return
	}

	components := getNodeValue(root, "components")
	if components == nil {
		return
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return
	}

	filterUnusedSchemas(schemas, unused)
}

// filterUnusedSchemas removes unused schemas from the schemas node
func filterUnusedSchemas(schemas *yaml.Node, unused []string) {
	unusedSet := make(map[string]bool)
	for _, name := range unused {
		unusedSet[name] = true
	}

	var newContent []*yaml.Node
	for i := 0; i < len(schemas.Content); i += 2 {
		schemaName := schemas.Content[i].Value
		schemaNode := schemas.Content[i+1]

		if !unusedSet[schemaName] {
			newContent = append(newContent, schemas.Content[i], schemaNode)
		}
	}

	schemas.Content = newContent
}

// getNodeValue gets a node value by key
func getNodeValue(node *yaml.Node, key string) *yaml.Node {
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

// yamlToFormattedJSON converts YAML output to properly formatted JSON while preserving field order
func yamlToFormattedJSON(yamlData []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(yamlData, &node); err != nil {
		return nil, err
	}

	return yamlNodeToJSON(&node, 0)
}

// yamlNodeToJSON recursively converts a yaml.Node to formatted JSON
func yamlNodeToJSON(node *yaml.Node, indent int) ([]byte, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		return handleDocumentNode(node, indent)
	case yaml.MappingNode:
		return handleMappingNode(node, indent)
	case yaml.SequenceNode:
		return handleSequenceNode(node, indent)
	case yaml.ScalarNode:
		return handleScalarNode(node)
	default:
		return []byte("null"), nil
	}
}

// handleDocumentNode handles document nodes
func handleDocumentNode(node *yaml.Node, indent int) ([]byte, error) {
	if len(node.Content) > 0 {
		return yamlNodeToJSON(node.Content[0], indent)
	}
	return []byte("null"), nil
}

// handleMappingNode handles mapping nodes (objects)
func handleMappingNode(node *yaml.Node, indent int) ([]byte, error) {
	if len(node.Content) == 0 {
		return []byte("{}"), nil
	}

	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	parts := make([]string, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		part, err := formatKeyValuePair(node.Content[i], node.Content[i+1], nextIndentStr, indent+1)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}

	if indent == 0 {
		return []byte("{\n" + strings.Join(parts, ",\n") + "\n}"), nil
	}
	return []byte("{\n" + strings.Join(parts, ",\n") + "\n" + indentStr + "}"), nil
}

// handleSequenceNode handles sequence nodes (arrays)
func handleSequenceNode(node *yaml.Node, indent int) ([]byte, error) {
	if len(node.Content) == 0 {
		return []byte("[]"), nil
	}

	// Always use multi-line formatting for consistency
	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	parts := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		itemJSON, err := yamlNodeToJSON(item, indent+1)
		if err != nil {
			return nil, err
		}
		parts = append(parts, nextIndentStr+string(itemJSON))
	}

	if indent == 0 {
		return []byte("[\n" + strings.Join(parts, ",\n") + "\n]"), nil
	}
	return []byte("[\n" + strings.Join(parts, ",\n") + "\n" + indentStr + "]"), nil
}

// handleScalarNode handles scalar nodes (primitives)
func handleScalarNode(node *yaml.Node) ([]byte, error) {
	switch node.Tag {
	case "!!null", "":
		return []byte("null"), nil
	case "!!bool":
		return []byte(node.Value), nil
	case "!!int", "!!float":
		return []byte(node.Value), nil
	case "!!str":
		return []byte(escapeJSONString(node.Value)), nil
	default:
		return []byte(escapeJSONString(node.Value)), nil
	}
}

// formatKeyValuePair formats a single key-value pair
func formatKeyValuePair(key, value *yaml.Node, nextIndentStr string, nextIndent int) (string, error) {
	keyJSON := fmt.Sprintf("\"%s\"", key.Value)

	valueJSON, err := yamlNodeToJSON(value, nextIndent)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s: %s", nextIndentStr, keyJSON, string(valueJSON)), nil
}

// escapeJSONString properly escapes a string for JSON output
func escapeJSONString(s string) string {
	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(s)

	result := buf.String()
	return strings.TrimSuffix(result, "\n")
}
