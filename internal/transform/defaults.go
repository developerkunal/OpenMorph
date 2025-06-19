package transform

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/developerkunal/OpenMorph/internal/config"
)

// DefaultsOptions extends the regular Options with default values settings
type DefaultsOptions struct {
	Options
	DefaultValues config.DefaultValues
}

// DefaultsResult represents the result of defaults processing
type DefaultsResult struct {
	Changed         bool
	ProcessedFiles  []string
	AppliedDefaults map[string][]string // file -> list of applied defaults
	SkippedTargets  map[string][]string // file -> list of skipped targets with reasons
}

// createDefaultsResult creates a new DefaultsResult with initialized maps
func createDefaultsResult() *DefaultsResult {
	return &DefaultsResult{
		ProcessedFiles:  []string{},
		AppliedDefaults: make(map[string][]string),
		SkippedTargets:  make(map[string][]string),
	}
}

// setDefaultsProcessedFiles sets the processed files for a DefaultsResult
func setDefaultsProcessedFiles(result *DefaultsResult, files []string) {
	result.ProcessedFiles = files
}

// setDefaultsChanged sets the changed flag for a DefaultsResult
func setDefaultsChanged(result *DefaultsResult, changed bool) {
	result.Changed = changed
}

// ProcessDefaultsInDir processes default values in all OpenAPI files in a directory
func ProcessDefaultsInDir(dir string, opts DefaultsOptions) (*DefaultsResult, error) {
	return processTransformInDir(
		dir,
		opts.DefaultValues.Enabled,
		len(opts.DefaultValues.Rules) == 0,
		createDefaultsResult,
		func(path string, result *DefaultsResult) (bool, error) {
			return processDefaultsInFile(path, opts, result)
		},
		setDefaultsProcessedFiles,
		setDefaultsChanged,
	)
}

// processDefaultsInFile processes default values in a single file
func processDefaultsInFile(path string, opts DefaultsOptions, result *DefaultsResult) (bool, error) {
	doc, err := loadAndParseDocument(path)
	if err != nil {
		return false, err
	}

	root := getRootNode(doc)

	if !isOpenAPIDocument(root) {
		return false, nil // Skip non-OpenAPI files
	}

	return processDocumentDefaults(doc, root, path, opts, result)
}

// processDocumentDefaults processes default values in a document
func processDocumentDefaults(doc, root *yaml.Node, path string, opts DefaultsOptions, result *DefaultsResult) (bool, error) {
	changed := false

	// Sort rules by priority (higher priority first)
	sortedRules := getSortedDefaultRules(opts.DefaultValues.Rules)

	for _, ruleEntry := range sortedRules {
		ruleName := ruleEntry.Name
		rule := ruleEntry.Rule

		switch rule.Target.Location {
		case "parameter":
			if processParameterDefaults(root, ruleName, rule, path, result) {
				changed = true
			}
		case "request_body":
			if processRequestBodyDefaults(root, ruleName, rule, path, result) {
				changed = true
			}
		case "response":
			if processResponseDefaults(root, ruleName, rule, path, result) {
				changed = true
			}
		case "component":
			if processComponentDefaults(root, ruleName, rule, path, result) {
				changed = true
			}
		}
	}

	if changed {
		return writeDefaultsDocument(doc, path, opts.DryRun)
	}

	return false, nil
}

// RuleEntry for sorting rules by priority
type RuleEntry struct {
	Name string
	Rule config.DefaultRule
}

// getSortedDefaultRules returns rules sorted by priority (higher first)
func getSortedDefaultRules(rules map[string]config.DefaultRule) []RuleEntry {
	var sortedRules []RuleEntry
	for name, rule := range rules {
		sortedRules = append(sortedRules, RuleEntry{Name: name, Rule: rule})
	}

	sort.Slice(sortedRules, func(i, j int) bool {
		return sortedRules[i].Rule.Priority > sortedRules[j].Rule.Priority
	})

	return sortedRules
}

// processParameterDefaults processes default values for parameters
func processParameterDefaults(root *yaml.Node, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	changed := false
	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if !matchesPathPattern(pathName, rule.Condition.PathPatterns) {
			continue
		}

		if processParametersInPath(pathNode, pathName, ruleName, rule, filePath, result) {
			changed = true
		}
	}

	return changed
}

// processParametersInPath processes parameters in a path for all operations
func processParametersInPath(pathNode *yaml.Node, pathName, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	changed := false

	for j := 0; j < len(pathNode.Content); j += 2 {
		operation := pathNode.Content[j].Value
		operationNode := pathNode.Content[j+1]

		if !isHTTPMethod(operation) {
			continue
		}

		if !matchesHTTPMethod(operation, rule.Condition.HTTPMethods) {
			continue
		}

		if processParametersInOperation(operationNode, operation, pathName, ruleName, rule, filePath, result) {
			changed = true
		}
	}

	return changed
}

// processParametersInOperation processes parameters in a single operation
func processParametersInOperation(operationNode *yaml.Node, operation, pathName, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	changed := false
	parameters := getNodeValue(operationNode, "parameters")
	if parameters == nil || parameters.Kind != yaml.SequenceNode {
		return false
	}

	operationKey := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)

	for _, paramNode := range parameters.Content {
		if paramNode.Kind != yaml.MappingNode {
			continue
		}

		if applyParameterDefault(paramNode, operationKey, ruleName, rule, filePath, result) {
			changed = true
		}
	}

	return changed
}

// applyParameterDefault applies a default value to a parameter if conditions match
func applyParameterDefault(paramNode *yaml.Node, operationKey, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	// Check parameter location (in: query, path, header, cookie)
	paramIn := getStringValue(paramNode, "in")
	if rule.Condition.ParameterIn != "" && paramIn != rule.Condition.ParameterIn {
		addSkippedTarget(result, filePath, fmt.Sprintf("%s parameter", operationKey),
			fmt.Sprintf("parameter in '%s' doesn't match rule condition '%s'", paramIn, rule.Condition.ParameterIn))
		return false
	}

	// Check property name if specified
	paramName := getStringValue(paramNode, "name")
	if rule.Condition.PropertyName != "" && !matchesPropertyName(paramName, rule.Condition.PropertyName) {
		addSkippedTarget(result, filePath, fmt.Sprintf("%s parameter %s", operationKey, paramName),
			fmt.Sprintf("parameter name doesn't match pattern '%s'", rule.Condition.PropertyName))
		return false
	}

	schema := getNodeValue(paramNode, "schema")
	if schema == nil {
		addSkippedTarget(result, filePath, fmt.Sprintf("%s parameter %s", operationKey, paramName), "no schema found")
		return false
	}

	// Check if default already exists
	if getNodeValue(schema, "default") != nil {
		addSkippedTarget(result, filePath, fmt.Sprintf("%s parameter %s", operationKey, paramName), "default already exists")
		return false
	}

	// Check type condition
	schemaType := getStringValue(schema, "type")
	if rule.Condition.Type != "" && schemaType != rule.Condition.Type {
		addSkippedTarget(result, filePath, fmt.Sprintf("%s parameter %s", operationKey, paramName),
			fmt.Sprintf("type '%s' doesn't match rule condition '%s'", schemaType, rule.Condition.Type))
		return false
	}

	// Check enum condition
	if rule.Condition.HasEnum {
		enumNode := getNodeValue(schema, "enum")
		if enumNode == nil {
			addSkippedTarget(result, filePath, fmt.Sprintf("%s parameter %s", operationKey, paramName), "no enum found but required by rule")
			return false
		}
	}

	// Apply the default value
	defaultValue := determineDefaultValue(rule, schema, paramNode)
	if defaultValue != nil {
		return addDefaultToSchema(schema, defaultValue, operationKey, paramName, ruleName, filePath, result)
	}

	return false
}

// processRequestBodyDefaults processes default values for request body schemas
func processRequestBodyDefaults(root *yaml.Node, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	return processOperationDefaults(root, ruleName, rule, filePath, result, processRequestBodyInOperation)
}

// processRequestBodyInOperation processes request body schemas in a single operation
func processRequestBodyInOperation(operationNode *yaml.Node, operation, pathName, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	requestBody := getNodeValue(operationNode, "requestBody")
	if requestBody == nil {
		return false
	}

	content := getNodeValue(requestBody, "content")
	if content == nil || content.Kind != yaml.MappingNode {
		return false
	}

	operationKey := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)
	changed := false

	// Process each content type (application/json, etc.)
	for k := 0; k < len(content.Content); k += 2 {
		contentType := content.Content[k].Value
		contentNode := content.Content[k+1]

		schema := getNodeValue(contentNode, "schema")
		if schema != nil {
			if processSchemaDefaults(schema, nil, operationKey+" requestBody "+contentType, ruleName, rule, filePath, result) {
				changed = true
			}
		}
	}

	return changed
}

// processResponseDefaults processes default values for response schemas
func processResponseDefaults(root *yaml.Node, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	return processOperationDefaults(root, ruleName, rule, filePath, result, processResponsesInOperation)
}

// processResponsesInOperation processes response schemas in a single operation
func processResponsesInOperation(operationNode *yaml.Node, operation, pathName, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	responses := getNodeValue(operationNode, "responses")
	if responses == nil || responses.Kind != yaml.MappingNode {
		return false
	}

	operationKey := fmt.Sprintf("%s %s", strings.ToUpper(operation), pathName)
	changed := false

	for k := 0; k < len(responses.Content); k += 2 {
		statusCode := responses.Content[k].Value
		responseNode := responses.Content[k+1]

		content := getNodeValue(responseNode, "content")
		if content == nil || content.Kind != yaml.MappingNode {
			continue
		}

		// Process each content type
		for l := 0; l < len(content.Content); l += 2 {
			contentType := content.Content[l].Value
			contentNode := content.Content[l+1]

			schema := getNodeValue(contentNode, "schema")
			if schema != nil {
				if processSchemaDefaults(schema, nil, operationKey+" response "+statusCode+" "+contentType, ruleName, rule, filePath, result) {
					changed = true
				}
			}
		}
	}

	return changed
}

// processComponentDefaults processes default values for component schemas
func processComponentDefaults(root *yaml.Node, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	components := getNodeValue(root, "components")
	if components == nil {
		return false
	}

	schemas := getNodeValue(components, "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return false
	}

	changed := false

	for i := 0; i < len(schemas.Content); i += 2 {
		schemaName := schemas.Content[i].Value
		schemaNode := schemas.Content[i+1]

		if processSchemaDefaults(schemaNode, nil, "component "+schemaName, ruleName, rule, filePath, result) {
			changed = true
		}
	}

	return changed
}

// processSchemaDefaults recursively processes schema defaults
func processSchemaDefaults(schema *yaml.Node, root *yaml.Node, context, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	if schema == nil || schema.Kind != yaml.MappingNode {
		return false
	}

	// Handle direct schema properties
	changed := processSchemaProperties(schema, root, context, ruleName, rule, filePath, result)

	// Handle arrays
	if processArrayItems(schema, root, context, ruleName, rule, filePath, result) {
		changed = true
	}

	// Handle compositions (oneOf, anyOf, allOf)
	if processCompositions(schema, root, context, ruleName, rule, filePath, result) {
		changed = true
	}

	return changed
}

// processSchemaProperties processes properties within a schema
func processSchemaProperties(schema *yaml.Node, root *yaml.Node, context, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	properties := getNodeValue(schema, "properties")
	if properties == nil || properties.Kind != yaml.MappingNode {
		return false
	}

	changed := false
	for i := 0; i < len(properties.Content); i += 2 {
		propName := properties.Content[i].Value
		propSchema := properties.Content[i+1]
		propContext := context + " property " + propName

		// Check and apply defaults to this property
		if shouldApplyDefaultToProperty(propSchema, propName, rule, propContext, filePath, result) {
			defaultValue := determineDefaultValue(rule, propSchema, nil)
			if defaultValue != nil {
				if addDefaultToSchema(propSchema, defaultValue, propContext, propName, ruleName, filePath, result) {
					changed = true
				}
			}
		}

		// Recursively process nested schemas
		if processSchemaDefaults(propSchema, root, propContext, ruleName, rule, filePath, result) {
			changed = true
		}
	}

	return changed
}

// processArrayItems processes array item schemas
func processArrayItems(schema *yaml.Node, root *yaml.Node, context, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	items := getNodeValue(schema, "items")
	if items == nil {
		return false
	}

	return processSchemaDefaults(items, root, context+" items", ruleName, rule, filePath, result)
}

// processCompositions processes oneOf, anyOf, allOf compositions
func processCompositions(schema *yaml.Node, root *yaml.Node, context, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult) bool {
	compositions := []string{"oneOf", "anyOf", "allOf"}
	changed := false

	for _, comp := range compositions {
		composition := getNodeValue(schema, comp)
		if composition != nil && composition.Kind == yaml.SequenceNode {
			for idx, item := range composition.Content {
				if processSchemaDefaults(item, root, fmt.Sprintf("%s %s[%d]", context, comp, idx), ruleName, rule, filePath, result) {
					changed = true
				}
			}
		}
	}

	return changed
}

// shouldApplyDefaultToProperty checks if a default should be applied to a property
func shouldApplyDefaultToProperty(propSchema *yaml.Node, propName string, rule config.DefaultRule, context, filePath string, result *DefaultsResult) bool {
	// Check if default already exists
	if getNodeValue(propSchema, "default") != nil {
		addSkippedTarget(result, filePath, context, "default already exists")
		return false
	}

	// Check property name pattern
	if rule.Condition.PropertyName != "" && !matchesPropertyName(propName, rule.Condition.PropertyName) {
		addSkippedTarget(result, filePath, context, fmt.Sprintf("property name doesn't match pattern '%s'", rule.Condition.PropertyName))
		return false
	}

	// Check type condition
	schemaType := getStringValue(propSchema, "type")
	if rule.Condition.Type != "" && schemaType != rule.Condition.Type {
		addSkippedTarget(result, filePath, context, fmt.Sprintf("type '%s' doesn't match rule condition '%s'", schemaType, rule.Condition.Type))
		return false
	}

	// Check enum condition
	if rule.Condition.HasEnum {
		enumNode := getNodeValue(propSchema, "enum")
		if enumNode == nil {
			addSkippedTarget(result, filePath, context, "no enum found but required by rule")
			return false
		}
	}

	// Check array condition
	if rule.Condition.IsArray && schemaType != "array" {
		addSkippedTarget(result, filePath, context, "not an array but required by rule")
		return false
	}

	return true
}

// determineDefaultValue determines the default value to apply based on rule configuration
func determineDefaultValue(rule config.DefaultRule, _ /* schema */, _ /* param */ *yaml.Node) interface{} {
	// If rule has a simple value, use it
	if rule.Value != nil {
		return rule.Value
	}

	// If rule has a template, process it
	if rule.Template != nil {
		// For now, return the template as-is
		// In the future, we could add template processing with context variables
		return rule.Template
	}

	return nil
}

// addDefaultToSchema adds a default value to a schema node
func addDefaultToSchema(schema *yaml.Node, defaultValue interface{}, context, _ /* propertyName */, ruleName, filePath string, result *DefaultsResult) bool {
	// Create default key node
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "default",
	}

	// Create default value node
	valueNode := createDefaultValueNode(defaultValue)
	if valueNode == nil {
		addSkippedTarget(result, filePath, context, "could not create value node for default")
		return false
	}

	// Add to schema
	schema.Content = append(schema.Content, keyNode, valueNode)

	// Record the applied default
	addAppliedDefault(result, filePath, fmt.Sprintf("%s: default = %v (rule: %s)", context, defaultValue, ruleName))

	return true
}

// createDefaultValueNode creates a YAML node from a default value
func createDefaultValueNode(value interface{}) *yaml.Node {
	switch v := value.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(v)}
	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatInt(v, 10)}
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatFloat(v, 'f', -1, 64)}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatBool(v)}
	case []interface{}:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range v {
			itemNode := createDefaultValueNode(item)
			if itemNode != nil {
				node.Content = append(node.Content, itemNode)
			}
		}
		return node
	case map[string]interface{}:
		node := &yaml.Node{Kind: yaml.MappingNode}
		for key, val := range v {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
			valNode := createDefaultValueNode(val)
			if valNode != nil {
				node.Content = append(node.Content, keyNode, valNode)
			}
		}
		return node
	default:
		return nil
	}
}

// processOperationDefaults is a helper that iterates through paths and operations
// and calls the provided processor function for each matching operation
func processOperationDefaults(root *yaml.Node, ruleName string, rule config.DefaultRule, filePath string, result *DefaultsResult,
	processor func(*yaml.Node, string, string, string, config.DefaultRule, string, *DefaultsResult) bool) bool {
	changed := false
	paths := getNodeValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i < len(paths.Content); i += 2 {
		pathName := paths.Content[i].Value
		pathNode := paths.Content[i+1]

		if !matchesPathPattern(pathName, rule.Condition.PathPatterns) {
			continue
		}

		for j := 0; j < len(pathNode.Content); j += 2 {
			operation := pathNode.Content[j].Value
			operationNode := pathNode.Content[j+1]

			if !isHTTPMethod(operation) {
				continue
			}

			if !matchesHTTPMethod(operation, rule.Condition.HTTPMethods) {
				continue
			}

			if processor(operationNode, operation, pathName, ruleName, rule, filePath, result) {
				changed = true
			}
		}
	}

	return changed
}

// Helper functions

func matchesPathPattern(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // No pattern means match all
	}
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return true
		}
	}
	return false
}

func matchesHTTPMethod(method string, methods []string) bool {
	if len(methods) == 0 {
		return true // No method filter means match all
	}
	for _, m := range methods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

func matchesPropertyName(name, pattern string) bool {
	if pattern == "" {
		return true
	}
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

func addAppliedDefault(result *DefaultsResult, filePath, defaultInfo string) {
	if result.AppliedDefaults[filePath] == nil {
		result.AppliedDefaults[filePath] = []string{}
	}
	result.AppliedDefaults[filePath] = append(result.AppliedDefaults[filePath], defaultInfo)
}

func addSkippedTarget(result *DefaultsResult, filePath, target, reason string) {
	if result.SkippedTargets[filePath] == nil {
		result.SkippedTargets[filePath] = []string{}
	}
	result.SkippedTargets[filePath] = append(result.SkippedTargets[filePath], fmt.Sprintf("%s: %s", target, reason))
}

func writeDefaultsDocument(doc *yaml.Node, path string, dryRun bool) (bool, error) {
	if dryRun {
		return true, nil // Return true to indicate changes were detected, but don't write
	}

	return writeModifiedDocument(doc, path)
}

// getStringValue is a helper to get string value from a YAML node
func getStringValue(node *yaml.Node, key string) string {
	valueNode := getNodeValue(node, key)
	if valueNode != nil {
		return valueNode.Value
	}
	return ""
}
