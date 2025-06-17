package pagination

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Strategy defines a pagination strategy with its parameters and response fields
type Strategy struct {
	Params []string
	Fields []string
}

// PaginationStrategies defines all supported pagination strategies
var PaginationStrategies = map[string]Strategy{
	"checkpoint": {
		Params: []string{"from", "take", "after"},
		Fields: []string{"next", "next_checkpoint"},
	},
	"offset": {
		Params: []string{"offset", "limit", "include_totals"},
		Fields: []string{"total", "offset", "limit", "count"},
	},
	"page": {
		Params: []string{"page", "per_page", "include_totals"},
		Fields: []string{"start", "limit", "total", "total_count", "page", "per_page"},
	},
	"cursor": {
		Params: []string{"cursor", "size"},
		Fields: []string{"next_cursor", "has_more"},
	},
	"none": {
		Params: []string{},
		Fields: []string{},
	},
}

// Options represents pagination transformation options
type Options struct {
	Priority []string // ordered list of pagination strategies by priority
}

// DetectedPagination represents detected pagination in an endpoint
type DetectedPagination struct {
	Strategy   string
	Parameters []string // parameter names found
	Fields     []string // response field names found
}

// ProcessResult contains the result of processing a single endpoint
type ProcessResult struct {
	Changed          bool
	RemovedParams    []string
	RemovedResponses []string
	ModifiedSchemas  []string
}

// DetectPaginationInParams detects pagination strategies in operation parameters
func DetectPaginationInParams(params *yaml.Node) []DetectedPagination {
	var detected []DetectedPagination

	if params == nil || params.Kind != yaml.SequenceNode {
		return detected
	}

	strategyParams := make(map[string][]string)

	// Scan through parameters
	for _, param := range params.Content {
		if param.Kind != yaml.MappingNode {
			continue
		}

		paramName := getStringValue(param, "name")
		if paramName == "" {
			continue
		}

		// Check which strategies this parameter belongs to
		for strategyName, strategy := range PaginationStrategies {
			for _, strategyParam := range strategy.Params {
				if matchesParam(paramName, strategyParam) {
					strategyParams[strategyName] = append(strategyParams[strategyName], paramName)
				}
			}
		}
	}

	// Convert to DetectedPagination, filtering out weak strategies
	// A strategy is considered "weak" if it only has shared parameters
	sharedParams := findSharedParams()

	for strategy, params := range strategyParams {
		// Check if this strategy has any non-shared parameters
		hasNonSharedParams := false
		for _, param := range params {
			if !sharedParams[param] {
				hasNonSharedParams = true
				break
			}
		}

		// Only include strategies that have non-shared parameters
		// This prevents strategies like "offset" with only "include_totals" from interfering
		if hasNonSharedParams {
			detected = append(detected, DetectedPagination{
				Strategy:   strategy,
				Parameters: params,
			})
		}
	}

	return detected
}

// findSharedParams identifies parameters that belong to multiple strategies
func findSharedParams() map[string]bool {
	sharedParams := make(map[string]bool)
	paramCount := make(map[string]int)

	// Count how many strategies each parameter appears in
	for _, strategy := range PaginationStrategies {
		for _, param := range strategy.Params {
			paramCount[param]++
		}
	}

	// Mark parameters that appear in multiple strategies as shared
	for param, count := range paramCount {
		if count > 1 {
			sharedParams[param] = true
		}
	}

	return sharedParams
}

// findSharedFields identifies fields that belong to multiple strategies
func findSharedFields() map[string]bool {
	sharedFields := make(map[string]bool)
	fieldCount := make(map[string]int)

	// Count how many strategies each field appears in
	for _, strategy := range PaginationStrategies {
		for _, field := range strategy.Fields {
			fieldCount[field]++
		}
	}

	// Mark fields that appear in multiple strategies as shared
	for field, count := range fieldCount {
		if count > 1 {
			sharedFields[field] = true
		}
	}

	return sharedFields
}

// DetectPaginationInResponses detects pagination strategies in operation responses
func DetectPaginationInResponses(responses *yaml.Node) []DetectedPagination {
	return DetectPaginationInResponsesWithDoc(responses, nil)
}

// DetectPaginationInResponsesWithDoc detects pagination strategies with document context for $ref resolution
func DetectPaginationInResponsesWithDoc(responses *yaml.Node, doc *yaml.Node) []DetectedPagination {
	var detected []DetectedPagination

	if responses == nil || responses.Kind != yaml.MappingNode {
		return detected
	}

	strategyFields := make(map[string][]string)

	// Walk through all responses
	for i := 0; i < len(responses.Content); i += 2 {
		responseCode := responses.Content[i].Value
		responseNode := responses.Content[i+1]

		// Skip non-success responses unless they contain pagination-like content
		// We process 2xx, 3xx, and default responses, plus 4xx that might contain pagination info
		if !isSuccessResponse(responseCode) && !isPaginationRelevantResponse(responseCode) {
			continue
		}

		var fields []string
		if doc != nil {
			fields = extractFieldsFromResponseWithDoc(responseNode, doc)
		} else {
			fields = extractFieldsFromResponse(responseNode)
		}

		// Check which strategies these fields belong to
		for strategyName, strategy := range PaginationStrategies {
			var matchedFields []string
			for _, field := range fields {
				for _, strategyField := range strategy.Fields {
					if matchesField(field, strategyField) {
						matchedFields = append(matchedFields, field)
					}
				}
			}
			if len(matchedFields) > 0 {
				strategyFields[strategyName] = append(strategyFields[strategyName], matchedFields...)
			}
		}
	}

	// Convert to DetectedPagination
	for strategy, fields := range strategyFields {
		detected = append(detected, DetectedPagination{
			Strategy: strategy,
			Fields:   fields,
		})
	}

	return detected
}

// ProcessEndpoint processes a single endpoint based on pagination priority
func ProcessEndpoint(operation *yaml.Node, opts Options) (*ProcessResult, error) {
	return ProcessEndpointWithDoc(operation, nil, opts)
}

// ProcessEndpointWithDoc processes a single endpoint with document context for $ref resolution
func ProcessEndpointWithDoc(operation *yaml.Node, doc *yaml.Node, opts Options) (*ProcessResult, error) {
	result := &ProcessResult{}

	if operation == nil || operation.Kind != yaml.MappingNode {
		return result, nil
	}

	// Get parameters and responses
	params := getNodeValue(operation, "parameters")
	responses := getNodeValue(operation, "responses")

	// Detect pagination in both params and responses
	paramPagination := DetectPaginationInParams(params)
	responsePagination := DetectPaginationInResponsesWithDoc(responses, doc)

	// Determine which strategies are present in PARAMETERS (primary for selection)
	paramStrategies := make(map[string]bool)
	for _, p := range paramPagination {
		paramStrategies[p.Strategy] = true
	}

	// Determine which strategies are present in RESPONSES (for cleanup only)
	responseStrategies := make(map[string]bool)
	for _, r := range responsePagination {
		responseStrategies[r.Strategy] = true
	}

	// If no pagination strategies detected in parameters, don't modify the endpoint
	// (we only process endpoints that have pagination parameters)
	if len(paramStrategies) == 0 {
		return result, nil
	}

	// Check if processing is needed:
	// 1. Multiple parameter strategies detected, OR
	// 2. Response strategies differ from parameter strategies (cleanup needed), OR
	// 3. There are orphaned shared parameters that don't belong to detected strategies, OR
	// 4. Response schemas contain mixed pagination types (plain array + paginated responses)
	needsProcessing := len(paramStrategies) > 1

	if !needsProcessing {
		// Single parameter strategy - check if response cleanup is needed
		for responseStrategy := range responseStrategies {
			if !paramStrategies[responseStrategy] {
				// Response has a strategy not in parameters - cleanup needed
				needsProcessing = true
				break
			}
		}
	}

	if !needsProcessing {
		// Single parameter strategy - check for orphaned shared parameters
		if params != nil {
			sharedParams := findSharedParams()
			for _, param := range params.Content {
				if param.Kind != yaml.MappingNode {
					continue
				}
				paramName := getStringValue(param, "name")
				if paramName != "" && sharedParams[paramName] {
					// This is a shared parameter - check if it belongs to any detected strategy
					belongsToDetected := false
					for strategy := range paramStrategies {
						for _, strategyParam := range PaginationStrategies[strategy].Params {
							if matchesParam(paramName, strategyParam) {
								belongsToDetected = true
								break
							}
						}
						if belongsToDetected {
							break
						}
					}
					if !belongsToDetected {
						// Orphaned shared parameter - needs to be removed
						needsProcessing = true
						break
					}
				}
			}
		}
	}

	if !needsProcessing {
		// Single parameter strategy - check for mixed response compositions
		// (plain array + paginated responses in oneOf/anyOf/allOf)
		if responses != nil && hasMixedResponseCompositions(responses, doc) {
			needsProcessing = true
		}
	}

	if !needsProcessing {
		// Single strategy, no cleanup needed
		return result, nil
	}

	// Select strategy based on strict priority order
	// Prefer strategies with parameters, but allow response-only strategies if no param strategies match
	var selectedStrategy string
	allStrategies := make(map[string]bool)
	for strategy := range paramStrategies {
		allStrategies[strategy] = true
	}
	for strategy := range responseStrategies {
		allStrategies[strategy] = true
	}

	// First pass: look for strategies that have parameters
	for _, priority := range opts.Priority {
		if priority == "none" && len(allStrategies) > 0 {
			// Special case: "none" means remove all pagination if it's highest priority
			selectedStrategy = "none"
			break
		}
		if paramStrategies[priority] {
			// Found a strategy that has parameters - prefer this
			selectedStrategy = priority
			break
		}
	}

	// Second pass: if no parameter strategy found, look for response-only strategies
	if selectedStrategy == "" {
		for _, priority := range opts.Priority {
			if responseStrategies[priority] && !paramStrategies[priority] {
				// Found a response-only strategy
				selectedStrategy = priority
				break
			}
		}
	}

	if selectedStrategy == "" {
		// No strategy in priority list found
		return result, nil
	}

	// Remove lower priority strategies
	if params != nil {
		removed := removeUnwantedParams(params, selectedStrategy, paramPagination)
		result.RemovedParams = removed
		if len(removed) > 0 {
			result.Changed = true
		}
	}

	if responses != nil {
		removed, modified := removeUnwantedResponsesWithDoc(responses, selectedStrategy, responsePagination, doc)
		result.RemovedResponses = removed
		result.ModifiedSchemas = modified
		if len(removed) > 0 || len(modified) > 0 {
			result.Changed = true
		}
	}

	return result, nil
}

// removeUnwantedParams removes parameters that don't match the selected strategy
func removeUnwantedParams(params *yaml.Node, selectedStrategy string, detected []DetectedPagination) []string {
	var removed []string

	if params.Kind != yaml.SequenceNode {
		return removed
	}

	// Create a new content slice without unwanted params
	var newContent []*yaml.Node

	for _, param := range params.Content {
		if param.Kind != yaml.MappingNode {
			newContent = append(newContent, param)
			continue
		}

		paramName := getStringValue(param, "name")
		if paramName == "" {
			newContent = append(newContent, param)
			continue
		}

		// Special handling for "none" strategy - remove all pagination parameters
		if selectedStrategy == "none" {
			isPaginationParam := false
			for _, d := range detected {
				for _, p := range d.Parameters {
					if p == paramName {
						isPaginationParam = true
						break
					}
				}
				if isPaginationParam {
					break
				}
			}

			if isPaginationParam {
				removed = append(removed, paramName)
			} else {
				newContent = append(newContent, param)
			}
			continue
		}

		// Check if this param belongs to the selected strategy
		belongsToSelected := false
		selectedParams := PaginationStrategies[selectedStrategy].Params
		for _, selectedParam := range selectedParams {
			if matchesParam(paramName, selectedParam) {
				belongsToSelected = true
				break
			}
		}

		// If the parameter belongs to the selected strategy, keep it
		if belongsToSelected {
			newContent = append(newContent, param)
		} else {
			// Check if this param belongs to any unwanted strategy (detected or not)
			belongsToUnwanted := false

			// First check detected strategies
			for _, d := range detected {
				if d.Strategy != selectedStrategy {
					for _, p := range d.Parameters {
						if p == paramName {
							belongsToUnwanted = true
							break
						}
					}
					if belongsToUnwanted {
						break
					}
				}
			}

			// Also check if this param belongs to any pagination strategy that wasn't detected
			// This handles shared parameters like include_totals
			if !belongsToUnwanted {
				for strategyName, strategy := range PaginationStrategies {
					if strategyName != selectedStrategy {
						for _, strategyParam := range strategy.Params {
							if matchesParam(paramName, strategyParam) {
								belongsToUnwanted = true
								break
							}
						}
						if belongsToUnwanted {
							break
						}
					}
				}
			}

			if belongsToUnwanted {
				removed = append(removed, paramName)
			} else {
				newContent = append(newContent, param)
			}
		}
	}

	params.Content = newContent
	return removed
}

// removeUnwantedResponses removes or modifies responses that contain unwanted pagination
func removeUnwantedResponses(responses *yaml.Node, selectedStrategy string, detected []DetectedPagination) ([]string, []string) {
	return removeUnwantedResponsesWithDoc(responses, selectedStrategy, detected, nil)
}

// removeUnwantedResponsesWithDoc removes or modifies responses with document context for $ref resolution
func removeUnwantedResponsesWithDoc(responses *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) ([]string, []string) {
	var removedResponses []string
	var modifiedSchemas []string

	if responses.Kind != yaml.MappingNode {
		return removedResponses, modifiedSchemas
	}

	// Create a new content slice
	var newContent []*yaml.Node

	for i := 0; i < len(responses.Content); i += 2 {
		responseCode := responses.Content[i]
		responseNode := responses.Content[i+1]

		// Check if this response contains unwanted pagination
		var fields []string
		if doc != nil {
			fields = extractFieldsFromResponseWithDoc(responseNode, doc)
		} else {
			fields = extractFieldsFromResponse(responseNode)
		}

		// Special handling for "none" strategy - remove all pagination fields
		if selectedStrategy == "none" {
			containsPaginationFields := false
			for _, field := range fields {
				// Check if field belongs to any detected strategy
				for _, d := range detected {
					for _, strategyField := range d.Fields {
						if matchesField(field, strategyField) {
							containsPaginationFields = true
							break
						}
					}
					if containsPaginationFields {
						break
					}
				}
				if containsPaginationFields {
					break
				}
			}

			if containsPaginationFields {
				// Clean all pagination fields from the schema
				modified := cleanResponseSchemaWithDoc(responseNode, selectedStrategy, detected, doc)
				if len(modified) > 0 {
					modifiedSchemas = append(modifiedSchemas, modified...)
				}
			}

			// Always keep the response for "none" strategy (just clean it)
			newContent = append(newContent, responseCode, responseNode)
			continue
		}

		containsUnwanted := false

		for _, field := range fields {
			// Check if field belongs to unwanted strategies (detected or not)
			// First check detected strategies
			for _, d := range detected {
				if d.Strategy != selectedStrategy {
					for _, unwantedField := range d.Fields {
						if matchesField(field, unwantedField) {
							containsUnwanted = true
							break
						}
					}
					if containsUnwanted {
						break
					}
				}
			}

			// Also check if this field belongs to any pagination strategy that wasn't detected
			// This handles shared fields like "total" that belong to multiple strategies
			if !containsUnwanted {
				for strategyName, strategy := range PaginationStrategies {
					if strategyName != selectedStrategy {
						for _, strategyField := range strategy.Fields {
							if matchesField(field, strategyField) {
								containsUnwanted = true
								break
							}
						}
						if containsUnwanted {
							break
						}
					}
				}
			}

			if containsUnwanted {
				break
			}
		}

		// Decision logic:
		// - If response contains unwanted pagination fields, clean the schema
		// - Never remove entire responses, always try to clean them first
		if containsUnwanted {
			// Clean the schema to remove unwanted pagination fields
			modified := cleanResponseSchemaWithDoc(responseNode, selectedStrategy, detected, doc)
			if len(modified) > 0 {
				modifiedSchemas = append(modifiedSchemas, modified...)
			}
		} else {
			// Even if no unwanted fields detected at response level, check for mixed compositions
			if hasMixedCompositionInResponse(responseNode, doc) {
				modified := cleanResponseSchemaWithDoc(responseNode, selectedStrategy, detected, doc)
				if len(modified) > 0 {
					modifiedSchemas = append(modifiedSchemas, modified...)
				}
			}
		}

		// Keep the response
		newContent = append(newContent, responseCode, responseNode)
	}

	responses.Content = newContent
	return removedResponses, modifiedSchemas
}

// cleanResponseSchema removes unwanted pagination fields from response schemas
func cleanResponseSchema(response *yaml.Node, selectedStrategy string, detected []DetectedPagination) []string {
	return cleanResponseSchemaWithDoc(response, selectedStrategy, detected, nil)
}

// cleanResponseSchemaWithDoc removes unwanted pagination fields with document context
func cleanResponseSchemaWithDoc(response *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) []string {
	var modified []string

	// Navigate to schema content
	content := getNodeValue(response, "content")
	if content == nil {
		return modified
	}

	// Handle different content types
	if content.Kind == yaml.MappingNode {
		for i := 0; i < len(content.Content); i += 2 {
			mediaType := content.Content[i].Value
			mediaTypeNode := content.Content[i+1]

			schema := getNodeValue(mediaTypeNode, "schema")
			if schema != nil {
				schemaModified := cleanSchemaNodeWithDoc(schema, selectedStrategy, detected, doc)
				if len(schemaModified) > 0 {
					modified = append(modified, fmt.Sprintf("%s schema", mediaType))
				}
			}
		}
	}

	return modified
}

// cleanSchemaNode recursively cleans a schema node
func cleanSchemaNode(schema *yaml.Node, selectedStrategy string, detected []DetectedPagination) []string {
	return cleanSchemaNodeWithDoc(schema, selectedStrategy, detected, nil)
}

// cleanSchemaNodeWithDoc recursively cleans a schema node with document context
func cleanSchemaNodeWithDoc(schema *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) []string {
	var modified []string

	if schema.Kind != yaml.MappingNode {
		return modified
	}

	// Handle $ref by resolving it first
	if ref := getNodeValue(schema, "$ref"); ref != nil && doc != nil {
		refPath := ref.Value
		resolvedSchema := resolveRef(refPath, doc)
		if resolvedSchema != nil {
			// Process the resolved schema
			return cleanSchemaNodeWithDoc(resolvedSchema, selectedStrategy, detected, doc)
		}
		// If we can't resolve the ref, fall through to process the current schema
	}

	// Handle oneOf, anyOf, allOf
	if oneOf := getNodeValue(schema, "oneOf"); oneOf != nil {
		if cleanCompositionNodeWithDoc(oneOf, selectedStrategy, detected, doc) {
			modified = append(modified, "oneOf")
		}
	}

	if anyOf := getNodeValue(schema, "anyOf"); anyOf != nil {
		if cleanCompositionNodeWithDoc(anyOf, selectedStrategy, detected, doc) {
			modified = append(modified, "anyOf")
		}
	}

	if allOf := getNodeValue(schema, "allOf"); allOf != nil {
		if cleanCompositionNodeWithDoc(allOf, selectedStrategy, detected, doc) {
			modified = append(modified, "allOf")
		}
	}

	// Handle properties
	if properties := getNodeValue(schema, "properties"); properties != nil {
		if cleanPropertiesNode(properties, selectedStrategy, detected) {
			modified = append(modified, "properties")
		}
	}

	return modified
}

// cleanCompositionNode cleans oneOf/anyOf/allOf nodes
func cleanCompositionNode(composition *yaml.Node, selectedStrategy string, detected []DetectedPagination) bool {
	return cleanCompositionNodeWithDoc(composition, selectedStrategy, detected, nil)
}

// cleanCompositionNodeWithDoc cleans oneOf/anyOf/allOf nodes with document context
func cleanCompositionNodeWithDoc(composition *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) bool {
	if composition.Kind != yaml.SequenceNode {
		return false
	}

	var newContent []*yaml.Node
	modified := false

	for _, item := range composition.Content {
		if shouldKeepSchemaItemWithDoc(item, selectedStrategy, detected, doc) {
			newContent = append(newContent, item)
		} else {
			modified = true
		}
	}

	composition.Content = newContent

	// If only one item remains in oneOf/anyOf, we could potentially flatten it
	// but for now, we'll keep the structure to preserve OpenAPI validity

	return modified
}

// cleanPropertiesNode removes unwanted pagination properties
func cleanPropertiesNode(properties *yaml.Node, selectedStrategy string, detected []DetectedPagination) bool {
	if properties.Kind != yaml.MappingNode {
		return false
	}

	var newContent []*yaml.Node
	modified := false

	for i := 0; i < len(properties.Content); i += 2 {
		propName := properties.Content[i].Value
		propNode := properties.Content[i+1]

		shouldRemove := false

		// Special handling for "none" strategy - remove all pagination fields
		if selectedStrategy == "none" {
			// Check if this property is any pagination field (from detected or all strategies)
			for _, d := range detected {
				for _, field := range d.Fields {
					if matchesField(propName, field) {
						shouldRemove = true
						break
					}
				}
				if shouldRemove {
					break
				}
			}

			// Also check against all strategy definitions for "none" strategy
			if !shouldRemove {
				for _, strategy := range PaginationStrategies {
					for _, strategyField := range strategy.Fields {
						if matchesField(propName, strategyField) {
							shouldRemove = true
							break
						}
					}
					if shouldRemove {
						break
					}
				}
			}
		}
		// For other strategies, don't remove individual fields - let shouldKeepSchemaItem handle whole objects

		if !shouldRemove {
			newContent = append(newContent, properties.Content[i], propNode)
		} else {
			modified = true
		}
	}

	properties.Content = newContent
	return modified
}

// shouldKeepSchemaItem determines if a schema item should be kept
func shouldKeepSchemaItem(item *yaml.Node, selectedStrategy string, detected []DetectedPagination) bool {
	return shouldKeepSchemaItemWithDoc(item, selectedStrategy, detected, nil)
}

// shouldKeepSchemaItemWithDoc determines if a schema item should be kept with document context
func shouldKeepSchemaItemWithDoc(item *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) bool {
	if item.Kind != yaml.MappingNode {
		return true // Keep non-object items
	}

	// Extract fields from this schema item
	var fields []string
	if doc != nil {
		fields = extractFieldsFromSchemaWithDoc(item, doc)
	} else {
		fields = extractFieldsFromSchema(item)
	}

	// Special handling for "none" strategy
	if selectedStrategy == "none" {
		// For "none" strategy, only keep items that have NO pagination fields
		containsAnyPagination := false
		for _, field := range fields {
			// Check if this field belongs to ANY pagination strategy
			for _, strategy := range PaginationStrategies {
				for _, strategyField := range strategy.Fields {
					if matchesField(field, strategyField) {
						containsAnyPagination = true
						break
					}
				}
				if containsAnyPagination {
					break
				}
			}
			if containsAnyPagination {
				break
			}
		}
		// For "none" strategy, keep only schemas that have NO pagination fields
		return !containsAnyPagination
	}

	// For other strategies, use strict strategy-specific logic
	// Rule: Keep schemas that contain the selected strategy's unique fields

	containsSelectedUniqueFields := false
	containsOtherStrategyUniqueFields := false

	selectedFields := PaginationStrategies[selectedStrategy].Fields

	// Check if this schema contains unique fields from the selected strategy
	for _, field := range fields {
		for _, selectedField := range selectedFields {
			if matchesField(field, selectedField) {
				// Check if this field is unique to the selected strategy or shared
				isShared := false
				for strategyName, strategy := range PaginationStrategies {
					if strategyName != selectedStrategy {
						for _, otherField := range strategy.Fields {
							if matchesField(selectedField, otherField) {
								isShared = true
								break
							}
						}
						if isShared {
							break
						}
					}
				}
				if !isShared {
					containsSelectedUniqueFields = true
					break
				}
			}
		}
		if containsSelectedUniqueFields {
			break
		}
	}

	// Check if this schema contains unique fields from other strategies
	for _, field := range fields {
		for strategyName, strategy := range PaginationStrategies {
			if strategyName != selectedStrategy {
				for _, strategyField := range strategy.Fields {
					if matchesField(field, strategyField) {
						// Check if this field is unique to this other strategy
						isShared := false
						for otherStrategyName, otherStrategy := range PaginationStrategies {
							if otherStrategyName != strategyName {
								for _, otherField := range otherStrategy.Fields {
									if matchesField(strategyField, otherField) {
										isShared = true
										break
									}
								}
								if isShared {
									break
								}
							}
						}
						if !isShared {
							containsOtherStrategyUniqueFields = true
							break
						}
					}
				}
				if containsOtherStrategyUniqueFields {
					break
				}
			}
		}
		if containsOtherStrategyUniqueFields {
			break
		}
	}

	// Decision logic for paginated endpoints:
	// 1. If schema contains unique fields from selected strategy -> KEEP
	// 2. If schema contains unique fields from other strategies -> REMOVE
	// 3. If schema contains no pagination fields:
	//    - For checkpoint/cursor strategies -> KEEP (allows non-paginated responses)
	//    - For offset/page strategies -> REMOVE (requires consistent pagination)
	// 4. If schema contains only shared fields -> REMOVE (not strategy-specific enough)

	if containsSelectedUniqueFields {
		return true // Contains unique fields from selected strategy
	}

	if containsOtherStrategyUniqueFields {
		return false // Contains unique fields from other strategies
	}

	// Handle schemas with no pagination fields
	// For paginated endpoints, we want only responses that match the selected strategy
	// Plain arrays (non-paginated responses) should be removed when pagination parameters are present
	// This ensures consistent pagination behavior across the API

	// For other pagination strategies, remove schemas that don't have strategy-specific fields
	// This ensures only one pagination response remains per endpoint
	return false
}

// hasMixedResponseComposition checks if responses contain mixed pagination types in oneOf/anyOf/allOf
func hasMixedResponseCompositions(responses *yaml.Node, doc *yaml.Node) bool {
	if responses == nil || responses.Kind != yaml.MappingNode {
		return false
	}

	// Walk through all responses
	for i := 0; i < len(responses.Content); i += 2 {
		responseCode := responses.Content[i].Value
		responseNode := responses.Content[i+1]

		// Only check success responses
		if !isSuccessResponse(responseCode) {
			continue
		}

		if hasMixedCompositionInResponse(responseNode, doc) {
			return true
		}
	}

	return false
}

// hasMixedCompositionInResponse checks if a single response contains mixed pagination types
func hasMixedCompositionInResponse(response *yaml.Node, doc *yaml.Node) bool {
	// Navigate to schema content
	content := getNodeValue(response, "content")
	if content == nil || content.Kind != yaml.MappingNode {
		return false
	}

	// Check each media type
	for i := 0; i < len(content.Content); i += 2 {
		mediaTypeNode := content.Content[i+1]
		schema := getNodeValue(mediaTypeNode, "schema")
		if schema != nil && hasMixedCompositionInSchema(schema, doc) {
			return true
		}
	}

	return false
}

// hasMixedCompositionInSchema checks if a schema contains mixed pagination types in compositions
func hasMixedCompositionInSchema(schema *yaml.Node, doc *yaml.Node) bool {
	// Handle $ref by resolving it first
	if ref := getNodeValue(schema, "$ref"); ref != nil {
		refPath := ref.Value
		resolvedSchema := resolveRef(refPath, doc)
		if resolvedSchema != nil {
			return hasMixedCompositionInSchema(resolvedSchema, doc)
		}
		return false
	}

	// Check oneOf, anyOf, allOf compositions
	compositions := []string{"oneOf", "anyOf", "allOf"}
	for _, compType := range compositions {
		if comp := getNodeValue(schema, compType); comp != nil {
			if hasMixedTypesInComposition(comp, doc) {
				return true
			}
		}
	}

	// Recursively check properties
	if properties := getNodeValue(schema, "properties"); properties != nil {
		for i := 0; i < len(properties.Content); i += 2 {
			propNode := properties.Content[i+1]
			if hasMixedCompositionInSchema(propNode, doc) {
				return true
			}
		}
	}

	return false
}

// hasMixedTypesInComposition checks if a composition contains both paginated and non-paginated types
func hasMixedTypesInComposition(composition *yaml.Node, doc *yaml.Node) bool {
	if composition.Kind != yaml.SequenceNode {
		return false
	}

	hasPlainArray := false
	hasPaginatedObject := false

	for _, item := range composition.Content {
		if isPlainArraySchema(item, doc) {
			hasPlainArray = true
		} else if isPaginatedObjectSchema(item, doc) {
			hasPaginatedObject = true
		}

		// Early exit if we found both types
		if hasPlainArray && hasPaginatedObject {
			return true
		}
	}

	return false
}

// isPlainArraySchema checks if a schema represents a plain array (non-paginated)
func isPlainArraySchema(schema *yaml.Node, doc *yaml.Node) bool {
	// Handle $ref by resolving it first
	if ref := getNodeValue(schema, "$ref"); ref != nil {
		refPath := ref.Value
		resolvedSchema := resolveRef(refPath, doc)
		if resolvedSchema != nil {
			return isPlainArraySchema(resolvedSchema, doc)
		}
		return false
	}

	// Check if it's a plain array type
	if typeNode := getNodeValue(schema, "type"); typeNode != nil {
		return typeNode.Value == "array"
	}

	return false
}

// isPaginatedObjectSchema checks if a schema represents a paginated object
func isPaginatedObjectSchema(schema *yaml.Node, doc *yaml.Node) bool {
	// Handle $ref by resolving it first
	if ref := getNodeValue(schema, "$ref"); ref != nil {
		refPath := ref.Value
		resolvedSchema := resolveRef(refPath, doc)
		if resolvedSchema != nil {
			return isPaginatedObjectSchema(resolvedSchema, doc)
		}
		return false
	}

	// Check if it's an object with pagination fields
	if typeNode := getNodeValue(schema, "type"); typeNode != nil && typeNode.Value == "object" {
		fields := extractFieldsFromSchemaWithDoc(schema, doc)
		for _, field := range fields {
			// Check if any field belongs to pagination strategies
			for _, strategy := range PaginationStrategies {
				for _, strategyField := range strategy.Fields {
					if matchesField(field, strategyField) {
						return true
					}
				}
			}
		}
	}

	return false
}

// Helper functions

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

func getStringValue(node *yaml.Node, key string) string {
	valueNode := getNodeValue(node, key)
	if valueNode != nil {
		return valueNode.Value
	}
	return ""
}

func extractFieldsFromResponse(response *yaml.Node) []string {
	var fields []string

	content := getNodeValue(response, "content")
	if content == nil {
		return fields
	}

	// Walk through content types
	if content.Kind == yaml.MappingNode {
		for i := 1; i < len(content.Content); i += 2 {
			mediaTypeNode := content.Content[i]
			schema := getNodeValue(mediaTypeNode, "schema")
			if schema != nil {
				fields = append(fields, extractFieldsFromSchema(schema)...)
			}
		}
	}

	return fields
}

// extractFieldsFromResponseWithDoc extracts fields from response with document context for $ref resolution
func extractFieldsFromResponseWithDoc(response *yaml.Node, doc *yaml.Node) []string {
	var fields []string

	content := getNodeValue(response, "content")
	if content == nil {
		return fields
	}

	// Walk through content types
	if content.Kind == yaml.MappingNode {
		for i := 1; i < len(content.Content); i += 2 {
			mediaTypeNode := content.Content[i]
			schema := getNodeValue(mediaTypeNode, "schema")
			if schema != nil {
				fields = append(fields, extractFieldsFromSchemaWithDoc(schema, doc)...)
			}
		}
	}

	return fields
}

func extractFieldsFromSchema(schema *yaml.Node) []string {
	var fields []string

	if schema == nil || schema.Kind != yaml.MappingNode {
		return fields
	}

	// Handle $ref - note: this version can't resolve refs without document context
	if ref := getNodeValue(schema, "$ref"); ref != nil {
		// Can't resolve $ref without document context, return empty
		return fields
	}

	// Handle direct properties
	if properties := getNodeValue(schema, "properties"); properties != nil {
		fields = append(fields, extractFieldsFromProperties(properties)...)
	}

	// Handle oneOf, anyOf, allOf
	if oneOf := getNodeValue(schema, "oneOf"); oneOf != nil {
		fields = append(fields, extractFieldsFromComposition(oneOf)...)
	}
	if anyOf := getNodeValue(schema, "anyOf"); anyOf != nil {
		fields = append(fields, extractFieldsFromComposition(anyOf)...)
	}
	if allOf := getNodeValue(schema, "allOf"); allOf != nil {
		fields = append(fields, extractFieldsFromComposition(allOf)...)
	}

	return fields
}

// extractFieldsFromSchemaWithDoc extracts fields from schema with document context for $ref resolution
func extractFieldsFromSchemaWithDoc(schema *yaml.Node, doc *yaml.Node) []string {
	var fields []string

	if schema == nil || schema.Kind != yaml.MappingNode {
		return fields
	}

	// Handle $ref by resolving it
	if ref := getNodeValue(schema, "$ref"); ref != nil {
		refPath := ref.Value
		resolvedSchema := resolveRef(refPath, doc)
		if resolvedSchema != nil {
			return extractFieldsFromSchemaWithDoc(resolvedSchema, doc)
		}
		return fields
	}

	// Handle direct properties
	if properties := getNodeValue(schema, "properties"); properties != nil {
		fields = append(fields, extractFieldsFromProperties(properties)...)
	}

	// Handle oneOf, anyOf, allOf
	if oneOf := getNodeValue(schema, "oneOf"); oneOf != nil {
		fields = append(fields, extractFieldsFromCompositionWithDoc(oneOf, doc)...)
	}
	if anyOf := getNodeValue(schema, "anyOf"); anyOf != nil {
		fields = append(fields, extractFieldsFromCompositionWithDoc(anyOf, doc)...)
	}
	if allOf := getNodeValue(schema, "allOf"); allOf != nil {
		fields = append(fields, extractFieldsFromCompositionWithDoc(allOf, doc)...)
	}

	return fields
}

// resolveRef resolves a $ref path to the actual schema node
func resolveRef(refPath string, doc *yaml.Node) *yaml.Node {
	if doc == nil || !strings.HasPrefix(refPath, "#/") {
		return nil
	}

	// Remove the "#/" prefix and split the path
	path := strings.TrimPrefix(refPath, "#/")
	parts := strings.Split(path, "/")

	current := doc
	for _, part := range parts {
		current = getNodeValue(current, part)
		if current == nil {
			return nil
		}
	}

	return current
}

func extractFieldsFromProperties(properties *yaml.Node) []string {
	var fields []string

	if properties.Kind != yaml.MappingNode {
		return fields
	}

	for i := 0; i < len(properties.Content); i += 2 {
		propName := properties.Content[i].Value
		fields = append(fields, propName)
	}

	return fields
}

func extractFieldsFromComposition(composition *yaml.Node) []string {
	var fields []string

	if composition.Kind != yaml.SequenceNode {
		return fields
	}

	for _, item := range composition.Content {
		fields = append(fields, extractFieldsFromSchema(item)...)
	}

	return fields
}

// extractFieldsFromCompositionWithDoc extracts fields from composition with document context for $ref resolution
func extractFieldsFromCompositionWithDoc(composition *yaml.Node, doc *yaml.Node) []string {
	var fields []string

	if composition.Kind != yaml.SequenceNode {
		return fields
	}

	for _, item := range composition.Content {
		fields = append(fields, extractFieldsFromSchemaWithDoc(item, doc)...)
	}

	return fields
}

func matchesParam(paramName, strategyParam string) bool {
	// Simple exact match for now, could be enhanced with fuzzy matching
	return strings.EqualFold(paramName, strategyParam)
}

func matchesField(fieldName, strategyField string) bool {
	// Simple exact match for now, could be enhanced with fuzzy matching
	return strings.EqualFold(fieldName, strategyField)
}

func isSuccessResponse(code string) bool {
	// Consider 2xx and some 3xx responses as success
	if matched, _ := regexp.MatchString(`^[23]\d\d$`, code); matched {
		return true
	}
	// Also handle default response
	return code == "default"
}

func isPaginationRelevantResponse(code string) bool {
	// Include 4xx responses that might contain pagination metadata
	// This is useful for comprehensive pagination field detection
	if matched, _ := regexp.MatchString(`^4\d\d$`, code); matched {
		return true
	}
	return false
}
