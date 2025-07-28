package transform

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError represents validation errors in OpenAPI structures
type ValidationError struct {
	Message  string
	Location string
	Field    string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error at %s.%s: %s", e.Location, e.Field, e.Message)
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ValidateCompositionStructures validates oneOf/anyOf/allOf structures in OpenAPI documents
func ValidateCompositionStructures(root *yaml.Node, filePath string) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	validateNode(root, "", filePath, result)

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// validateNode recursively validates a YAML node for composition issues
func validateNode(node *yaml.Node, path, filePath string, result *ValidationResult) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		validateMappingNode(node, path, filePath, result)
	case yaml.SequenceNode:
		validateSequenceNode(node, path, filePath, result)
	}
}

// validateMappingNode validates mapping nodes for composition issues
func validateMappingNode(node *yaml.Node, path, filePath string, result *ValidationResult) {
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			result.Errors = append(result.Errors, ValidationError{
				Message:  "malformed mapping node: missing value for key",
				Location: path,
				Field:    "structure",
			})
			continue
		}

		key := node.Content[i].Value
		value := node.Content[i+1]
		currentPath := buildPath(path, key)

		// Validate composition structures
		if isCompositionKey(key) {
			validateComposition(key, value, currentPath, result)
		}

		// Recursively validate nested structures
		validateNode(value, currentPath, filePath, result)
	}
}

// validateSequenceNode validates sequence nodes
func validateSequenceNode(node *yaml.Node, path, filePath string, result *ValidationResult) {
	for i, item := range node.Content {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		validateNode(item, itemPath, filePath, result)
	}
}

// validateComposition validates oneOf/anyOf/allOf structures
func validateComposition(compositionType string, compositionNode *yaml.Node, path string, result *ValidationResult) {
	if compositionNode.Kind != yaml.SequenceNode {
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("%s must be an array", compositionType),
			Location: path,
			Field:    compositionType,
		})
		return
	}

	// Check for empty compositions
	if len(compositionNode.Content) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("%s array is empty", compositionType),
			Location: path,
			Field:    compositionType,
		})
		return
	}

	// Validate each item in the composition
	for i, item := range compositionNode.Content {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		validateCompositionItem(item, itemPath, compositionType, result)
	}

	// Check for redundant compositions (single item that could be flattened)
	if len(compositionNode.Content) == 1 {
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("%s with single item could be flattened", compositionType),
			Location: path,
			Field:    compositionType,
		})
	}
}

// validateCompositionItem validates individual items within compositions
func validateCompositionItem(item *yaml.Node, path, compositionType string, result *ValidationResult) {
	if item.Kind != yaml.MappingNode {
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("%s item must be an object", compositionType),
			Location: path,
			Field:    "type",
		})
		return
	}

	// Check for malformed $ref
	hasRef := false
	hasOtherProperties := false

	for i := 0; i < len(item.Content); i += 2 {
		if i+1 >= len(item.Content) {
			continue
		}

		key := item.Content[i].Value
		value := item.Content[i+1]

		if key == "$ref" {
			hasRef = true
			if value.Kind != yaml.ScalarNode || value.Value == "" {
				result.Errors = append(result.Errors, ValidationError{
					Message:  "$ref must have a non-empty string value",
					Location: path,
					Field:    "$ref",
				})
			}
		} else {
			hasOtherProperties = true
		}
	}

	// Warn about mixed $ref and other properties (allowed but potentially confusing)
	if hasRef && hasOtherProperties {
		result.Errors = append(result.Errors, ValidationError{
			Message:  "$ref with additional properties may cause unexpected behavior",
			Location: path,
			Field:    "$ref",
		})
	}
}

// isCompositionKey checks if a key represents a composition structure
func isCompositionKey(key string) bool {
	return key == "oneOf" || key == "anyOf" || key == "allOf"
}

// buildPath builds a path string for error reporting
func buildPath(parentPath, key string) string {
	if parentPath == "" {
		return key
	}
	return parentPath + "." + key
}

// ReportValidationErrors formats validation errors for display with improved visual formatting
func ReportValidationErrors(validationResult *ValidationResult, filePath string) string {
	if validationResult.Valid {
		return ""
	}

	var sb strings.Builder

	// Header with visual styling matching OpenMorph design
	sb.WriteString(fmt.Sprintf("\n\033[1;33m📋 Validation Issues Found in %s\033[0m\n", filePath))
	sb.WriteString("\033[36m┌────────────────────────────────────────────────────────────────┐\033[0m\n")
	sb.WriteString(fmt.Sprintf("\033[36m│\033[0m \033[1;31m⚠️  Found %d validation issues that will be auto-fixed\033[0m \033[36m       │\033[0m\n", len(validationResult.Errors)))
	sb.WriteString("\033[36m└────────────────────────────────────────────────────────────────┘\033[0m\n")

	// Categorize errors for better organization
	flatteningIssues := []ValidationError{}
	emptyIssues := []ValidationError{}
	malformedIssues := []ValidationError{}
	otherIssues := []ValidationError{}

	for _, err := range validationResult.Errors {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "single item could be flattened"):
			flatteningIssues = append(flatteningIssues, err)
		case strings.Contains(errMsg, "array is empty"):
			emptyIssues = append(emptyIssues, err)
		case strings.Contains(errMsg, "non-empty string value"):
			malformedIssues = append(malformedIssues, err)
		default:
			otherIssues = append(otherIssues, err)
		}
	}

	// Print categorized issues with icons and better formatting
	if len(flatteningIssues) > 0 {
		sb.WriteString(fmt.Sprintf("\n\033[1;34m🔄 Flattening Opportunities (%d)\033[0m\n", len(flatteningIssues)))
		for _, err := range flatteningIssues {
			location := extractLocation(err.Error())
			sb.WriteString(fmt.Sprintf("   \033[34m▸\033[0m \033[33m%s\033[0m\n", location))
		}
	}

	if len(emptyIssues) > 0 {
		sb.WriteString(fmt.Sprintf("\n\033[1;31m🗑️  Empty Compositions (%d)\033[0m\n", len(emptyIssues)))
		for _, err := range emptyIssues {
			location := extractLocation(err.Error())
			sb.WriteString(fmt.Sprintf("   \033[31m▸\033[0m \033[33m%s\033[0m\n", location))
		}
	}

	if len(malformedIssues) > 0 {
		sb.WriteString(fmt.Sprintf("\n\033[1;35m🔗 Malformed References (%d)\033[0m\n", len(malformedIssues)))
		for _, err := range malformedIssues {
			location := extractLocation(err.Error())
			sb.WriteString(fmt.Sprintf("   \033[35m▸\033[0m \033[33m%s\033[0m\n", location))
		}
	}

	if len(otherIssues) > 0 {
		sb.WriteString(fmt.Sprintf("\n\033[1;37m⚠️  Other Issues (%d)\033[0m\n", len(otherIssues)))
		for _, err := range otherIssues {
			location := extractLocation(err.Error())
			sb.WriteString(fmt.Sprintf("   \033[37m▸\033[0m \033[33m%s\033[0m\n", location))
		}
	}

	// Footer with helpful information
	sb.WriteString("\n\033[32m💡 These issues will be automatically resolved during transformation.\033[0m\n")

	return sb.String()
}

// extractLocation extracts the location part from validation error messages
func extractLocation(errorMessage string) string {
	// Extract location from error messages like "validation error at paths./clients.get..."
	if strings.Contains(errorMessage, "validation error at ") {
		parts := strings.Split(errorMessage, "validation error at ")
		if len(parts) > 1 {
			locationAndMsg := parts[1]
			if colonIndex := strings.Index(locationAndMsg, ": "); colonIndex != -1 {
				return locationAndMsg[:colonIndex]
			}
			return locationAndMsg
		}
	}
	return errorMessage
}

// ValidateAndReportCompositions validates compositions and returns formatted error report
func ValidateAndReportCompositions(root *yaml.Node, filePath string) string {
	validationResult := ValidateCompositionStructures(root, filePath)
	return ReportValidationErrors(validationResult, filePath)
}
