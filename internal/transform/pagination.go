package transform

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/developerkunal/OpenMorph/internal/pagination"
	"gopkg.in/yaml.v3"
)

// PaginationOptions extends the regular Options with pagination-specific settings
type PaginationOptions struct {
	Options
	PaginationPriority []string
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
	orig, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(orig, &doc); err != nil {
		return false, fmt.Errorf("failed to parse YAML/JSON: %w", err)
	}

	// Find the root document node
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}

	// Check if this is an OpenAPI document
	if !isOpenAPIDocument(root) {
		return false, nil // Skip non-OpenAPI files
	}

	// Track components before modification

	// Track components before modification
	componentsBefore := extractComponentRefs(root)

	// Process pagination in paths
	changed := processPaginationInPaths(root, opts, path, result)

	if changed {
		// Check for unused components after modification
		componentsAfter := extractComponentRefs(root)
		unused := findUnusedComponents(root, componentsBefore, componentsAfter)
		if len(unused) > 0 {
			// Remove unused components
			removeUnusedComponents(root, unused)
			result.UnusedComponents = append(result.UnusedComponents, unused...)
		}

		// Write the modified file with appropriate formatting
		var output []byte
		if IsJSON(path) {
			// For JSON files, preserve both formatting AND field order
			// Use yaml.Marshal but format as JSON - this preserves the yaml.Node order
			yamlOutput, err := yaml.Marshal(&doc)
			if err != nil {
				return false, fmt.Errorf("failed to marshal YAML: %w", err)
			}

			// Convert YAML to JSON while preserving order
			output, err = yamlToFormattedJSON(yamlOutput)
			if err != nil {
				return false, fmt.Errorf("failed to convert to JSON: %w", err)
			}
		} else {
			// For YAML files, use yaml.Marshal to preserve comments and structure
			output, err = yaml.Marshal(&doc)
			if err != nil {
				return false, fmt.Errorf("failed to marshal YAML: %w", err)
			}
		}

		if opts.DryRun {
			fmt.Printf("Would modify %s (pagination processing)\n", path)
			return true, nil
		}

		if opts.Backup {
			if err := os.WriteFile(path+".bak", orig, 0644); err != nil {
				return false, fmt.Errorf("failed to create backup: %w", err)
			}
		}

		if err := os.WriteFile(path, output, 0644); err != nil {
			return false, fmt.Errorf("failed to write file: %w", err)
		}
	}

	return changed, nil
}

// isOpenAPIDocument checks if the document is an OpenAPI specification
func isOpenAPIDocument(root *yaml.Node) bool {
	if root.Kind != yaml.MappingNode {
		return false
	}

	// Look for openapi or swagger version field
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i].Value
		if key == "openapi" || key == "swagger" {
			return true
		}
	}
	return false
}

// processPaginationInPaths processes pagination in the paths section
func processPaginationInPaths(root *yaml.Node, opts PaginationOptions, filePath string, result *PaginationResult) bool {
	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	changed := false
	paginationOpts := pagination.Options{Priority: opts.PaginationPriority}

	// Iterate through each path
	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if pathNode.Kind != yaml.MappingNode {
			continue
		}

		// Iterate through each operation (GET, POST, etc.)
		for j := 0; j < len(pathNode.Content); j += 2 {
			operation := pathNode.Content[j].Value
			operationNode := pathNode.Content[j+1]

			// Skip non-operation keys (like parameters, summary, etc.)
			if !isHTTPMethod(operation) {
				continue
			}

			// Process this operation
			operationResult, err := pagination.ProcessEndpointWithDoc(operationNode, root, paginationOpts)
			if err != nil {
				fmt.Printf("Warning: failed to process %s %s: %v\n", operation, pathName, err)
				continue
			}

			if operationResult.Changed {
				changed = true

				// Track what was changed
				if len(operationResult.RemovedParams) > 0 {
					key := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)
					result.RemovedParams[key] = operationResult.RemovedParams
				}

				if len(operationResult.RemovedResponses) > 0 {
					key := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)
					result.RemovedResponses[key] = operationResult.RemovedResponses
				}

				if len(operationResult.ModifiedSchemas) > 0 {
					key := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)
					result.ModifiedSchemas[key] = operationResult.ModifiedSchemas
				}
			}
		}
	}

	return changed
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
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			value := node.Content[i+1]

			if key == "$ref" && value.Value != "" {
				refs[value.Value] = true
			} else {
				extractRefsFromNode(value, refs)
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			extractRefsFromNode(item, refs)
		}
	}
}

// findUnusedComponents compares before and after component usage
func findUnusedComponents(root *yaml.Node, before, after map[string]bool) []string {
	var unused []string

	// Get all component definitions
	components := getNodeValue(root, "components")
	if components == nil {
		return unused
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil {
		return unused
	}

	// Check each schema component
	if schemas.Kind == yaml.MappingNode {
		for i := 0; i < len(schemas.Content); i += 2 {
			schemaName := schemas.Content[i].Value
			ref := "#/components/schemas/" + schemaName

			// If it was used before but not used after, it's now unused
			if before[ref] && !after[ref] {
				unused = append(unused, schemaName)
			}
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

	// Create set for faster lookup
	unusedSet := make(map[string]bool)
	for _, name := range unused {
		unusedSet[name] = true
	}

	// Filter out unused schemas
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

// Helper function to get a node value (already exists in pagination.go, redefining here for independence)
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
	// Parse the YAML data to preserve order
	var node yaml.Node
	if err := yaml.Unmarshal(yamlData, &node); err != nil {
		return nil, err
	}

	// Convert to JSON with proper formatting
	return yamlNodeToJSON(&node, 0)
}

// yamlNodeToJSON recursively converts a yaml.Node to formatted JSON
func yamlNodeToJSON(node *yaml.Node, indent int) ([]byte, error) {
	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return yamlNodeToJSON(node.Content[0], indent)
		}
		return []byte("null"), nil

	case yaml.MappingNode:
		if len(node.Content) == 0 {
			return []byte("{}"), nil
		}

		var parts []string
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			// Format key
			keyJSON := fmt.Sprintf("\"%s\"", key.Value)

			// Format value
			valueJSON, err := yamlNodeToJSON(value, indent+1)
			if err != nil {
				return nil, err
			}

			parts = append(parts, fmt.Sprintf("%s%s: %s", nextIndentStr, keyJSON, string(valueJSON)))
		}

		return []byte(fmt.Sprintf("{\n%s\n%s}", strings.Join(parts, ",\n"), indentStr)), nil

	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			return []byte("[]"), nil
		}

		var parts []string
		for _, item := range node.Content {
			itemJSON, err := yamlNodeToJSON(item, indent+1)
			if err != nil {
				return nil, err
			}
			parts = append(parts, fmt.Sprintf("%s%s", nextIndentStr, string(itemJSON)))
		}

		return []byte(fmt.Sprintf("[\n%s\n%s]", strings.Join(parts, ",\n"), indentStr)), nil

	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str":
			return []byte(escapeJSONString(node.Value)), nil
		case "!!int", "!!float":
			return []byte(node.Value), nil
		case "!!bool":
			return []byte(node.Value), nil
		case "!!null":
			return []byte("null"), nil
		default:
			// Default to string for unknown types
			return []byte(escapeJSONString(node.Value)), nil
		}

	default:
		return []byte("null"), nil
	}
}

// escapeJSONString properly escapes a string for JSON output
func escapeJSONString(s string) string {
	// Use Go's built-in JSON string escaping by marshaling a string
	// But disable HTML escaping to preserve HTML tags in OpenAPI descriptions
	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.Encode(s)

	// Remove the trailing newline that Encode adds
	result := buf.String()
	return strings.TrimSuffix(result, "\n")
}
