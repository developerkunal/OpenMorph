package transform

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FlattenOptions extends the regular Options with flattening-specific settings
type FlattenOptions struct {
	Options
	FlattenResponses bool
}

// FlattenResult represents the result of flattening processing
type FlattenResult struct {
	Changed           bool
	ProcessedFiles    []string
	FlattenedRefs     map[string][]string // file -> flattened reference paths
	RemovedComponents map[string][]string // file -> removed component names
}

// ProcessFlatteningInDir processes response flattening in all OpenAPI files in a directory
func ProcessFlatteningInDir(dir string, opts FlattenOptions) (*FlattenResult, error) {
	result := &FlattenResult{
		ProcessedFiles:    []string{},
		FlattenedRefs:     make(map[string][]string),
		RemovedComponents: make(map[string][]string),
	}

	if !opts.FlattenResponses {
		return result, nil // No flattening configured
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if IsYAML(path) || IsJSON(path) {
			changed, err := processFlatteningInFile(path, opts, result)
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

// processFlatteningInFile processes flattening in a single file
func processFlatteningInFile(path string, opts FlattenOptions, result *FlattenResult) (bool, error) {
	doc, err := loadAndParseDocument(path)
	if err != nil {
		return false, err
	}

	root := getRootNode(doc)

	if !isOpenAPIDocument(root) {
		return false, nil // Skip non-OpenAPI files
	}

	return processDocumentFlattening(doc, root, path, opts, result)
}

// processDocumentFlattening processes flattening in a document
func processDocumentFlattening(doc, root *yaml.Node, path string, opts FlattenOptions, result *FlattenResult) (bool, error) {
	// Track component references before flattening to identify unused ones later
	componentsBefore := extractComponentRefs(root)

	// First pass: flatten oneOf/anyOf/allOf with single refs
	changed := false
	processComponentsFlattening(root, path, result, &changed)
	processPathsFlattening(root, path, result, &changed)

	// Second pass: flatten reference chains (optional, more aggressive)
	if opts.FlattenResponses {
		if flattenReferenceChains(root, path, result, &changed) {
			changed = true
		}
	}

	if changed {
		// Third pass: clean up unused components after flattening
		componentsAfter := extractComponentRefs(root)
		unused := findUnusedComponents(root, componentsBefore, componentsAfter)
		if len(unused) > 0 {
			removeUnusedComponents(root, unused)
			// Record the removed components
			if result.RemovedComponents == nil {
				result.RemovedComponents = make(map[string][]string)
			}
			result.RemovedComponents[path] = unused
		}

		// Only write to file if not in dry-run mode
		if opts.DryRun {
			return true, nil // Return true to indicate changes were detected, but don't write
		}

		return writeModifiedDocument(doc, path)
	}

	return false, nil
}

// flattenReferenceChains flattens chains of references to point directly to final targets
func flattenReferenceChains(root *yaml.Node, filePath string, result *FlattenResult, changed *bool) bool {
	// Build a map of schema name to its direct reference (if it's just a $ref)
	refMap := buildDirectRefMap(root)

	if len(refMap) == 0 {
		return false
	}
	// Flatten reference chains in components/schemas
	// Capture the result of the first flattening operation
	schemaChanged := flattenSchemaReferences(root, refMap, filePath, result)

	// Flatten reference chains in paths
	// Capture the result of the second flattening operation
	pathChanged := flattenPathReferences(root, refMap, filePath, result)

	// Combine the results: localChanged is true if either operation made a change
	localChanged := schemaChanged || pathChanged

	if localChanged {
		*changed = true
	}
	return localChanged
}

// buildDirectRefMap builds a map of schema names that are direct references
func buildDirectRefMap(root *yaml.Node) map[string]string {
	refMap := make(map[string]string)

	components := getNodeValue(root, "components")
	if components == nil {
		return refMap
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return refMap
	}

	for i := 0; i < len(schemas.Content); i += 2 {
		schemaName := schemas.Content[i].Value
		schemaNode := schemas.Content[i+1]

		if schemaNode.Kind == yaml.MappingNode {
			// Check if this schema is just a direct $ref
			if refValue := getDirectRef(schemaNode); refValue != "" {
				refMap[schemaName] = refValue
			}
		}
	}

	return refMap
}

// getDirectRef returns the $ref value if the node is just a direct reference
func getDirectRef(node *yaml.Node) string {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return ""
	}

	if node.Content[0].Value == "$ref" {
		return node.Content[1].Value
	}

	return ""
}

// flattenSchemaReferences flattens reference chains in schemas
func flattenSchemaReferences(root *yaml.Node, refMap map[string]string, filePath string, result *FlattenResult) bool {
	localChanged := false

	components := getNodeValue(root, "components")
	if components == nil {
		return false
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i < len(schemas.Content); i += 2 {
		schemaName := schemas.Content[i].Value
		schemaNode := schemas.Content[i+1]

		if updateReferencesInNode(schemaNode, refMap, filePath, result, schemaName) {
			localChanged = true
		}
	}

	return localChanged
}

// flattenPathReferences flattens reference chains in paths
func flattenPathReferences(root *yaml.Node, refMap map[string]string, filePath string, result *FlattenResult) bool {
	localChanged := false

	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if updateReferencesInNode(pathNode, refMap, filePath, result, pathName) {
			localChanged = true
		}
	}

	return localChanged
}

// updateReferencesInNode recursively updates $ref values in a node
func updateReferencesInNode(node *yaml.Node, refMap map[string]string, filePath string, result *FlattenResult, context string) bool {
	if node == nil {
		return false
	}

	localChanged := false

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			value := node.Content[i+1]

			if key == "$ref" && value.Kind == yaml.ScalarNode {
				// Check if this reference can be flattened
				if newRef := resolveReferenceChain(value.Value, refMap); newRef != value.Value {
					value.Value = newRef
					localChanged = true

					// Record the flattening
					if result.FlattenedRefs[filePath] == nil {
						result.FlattenedRefs[filePath] = []string{}
					}
					result.FlattenedRefs[filePath] = append(result.FlattenedRefs[filePath],
						fmt.Sprintf("%s: %s -> %s", context, value.Value, newRef))
				}
			} else {
				if updateReferencesInNode(value, refMap, filePath, result, context) {
					localChanged = true
				}
			}
		}

	case yaml.SequenceNode:
		for _, item := range node.Content {
			if updateReferencesInNode(item, refMap, filePath, result, context) {
				localChanged = true
			}
		}
	}

	return localChanged
}

// resolveReferenceChain resolves a reference chain to its final target
func resolveReferenceChain(ref string, refMap map[string]string) string {
	// Extract schema name from reference like "#/components/schemas/SchemaName"
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return ref
	}

	schemaName := ref[len(prefix):]
	visited := make(map[string]bool)

	for {
		if visited[schemaName] {
			// Circular reference, return current ref
			return ref
		}

		visited[schemaName] = true

		if targetRef, exists := refMap[schemaName]; exists {
			// Extract target schema name
			if strings.HasPrefix(targetRef, prefix) {
				targetSchemaName := targetRef[len(prefix):]
				schemaName = targetSchemaName
				ref = targetRef
			} else {
				// Not a schema reference, stop here
				return ref
			}
		} else {
			// No further reference found, this is the final target
			return ref
		}
	}
}

// processComponentsFlattening processes flattening in the components section
func processComponentsFlattening(root *yaml.Node, path string, result *FlattenResult, changed *bool) bool {
	components := getNodeValue(root, "components")
	if components == nil || components.Kind != yaml.MappingNode {
		return false
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return false
	}

	localChanged := false
	for i := 0; i < len(schemas.Content); i += 2 {
		schemaName := schemas.Content[i].Value
		schemaNode := schemas.Content[i+1]

		if flattenSchemaNode(schemaNode, schemaName, path, result) {
			localChanged = true
		}
	}

	if localChanged {
		*changed = true
	}
	return localChanged
}

// processPathsFlattening processes flattening in the paths section
func processPathsFlattening(root *yaml.Node, path string, result *FlattenResult, changed *bool) bool {
	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	localChanged := false
	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if pathNode.Kind != yaml.MappingNode {
			continue
		}

		if flattenPathNode(pathNode, pathName, path, result) {
			localChanged = true
		}
	}

	if localChanged {
		*changed = true
	}
	return localChanged
}

// flattenSchemaNode flattens oneOf/anyOf/allOf in a schema node
func flattenSchemaNode(node *yaml.Node, schemaName, path string, result *FlattenResult) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}

	changed := false

	// Check for oneOf, anyOf, allOf and try to flatten them
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]

		if key == "oneOf" || key == "anyOf" || key == "allOf" {
			if refValue := getSingleRefFromArray(value); refValue != "" {
				// Replace the oneOf/anyOf/allOf with direct $ref
				node.Content[i] = &yaml.Node{Kind: yaml.ScalarNode, Value: "$ref"}
				node.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: refValue}

				// Record the flattening
				flattenedPath := fmt.Sprintf("%s.%s -> $ref: %s", schemaName, key, refValue)
				if result.FlattenedRefs[path] == nil {
					result.FlattenedRefs[path] = []string{}
				}
				result.FlattenedRefs[path] = append(result.FlattenedRefs[path], flattenedPath)
				changed = true
			}
		} else if value.Kind == yaml.MappingNode {
			// Recursively process nested objects
			if flattenSchemaNode(value, schemaName, path, result) {
				changed = true
			}
		} else if value.Kind == yaml.SequenceNode {
			// Process arrays
			for _, item := range value.Content {
				if flattenSchemaNode(item, schemaName, path, result) {
					changed = true
				}
			}
		}
	}

	return changed
}

// flattenPathNode flattens oneOf/anyOf/allOf in path responses
func flattenPathNode(node *yaml.Node, pathName, path string, result *FlattenResult) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}

	changed := false

	// Process each HTTP method
	for i := 0; i < len(node.Content); i += 2 {
		method := node.Content[i].Value
		methodNode := node.Content[i+1]

		if methodNode.Kind == yaml.MappingNode {
			responses := getNodeValue(methodNode, "responses")
			if responses != nil && responses.Kind == yaml.MappingNode {
				if flattenResponsesNode(responses, fmt.Sprintf("%s %s", method, pathName), path, result) {
					changed = true
				}
			}
		}
	}

	return changed
}

// flattenResponsesNode flattens oneOf/anyOf/allOf in responses
func flattenResponsesNode(node *yaml.Node, operation, path string, result *FlattenResult) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}

	changed := false

	// Process each response code
	for i := 0; i < len(node.Content); i += 2 {
		responseCode := node.Content[i].Value
		responseNode := node.Content[i+1]

		if responseNode.Kind == yaml.MappingNode {
			content := getNodeValue(responseNode, "content")
			if content != nil && content.Kind == yaml.MappingNode {
				// Process each media type
				for j := 0; j < len(content.Content); j += 2 {
					mediaType := content.Content[j].Value
					mediaNode := content.Content[j+1]

					if mediaNode.Kind == yaml.MappingNode {
						schema := getNodeValue(mediaNode, "schema")
						if schema != nil {
							schemaPath := fmt.Sprintf("%s -> %s -> %s", operation, responseCode, mediaType)
							if flattenSchemaNode(schema, schemaPath, path, result) {
								changed = true
							}
						}
					}
				}
			}
		}
	}

	return changed
}

// getSingleRefFromArray checks if an array contains only one $ref and returns it
func getSingleRefFromArray(arrayNode *yaml.Node) string {
	if arrayNode == nil || arrayNode.Kind != yaml.SequenceNode {
		return ""
	}

	// Check if array has exactly one element
	if len(arrayNode.Content) != 1 {
		return ""
	}

	element := arrayNode.Content[0]
	if element.Kind != yaml.MappingNode {
		return ""
	}

	// Check if the element contains only a $ref
	var refValue string
	hasOnlyRef := true
	for i := 0; i < len(element.Content); i += 2 {
		key := element.Content[i].Value
		value := element.Content[i+1].Value

		if key == "$ref" {
			refValue = value
		} else {
			// If there are other properties besides $ref, don't flatten
			hasOnlyRef = false
			break
		}
	}

	if !hasOnlyRef || refValue == "" {
		return ""
	}

	return refValue
}
