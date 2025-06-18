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
	return DetectPaginationInParamsWithDoc(params, nil)
}

// DetectPaginationInParamsWithDoc detects pagination strategies in operation parameters with document context for $ref resolution
func DetectPaginationInParamsWithDoc(params *yaml.Node, doc *yaml.Node) []DetectedPagination {
	var detected []DetectedPagination

	if params == nil || params.Kind != yaml.SequenceNode {
		return detected
	}

	strategyParams := collectStrategyParams(params, doc)

	// Convert to DetectedPagination, filtering out weak strategies
	detected = filterWeakStrategies(strategyParams)

	return detected
}

// collectStrategyParams scans through parameters and collects which strategies each parameter belongs to
func collectStrategyParams(params *yaml.Node, doc *yaml.Node) map[string][]string {
	strategyParams := make(map[string][]string)

	// Scan through parameters
	for _, param := range params.Content {
		if param.Kind != yaml.MappingNode {
			continue
		}

		paramName := extractParameterName(param, doc)
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

	return strategyParams
}

// extractParameterName extracts the parameter name from a param node, handling $ref resolution
func extractParameterName(param *yaml.Node, doc *yaml.Node) string {
	var paramName string

	// Handle $ref by resolving it first
	if ref := getNodeValue(param, "$ref"); ref != nil && doc != nil {
		refPath := ref.Value
		resolvedParam := resolveRef(refPath, doc)
		if resolvedParam != nil {
			paramName = getStringValue(resolvedParam, "name")
		}
	} else {
		paramName = getStringValue(param, "name")
	}

	return paramName
}

// filterWeakStrategies converts strategy params to DetectedPagination, filtering out weak strategies
func filterWeakStrategies(strategyParams map[string][]string) []DetectedPagination {
	var detected []DetectedPagination

	// A strategy is considered "weak" if it only has shared parameters
	sharedParams := findSharedParams()

	for strategy, params := range strategyParams {
		if hasNonSharedParams(params, sharedParams) {
			detected = append(detected, DetectedPagination{
				Strategy:   strategy,
				Parameters: params,
			})
		}
	}

	return detected
}

// hasNonSharedParams checks if a strategy has any non-shared parameters
func hasNonSharedParams(params []string, sharedParams map[string]bool) bool {
	for _, param := range params {
		if !sharedParams[param] {
			return true
		}
	}
	return false
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

	params := getNodeValue(operation, "parameters")
	responses := getNodeValue(operation, "responses")

	strategies := detectPaginationStrategies(params, responses, doc)
	if len(strategies.paramStrategies) == 0 {
		return result, nil
	}

	if !needsProcessingCheck(strategies, params, responses, doc) {
		return result, nil
	}

	selectedStrategy := selectBestStrategy(strategies, opts)
	if selectedStrategy == "" {
		return result, nil
	}

	return processEndpointCleanup(params, responses, selectedStrategy, strategies.allPagination, doc, result)
}

// detectPaginationStrategies extracts pagination strategies from params and responses
func detectPaginationStrategies(params, responses *yaml.Node, doc *yaml.Node) *paginationStrategies {
	paramPagination := DetectPaginationInParamsWithDoc(params, doc)
	responsePagination := DetectPaginationInResponsesWithDoc(responses, doc)

	paramStrategies := make(map[string]bool)
	for _, p := range paramPagination {
		paramStrategies[p.Strategy] = true
	}

	responseStrategies := make(map[string]bool)
	for _, r := range responsePagination {
		responseStrategies[r.Strategy] = true
	}

	allPagination := append(paramPagination, responsePagination...)

	return &paginationStrategies{
		paramStrategies:    paramStrategies,
		responseStrategies: responseStrategies,
		allPagination:      allPagination,
	}
}

// paginationStrategies holds detected pagination strategy information
type paginationStrategies struct {
	paramStrategies    map[string]bool
	responseStrategies map[string]bool
	allPagination      []DetectedPagination
}

// needsProcessingCheck determines if endpoint processing is needed
func needsProcessingCheck(strategies *paginationStrategies, params, responses *yaml.Node, doc *yaml.Node) bool {
	if len(strategies.paramStrategies) > 1 {
		return true
	}

	if hasResponseCleanupNeeded(strategies) {
		return true
	}

	if hasOrphanedSharedParamsWithDoc(params, strategies.paramStrategies, doc) {
		return true
	}

	if responses != nil && hasMixedResponseCompositions(responses, doc) {
		return true
	}

	return false
}

// hasResponseCleanupNeeded checks if response cleanup is needed
func hasResponseCleanupNeeded(strategies *paginationStrategies) bool {
	for responseStrategy := range strategies.responseStrategies {
		if !strategies.paramStrategies[responseStrategy] {
			return true
		}
	}
	return false
}

// hasOrphanedSharedParams checks for orphaned shared parameters
func hasOrphanedSharedParams(params *yaml.Node, paramStrategies map[string]bool) bool {
	return hasOrphanedSharedParamsWithDoc(params, paramStrategies, nil)
}

// hasOrphanedSharedParamsWithDoc checks for orphaned shared parameters with document context for $ref resolution
func hasOrphanedSharedParamsWithDoc(params *yaml.Node, paramStrategies map[string]bool, doc *yaml.Node) bool {
	if params == nil {
		return false
	}

	sharedParams := findSharedParams()
	for _, param := range params.Content {
		if param.Kind != yaml.MappingNode {
			continue
		}

		var paramName string
		var resolvedParam *yaml.Node

		// Handle $ref by resolving it first
		if ref := getNodeValue(param, "$ref"); ref != nil && doc != nil {
			refPath := ref.Value
			resolvedParam = resolveRef(refPath, doc)
			if resolvedParam != nil {
				paramName = getStringValue(resolvedParam, "name")
			}
		} else {
			paramName = getStringValue(param, "name")
		}

		if paramName == "" || !sharedParams[paramName] {
			continue
		}

		if !belongsToAnyDetectedStrategy(paramName, paramStrategies) {
			return true
		}
	}
	return false
}

// belongsToAnyDetectedStrategy checks if parameter belongs to any detected strategy
func belongsToAnyDetectedStrategy(paramName string, paramStrategies map[string]bool) bool {
	for strategy := range paramStrategies {
		for _, strategyParam := range PaginationStrategies[strategy].Params {
			if matchesParam(paramName, strategyParam) {
				return true
			}
		}
	}
	return false
}

// selectBestStrategy selects the best strategy based on priority
func selectBestStrategy(strategies *paginationStrategies, opts Options) string {
	allStrategies := make(map[string]bool)
	for strategy := range strategies.paramStrategies {
		allStrategies[strategy] = true
	}
	for strategy := range strategies.responseStrategies {
		allStrategies[strategy] = true
	}

	// First pass: look for strategies that have parameters
	for _, priority := range opts.Priority {
		if priority == "none" && len(allStrategies) > 0 {
			return "none"
		}
		if strategies.paramStrategies[priority] {
			return priority
		}
	}

	// Second pass: look for response-only strategies
	for _, priority := range opts.Priority {
		if strategies.responseStrategies[priority] && !strategies.paramStrategies[priority] {
			return priority
		}
	}

	return ""
}

// processEndpointCleanup performs the actual cleanup of params and responses
func processEndpointCleanup(params, responses *yaml.Node, selectedStrategy string, allPagination []DetectedPagination, doc *yaml.Node, result *ProcessResult) (*ProcessResult, error) {
	if params != nil {
		removed := removeUnwantedParamsWithDoc(params, selectedStrategy, allPagination, doc)
		result.RemovedParams = removed
		if len(removed) > 0 {
			result.Changed = true
		}
	}

	if responses != nil {
		removed, modified := removeUnwantedResponsesWithDoc(responses, selectedStrategy, allPagination, doc)
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
	return removeUnwantedParamsWithDoc(params, selectedStrategy, detected, nil)
}

// removeUnwantedParamsWithDoc removes parameters that don't match the selected strategy with document context for $ref resolution
func removeUnwantedParamsWithDoc(params *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) []string {
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

		var paramName string
		var resolvedParam *yaml.Node

		// Handle $ref by resolving it first
		if ref := getNodeValue(param, "$ref"); ref != nil && doc != nil {
			refPath := ref.Value
			resolvedParam = resolveRef(refPath, doc)
			if resolvedParam != nil {
				paramName = getStringValue(resolvedParam, "name")
			}
		} else {
			paramName = getStringValue(param, "name")
		}

		if paramName == "" {
			newContent = append(newContent, param)
			continue
		}

		shouldKeep := shouldKeepParameter(paramName, selectedStrategy, detected)
		if shouldKeep {
			newContent = append(newContent, param)
		} else {
			removed = append(removed, paramName)
		}
	}

	params.Content = newContent
	return removed
}

// shouldKeepParameter determines if a parameter should be kept based on the selected strategy
func shouldKeepParameter(paramName, selectedStrategy string, detected []DetectedPagination) bool {
	// Special handling for "none" strategy - remove all pagination parameters
	if selectedStrategy == "none" {
		return !isPaginationParameter(paramName, detected)
	}

	// Check if this param belongs to the selected strategy
	if belongsToStrategy(paramName, selectedStrategy) {
		return true
	}

	// If it doesn't belong to selected strategy, check if it belongs to any pagination strategy
	return !belongsToAnyPaginationStrategy(paramName, selectedStrategy, detected)
}

// isPaginationParameter checks if a parameter is a pagination parameter
func isPaginationParameter(paramName string, detected []DetectedPagination) bool {
	for _, d := range detected {
		for _, p := range d.Parameters {
			if p == paramName {
				return true
			}
		}
	}
	return false
}

// belongsToStrategy checks if a parameter belongs to a specific strategy
func belongsToStrategy(paramName, strategy string) bool {
	selectedParams := PaginationStrategies[strategy].Params
	for _, selectedParam := range selectedParams {
		if matchesParam(paramName, selectedParam) {
			return true
		}
	}
	return false
}

// belongsToAnyPaginationStrategy checks if a parameter belongs to any pagination strategy (detected or not)
func belongsToAnyPaginationStrategy(paramName, selectedStrategy string, detected []DetectedPagination) bool {
	// First check detected strategies
	for _, d := range detected {
		if d.Strategy != selectedStrategy {
			for _, p := range d.Parameters {
				if p == paramName {
					return true
				}
			}
		}
	}

	// Also check if this param belongs to any pagination strategy that wasn't detected
	for strategyName, strategy := range PaginationStrategies {
		if strategyName != selectedStrategy {
			for _, strategyParam := range strategy.Params {
				if matchesParam(paramName, strategyParam) {
					return true
				}
			}
		}
	}

	return false
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

	var newContent []*yaml.Node

	for i := 0; i < len(responses.Content); i += 2 {
		responseCode := responses.Content[i]
		responseNode := responses.Content[i+1]

		processResult := processResponseForCleanup(responseNode, selectedStrategy, detected, doc)

		newContent = append(newContent, responseCode, responseNode)
		if len(processResult.modifications) > 0 {
			modifiedSchemas = append(modifiedSchemas, processResult.modifications...)
		}
	}

	responses.Content = newContent
	return removedResponses, modifiedSchemas
}

// responseCleanupResult holds the result of processing a response for cleanup
type responseCleanupResult struct {
	modifications []string
}

// processResponseForCleanup processes a single response for pagination cleanup
func processResponseForCleanup(responseNode *yaml.Node, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) responseCleanupResult {
	var fields []string
	if doc != nil {
		fields = extractFieldsFromResponseWithDoc(responseNode, doc)
	} else {
		fields = extractFieldsFromResponse(responseNode)
	}

	if selectedStrategy == "none" {
		return processResponseForNoneCleanup(responseNode, fields, detected, doc)
	}

	return processResponseForStrategyCleanup(responseNode, fields, selectedStrategy, detected, doc)
}

// processResponseForNoneCleanup handles cleanup for "none" strategy
func processResponseForNoneCleanup(responseNode *yaml.Node, fields []string, detected []DetectedPagination, doc *yaml.Node) responseCleanupResult {
	containsPaginationFields := checkForPaginationFields(fields, detected)

	var modifications []string
	if containsPaginationFields {
		modifications = cleanResponseSchemaWithDoc(responseNode, "none", detected, doc)
	}

	return responseCleanupResult{modifications: modifications}
}

// processResponseForStrategyCleanup handles cleanup for specific strategies
func processResponseForStrategyCleanup(responseNode *yaml.Node, fields []string, selectedStrategy string, detected []DetectedPagination, doc *yaml.Node) responseCleanupResult {
	containsUnwanted := checkForUnwantedFields(fields, selectedStrategy, detected)

	var modifications []string
	if containsUnwanted {
		modifications = cleanResponseSchemaWithDoc(responseNode, selectedStrategy, detected, doc)
	} else if hasMixedCompositionInResponse(responseNode, doc) {
		modifications = cleanResponseSchemaWithDoc(responseNode, selectedStrategy, detected, doc)
	}

	return responseCleanupResult{modifications: modifications}
}

// checkForPaginationFields checks if fields contain any pagination fields from detected strategies
func checkForPaginationFields(fields []string, detected []DetectedPagination) bool {
	for _, field := range fields {
		for _, d := range detected {
			for _, strategyField := range d.Fields {
				if matchesField(field, strategyField) {
					return true
				}
			}
		}
	}
	return false
}

// checkForUnwantedFields checks if fields contain unwanted pagination fields
func checkForUnwantedFields(fields []string, selectedStrategy string, detected []DetectedPagination) bool {
	for _, field := range fields {
		if fieldBelongsToUnwantedStrategy(field, selectedStrategy, detected) {
			return true
		}
	}
	return false
}

// fieldBelongsToUnwantedStrategy checks if a field belongs to an unwanted strategy
func fieldBelongsToUnwantedStrategy(field, selectedStrategy string, detected []DetectedPagination) bool {
	// Check detected strategies
	if fieldBelongsToNonSelectedDetectedStrategy(field, selectedStrategy, detected) {
		return true
	}

	// Check all pagination strategies that weren't detected
	return fieldBelongsToNonSelectedPaginationStrategy(field, selectedStrategy)
}

// fieldBelongsToNonSelectedDetectedStrategy checks if field belongs to non-selected detected strategies
func fieldBelongsToNonSelectedDetectedStrategy(field, selectedStrategy string, detected []DetectedPagination) bool {
	for _, d := range detected {
		if d.Strategy != selectedStrategy {
			for _, unwantedField := range d.Fields {
				if matchesField(field, unwantedField) {
					return true
				}
			}
		}
	}
	return false
}

// fieldBelongsToNonSelectedPaginationStrategy checks if field belongs to non-selected pagination strategies
func fieldBelongsToNonSelectedPaginationStrategy(field, selectedStrategy string) bool {
	for strategyName, strategy := range PaginationStrategies {
		if strategyName != selectedStrategy {
			for _, strategyField := range strategy.Fields {
				if matchesField(field, strategyField) {
					return true
				}
			}
		}
	}
	return false
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

		shouldRemove := shouldRemoveProperty(propName, selectedStrategy, detected, properties)

		if !shouldRemove {
			newContent = append(newContent, properties.Content[i], propNode)
		} else {
			modified = true
		}
	}

	if modified {
		properties.Content = newContent
	}

	return modified
}

// shouldRemoveProperty determines if a property should be removed
func shouldRemoveProperty(propName, selectedStrategy string, detected []DetectedPagination, properties *yaml.Node) bool {
	if selectedStrategy == "none" {
		return shouldRemoveForNoneStrategy(propName, detected)
	}

	return shouldRemoveForOtherStrategy(propName, selectedStrategy, detected, properties)
}

// shouldRemoveForNoneStrategy handles removal logic for "none" strategy
func shouldRemoveForNoneStrategy(propName string, detected []DetectedPagination) bool {
	// Check if this property is any pagination field (from detected or all strategies)
	for _, d := range detected {
		for _, field := range d.Fields {
			if matchesField(propName, field) {
				return true
			}
		}
	}

	// Also check against all strategy definitions for "none" strategy
	for _, strategy := range PaginationStrategies {
		for _, strategyField := range strategy.Fields {
			if matchesField(propName, strategyField) {
				return true
			}
		}
	}

	return false
}

// shouldRemoveForOtherStrategy handles removal logic for non-"none" strategies
func shouldRemoveForOtherStrategy(propName, selectedStrategy string, detected []DetectedPagination, properties *yaml.Node) bool {
	belongsToSelected := belongsToSelectedStrategy(propName, selectedStrategy)
	belongsToNonSelected := belongsToNonSelectedStrategy(propName, selectedStrategy, detected)

	if belongsToSelected && belongsToNonSelected {
		return handleSharedFieldDecision(propName, selectedStrategy, detected, properties)
	}

	if belongsToSelected && !belongsToNonSelected {
		return false // Keep fields that only belong to selected strategy
	}

	if !belongsToSelected && belongsToNonSelected {
		return true // Remove fields that only belong to non-selected strategy
	}

	// Field doesn't belong to any detected strategy, check all strategies
	return belongsToAnyNonSelectedStrategy(propName, selectedStrategy)
}

// belongsToSelectedStrategy checks if property belongs to the selected strategy
func belongsToSelectedStrategy(propName, selectedStrategy string) bool {
	selectedStrategyDef := PaginationStrategies[selectedStrategy]
	for _, selectedField := range selectedStrategyDef.Fields {
		if matchesField(propName, selectedField) {
			return true
		}
	}
	return false
}

// belongsToNonSelectedStrategy checks if property belongs to any non-selected detected strategy
func belongsToNonSelectedStrategy(propName, selectedStrategy string, detected []DetectedPagination) bool {
	for _, d := range detected {
		if d.Strategy != selectedStrategy {
			for _, field := range d.Fields {
				if matchesField(propName, field) {
					return true
				}
			}
		}
	}
	return false
}

// handleSharedFieldDecision decides whether to keep or remove shared fields
func handleSharedFieldDecision(propName, selectedStrategy string, detected []DetectedPagination, properties *yaml.Node) bool {
	selectedStrategyDef := PaginationStrategies[selectedStrategy]

	hasSelectedStrategyFields, hasNonSelectedStrategyFields := analyzeSchemaContext(
		propName, selectedStrategy, selectedStrategyDef, detected, properties)

	// If this schema has fields from selected strategy, keep shared fields
	// If this schema only has fields from non-selected strategies, remove shared fields
	if hasSelectedStrategyFields {
		return false
	} else if hasNonSelectedStrategyFields {
		return true
	}

	// No clear indicators, default to keeping shared fields
	return false
}

// analyzeSchemaContext analyzes the schema context to determine strategy indicators
func analyzeSchemaContext(propName, selectedStrategy string, selectedStrategyDef Strategy, detected []DetectedPagination, properties *yaml.Node) (bool, bool) {
	hasSelectedStrategyFields := false
	hasNonSelectedStrategyFields := false

	if properties.Kind != yaml.MappingNode {
		return hasSelectedStrategyFields, hasNonSelectedStrategyFields
	}

	for j := 0; j < len(properties.Content); j += 2 {
		siblingName := properties.Content[j].Value
		if siblingName == propName {
			continue // skip self
		}

		// Check if sibling belongs to selected strategy
		for _, selectedField := range selectedStrategyDef.Fields {
			if matchesField(siblingName, selectedField) {
				hasSelectedStrategyFields = true
				break
			}
		}

		// Check if sibling belongs to non-selected strategies
		for _, d := range detected {
			if d.Strategy != selectedStrategy {
				for _, field := range d.Fields {
					if matchesField(siblingName, field) {
						hasNonSelectedStrategyFields = true
						break
					}
				}
				if hasNonSelectedStrategyFields {
					break
				}
			}
		}
	}

	return hasSelectedStrategyFields, hasNonSelectedStrategyFields
}

// belongsToAnyNonSelectedStrategy checks if property belongs to any non-selected strategy
func belongsToAnyNonSelectedStrategy(propName, selectedStrategy string) bool {
	for strategyName, strategy := range PaginationStrategies {
		if strategyName == selectedStrategy {
			continue
		}

		for _, strategyField := range strategy.Fields {
			if matchesField(propName, strategyField) {
				return true
			}
		}
	}
	return false
}

// shouldKeepSchemaItem determines if a schema item should be kept
func shouldKeepSchemaItem(item *yaml.Node, selectedStrategy string, detected []DetectedPagination) bool {
	return shouldKeepSchemaItemWithDoc(item, selectedStrategy, detected, nil)
}

// shouldKeepSchemaItemWithDoc determines if a schema item should be kept with document context
func shouldKeepSchemaItemWithDoc(item *yaml.Node, selectedStrategy string, _ []DetectedPagination, doc *yaml.Node) bool {
	if item.Kind != yaml.MappingNode {
		return true // Keep non-object items
	}

	var fields []string
	if doc != nil {
		fields = extractFieldsFromSchemaWithDoc(item, doc)
	} else {
		fields = extractFieldsFromSchema(item)
	}

	if selectedStrategy == "none" {
		return shouldKeepForNoneStrategy(fields)
	}

	return shouldKeepForOtherStrategy(fields, selectedStrategy)
}

// shouldKeepForNoneStrategy determines if schema should be kept for "none" strategy
func shouldKeepForNoneStrategy(fields []string) bool {
	// For "none" strategy, only keep items that have NO pagination fields
	for _, field := range fields {
		if fieldBelongsToAnyPaginationStrategy(field) {
			return false
		}
	}
	return true
}

// fieldBelongsToAnyPaginationStrategy checks if field belongs to any pagination strategy
func fieldBelongsToAnyPaginationStrategy(field string) bool {
	for _, strategy := range PaginationStrategies {
		for _, strategyField := range strategy.Fields {
			if matchesField(field, strategyField) {
				return true
			}
		}
	}
	return false
}

// shouldKeepForOtherStrategy determines if schema should be kept for non-"none" strategies
func shouldKeepForOtherStrategy(fields []string, selectedStrategy string) bool {
	containsSelectedUniqueFields := hasUniqueFieldsFromStrategy(fields, selectedStrategy)
	containsOtherStrategyUniqueFields := hasUniqueFieldsFromOtherStrategies(fields, selectedStrategy)

	if containsSelectedUniqueFields {
		return true // Contains unique fields from selected strategy
	}

	if containsOtherStrategyUniqueFields {
		return false // Contains unique fields from other strategies
	}

	// Handle schemas with no pagination fields - remove for consistent pagination behavior
	return false
}

// hasUniqueFieldsFromStrategy checks if fields contain unique fields from the selected strategy
func hasUniqueFieldsFromStrategy(fields []string, selectedStrategy string) bool {
	selectedFields := PaginationStrategies[selectedStrategy].Fields

	for _, field := range fields {
		for _, selectedField := range selectedFields {
			if matchesField(field, selectedField) && isFieldUniqueToStrategy(selectedField, selectedStrategy) {
				return true
			}
		}
	}
	return false
}

// hasUniqueFieldsFromOtherStrategies checks if fields contain unique fields from other strategies
func hasUniqueFieldsFromOtherStrategies(fields []string, selectedStrategy string) bool {
	for _, field := range fields {
		for strategyName, strategy := range PaginationStrategies {
			if strategyName != selectedStrategy {
				for _, strategyField := range strategy.Fields {
					if matchesField(field, strategyField) && isFieldUniqueToStrategy(strategyField, strategyName) {
						return true
					}
				}
			}
		}
	}
	return false
}

// isFieldUniqueToStrategy checks if a field is unique to a specific strategy
func isFieldUniqueToStrategy(field, strategy string) bool {
	for strategyName, strategyDef := range PaginationStrategies {
		if strategyName != strategy {
			for _, otherField := range strategyDef.Fields {
				if matchesField(field, otherField) {
					return false // Field is shared with another strategy
				}
			}
		}
	}
	return true // Field is unique to this strategy
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
