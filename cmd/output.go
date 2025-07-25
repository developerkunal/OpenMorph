package cmd

import (
	"fmt"
	"strings"

	"github.com/developerkunal/OpenMorph/internal/config"
	"github.com/developerkunal/OpenMorph/internal/transform"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorRed    = "\033[31m"
)

// printConfigSummary prints a formatted summary of the configuration
func printConfigSummary(cfg *config.Config, vendorProviders []string, outputFile string) {
	printConfigHeader()
	printCoreSettings(cfg, outputFile)
	printAdditionalSettings(cfg)
	printEnabledFeatures(cfg, vendorProviders)
	printMappingsSection(cfg)
	printConfigFooter()
}

// printConfigHeader prints the configuration header
func printConfigHeader() {
	fmt.Printf("\n%s╭─────────────────────────────────────────────────────────────────╮%s\n", colorCyan, colorReset)
	fmt.Printf("%s│%s %s🔧 OpenMorph Configuration%s %s                               │%s\n", colorCyan, colorReset, colorBold, colorReset, colorCyan, colorReset)
	fmt.Printf("%s╰─────────────────────────────────────────────────────────────────╯%s\n", colorCyan, colorReset)
}

// printCoreSettings prints the core configuration settings
func printCoreSettings(cfg *config.Config, outputFile string) {
	fmt.Printf("\n%s📋 Core Settings%s\n", colorBold, colorReset)
	fmt.Printf("   📁 %sInput:%s         %s%s%s\n", colorCyan, colorReset, colorGreen, cfg.Input, colorReset)
	if outputFile != "" {
		fmt.Printf("   📄 %sOutput:%s        %s%s%s\n", colorCyan, colorReset, colorGreen, outputFile, colorReset)
	}
	fmt.Printf("   💾 %sBackup:%s        %s%v%s\n", colorCyan, colorReset, getStatusColor(cfg.Backup), cfg.Backup, colorReset)
	fmt.Printf("   ✅ %sValidate:%s      %s%v%s\n", colorCyan, colorReset, getStatusColor(cfg.Validate), cfg.Validate, colorReset)
	fmt.Printf("   🔄 %sFlatten:%s       %s%v%s\n", colorCyan, colorReset, getStatusColor(cfg.FlattenResponses), cfg.FlattenResponses, colorReset)
}

// printAdditionalSettings prints additional configuration settings
func printAdditionalSettings(cfg *config.Config) {
	if len(cfg.Exclude) > 0 || len(cfg.PaginationPriority) > 0 {
		fmt.Printf("\n%s⚙️  Additional Settings%s\n", colorBold, colorReset)

		if len(cfg.Exclude) > 0 {
			fmt.Printf("   🚫 %sExclude:%s       %s%v%s\n", colorCyan, colorReset, colorYellow, cfg.Exclude, colorReset)
		}

		if len(cfg.PaginationPriority) > 0 {
			fmt.Printf("   📊 %sPagination:%s    %s%v%s\n", colorCyan, colorReset, colorPurple, cfg.PaginationPriority, colorReset)
		}
	}
}

// printEnabledFeatures prints the enabled features section
func printEnabledFeatures(cfg *config.Config, vendorProviders []string) {
	featureEnabled := cfg.VendorExtensions.Enabled || cfg.DefaultValues.Enabled
	if !featureEnabled {
		return
	}

	fmt.Printf("\n%s🚀 Enabled Features%s\n", colorBold, colorReset)

	// Vendor extensions
	if cfg.VendorExtensions.Enabled {
		printVendorExtensionFeature(cfg, vendorProviders)
	}

	// Default values
	if cfg.DefaultValues.Enabled {
		printDefaultValuesFeature(cfg)
	}
}

// printVendorExtensionFeature prints vendor extension feature details
func printVendorExtensionFeature(cfg *config.Config, vendorProviders []string) {
	fmt.Printf("   🏷️  %sVendor Extensions%s\n", colorGreen, colorReset)
	if len(vendorProviders) > 0 {
		fmt.Printf("      %s↳ Target:%s       %s%v%s\n", colorBlue, colorReset, colorGreen, vendorProviders, colorReset)
	} else {
		providerNames := make([]string, 0, len(cfg.VendorExtensions.Providers))
		for name := range cfg.VendorExtensions.Providers {
			providerNames = append(providerNames, name)
		}
		if len(providerNames) > 0 {
			fmt.Printf("      %s↳ Providers:%s    %s%v%s\n", colorBlue, colorReset, colorGreen, providerNames, colorReset)
		}
	}
}

// printDefaultValuesFeature prints default values feature details
func printDefaultValuesFeature(cfg *config.Config) {
	fmt.Printf("   ⚙️  %sDefault Values%s\n", colorGreen, colorReset)
	if len(cfg.DefaultValues.Rules) > 0 {
		fmt.Printf("      %s↳ Rules:%s        %s%d configured%s\n", colorBlue, colorReset, colorGreen, len(cfg.DefaultValues.Rules), colorReset)
	}
}

// printMappingsSection prints the mappings section
func printMappingsSection(cfg *config.Config) {
	// Mappings
	if len(cfg.Mappings) > 0 {
		fmt.Printf("\n%s🔀 Key Mappings%s\n", colorBold, colorReset)
		for k, v := range cfg.Mappings {
			fmt.Printf("   %s%s%s %s→%s %s%s%s\n", colorYellow, k, colorReset, colorGreen, colorReset, colorBlue, v, colorReset)
		}
	}
}

// printConfigFooter prints the configuration footer
func printConfigFooter() {
	fmt.Printf("\n%s┌─────────────────────────────────────────────────────────────────┐%s\n", colorGreen, colorReset)
	fmt.Printf("%s│%s %s🚀 Starting transformation...%s %s                            │%s\n", colorGreen, colorReset, colorBold, colorReset, colorGreen, colorReset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────────┘%s\n", colorGreen, colorReset)
}

// getStatusColor returns green for true, red for false
func getStatusColor(status bool) string {
	if status {
		return colorGreen
	}
	return colorRed
}

// Utility functions for consistent output formatting
func printHeader(title string, emoji string) {
	fmt.Printf("\n%s %s%s%s%s\n", emoji, colorBold, title, colorReset, colorReset)
	fmt.Printf("═══════════════════════════════════════════\n")
}

func printSuccess(message string) {
	fmt.Printf("\n%s✅ %s%s\n", colorGreen, message, colorReset)
}

func printInfo(message string) {
	fmt.Printf("⏭️  %s%s%s\n", colorYellow, message, colorReset)
}

func printFileHeader(filename string) {
	fmt.Printf("   %s📁 %s%s%s\n", colorBlue, colorBold, filename, colorReset)
}

func printListItem(text string, itemColor string) {
	fmt.Printf("      %s• %s%s%s\n", itemColor, colorReset, text, colorReset)
}

// Pagination results printing
func printPaginationResults(paginationResult *transform.PaginationResult) {
	if paginationResult.Changed {
		fmt.Printf("\n%s╭─────────────────────────────────────────────────────────────────╮%s\n", colorBlue, colorReset)
		fmt.Printf("%s│%s %s🔄 Pagination Processing Results%s %s                        │%s\n", colorBlue, colorReset, colorBold, colorReset, colorBlue, colorReset)
		fmt.Printf("%s╰─────────────────────────────────────────────────────────────────╯%s\n", colorBlue, colorReset)

		fmt.Printf("\n� %sSummary:%s %s%d files processed%s\n",
			colorCyan, colorReset, colorGreen, len(paginationResult.ProcessedFiles), colorReset)

		// Print removed parameters with better formatting
		if len(paginationResult.RemovedParams) > 0 {
			fmt.Printf("\n%s🗑️  Removed Parameters%s\n", colorRed, colorReset)
			for file, params := range paginationResult.RemovedParams {
				fmt.Printf("   %s●%s %s%s%s\n", colorYellow, colorReset, colorBold, file, colorReset)
				for _, param := range params {
					fmt.Printf("     %s▸%s %s%s%s\n", colorRed, colorReset, colorRed, param, colorReset)
				}
			}
		}

		fmt.Printf("\n%s┌─────────────────────────────────────────────────────────────────┐%s\n", colorGreen, colorReset)
		fmt.Printf("%s│%s %s✅ Pagination cleanup completed successfully%s %s              │%s\n", colorGreen, colorReset, colorBold, colorReset, colorGreen, colorReset)
		fmt.Printf("%s└─────────────────────────────────────────────────────────────────┘%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("\n%s⏭️  No pagination changes needed%s\n", colorYellow, colorReset)
	}
}

// printFlattenResultsImproved prints flatten results with better formatting
func printFlattenResultsImproved(flattenResult *transform.FlattenResult) {
	if flattenResult.Changed {
		printFlattenHeader(flattenResult)
		printFlattenedRefs(flattenResult.FlattenedRefs)
		printRemovedComponents(flattenResult.RemovedComponents)
		printSuccess("Response flattening completed successfully")
	} else {
		printInfo("No flattening changes needed")
	}
}

// printFlattenHeader prints the header for flatten results
func printFlattenHeader(flattenResult *transform.FlattenResult) {
	fmt.Printf("\n%s📋 Response Flattening Results%s\n", colorBold, colorReset)
	fmt.Printf("%s┌─────────────────────────────────────────────────────────────────┐%s\n", colorCyan, colorReset)
	fmt.Printf("%s│%s %s📄 Processed files:%s %s%d%s %s                                 │%s\n",
		colorCyan, colorReset, colorBold, colorReset, colorGreen, len(flattenResult.ProcessedFiles), colorReset, colorCyan, colorReset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────────┘%s\n", colorCyan, colorReset)
}

// printFlattenedRefs prints flattened references with categorization
func printFlattenedRefs(flattenedRefs map[string][]string) {
	if len(flattenedRefs) == 0 {
		return
	}

	fmt.Printf("\n%s🔧 Schema Transformations%s\n", colorGreen, colorReset)

	for file, refs := range flattenedRefs {
		fmt.Printf("\n   %s●%s %s%s%s\n", colorYellow, colorReset, colorBold, file, colorReset)
		printCategorizedRefs(refs)
	}
}

// printCategorizedRefs categorizes and prints references
func printCategorizedRefs(refs []string) {
	categories := categorizeRefs(refs)

	printRefCategory("🔗 oneOf Flattening", categories.oneOf, colorPurple)
	printRefCategory("🔗 anyOf Flattening", categories.anyOf, colorPurple)
	printRefCategory("🔗 allOf Flattening", categories.allOf, colorPurple)
	printRefCategory("🛤️  Path References", categories.paths, colorBlue)
	printRefCategory("⚙️  Other Transformations", categories.others, colorYellow)
}

// refCategories holds categorized references
type refCategories struct {
	oneOf  []string
	anyOf  []string
	allOf  []string
	paths  []string
	others []string
}

// categorizeRefs groups references by transformation type
func categorizeRefs(refs []string) refCategories {
	categories := refCategories{}

	for _, ref := range refs {
		switch {
		case strings.Contains(ref, ".oneOf ->"):
			categories.oneOf = append(categories.oneOf, ref)
		case strings.Contains(ref, ".anyOf ->"):
			categories.anyOf = append(categories.anyOf, ref)
		case strings.Contains(ref, ".allOf ->"):
			categories.allOf = append(categories.allOf, ref)
		case isPathRef(ref):
			categories.paths = append(categories.paths, ref)
		default:
			categories.others = append(categories.others, ref)
		}
	}

	return categories
}

// isPathRef checks if a reference is a path reference
func isPathRef(ref string) bool {
	return strings.HasPrefix(ref, "get ") || strings.HasPrefix(ref, "post ") ||
		strings.HasPrefix(ref, "put ") || strings.HasPrefix(ref, "delete ") ||
		strings.HasPrefix(ref, "/")
}

// printRefCategory prints a category of references
func printRefCategory(title string, refs []string, titleColor string) {
	if len(refs) == 0 {
		return
	}

	fmt.Printf("     %s%s%s (%s%d items%s)\n", titleColor, title, colorReset, colorYellow, len(refs), colorReset)
	for _, ref := range refs {
		printFormattedRef(ref)
	}
}

// printFormattedRef prints a reference with proper formatting
func printFormattedRef(ref string) {
	switch {
	case strings.Contains(ref, " -> "):
		parts := strings.Split(ref, " -> ")
		if len(parts) == 2 {
			fmt.Printf("       %s▸%s %s%s%s\n", colorCyan, colorReset, colorYellow, parts[0], colorReset)
			fmt.Printf("         %s→%s %s%s%s\n", colorGreen, colorReset, colorBlue, parts[1], colorReset)
		} else {
			fmt.Printf("       %s▸%s %s%s%s\n", colorCyan, colorReset, colorYellow, ref, colorReset)
		}
	case strings.Contains(ref, ": "):
		parts := strings.Split(ref, ": ")
		if len(parts) >= 2 {
			fmt.Printf("       %s▸%s %s%s%s\n", colorCyan, colorReset, colorYellow, parts[0], colorReset)
			fmt.Printf("         %s→%s %s%s%s\n", colorGreen, colorReset, colorBlue, strings.Join(parts[1:], ": "), colorReset)
		} else {
			fmt.Printf("       %s▸%s %s%s%s\n", colorCyan, colorReset, colorYellow, ref, colorReset)
		}
	default:
		fmt.Printf("       %s▸%s %s%s%s\n", colorCyan, colorReset, colorYellow, ref, colorReset)
	}
}

// printRemovedComponents prints removed components
func printRemovedComponents(removedComponents map[string][]string) {
	if len(removedComponents) == 0 {
		return
	}

	fmt.Printf("\n%s🗑️  Removed Components%s\n", colorRed, colorReset)
	for file, components := range removedComponents {
		fmt.Printf("   %s●%s %s%s%s (%s%d components removed%s)\n", colorYellow, colorReset, colorBold, file, colorReset, colorRed, len(components), colorReset)
		for _, component := range components {
			fmt.Printf("     %s▸%s %s%s%s\n", colorRed, colorReset, colorRed, component, colorReset)
		}
	}
}

// Vendor extension results printing
func printVendorExtensionResults(vendorResult *transform.VendorExtensionResult) {
	if vendorResult.Changed {
		printVendorExtensionHeader(vendorResult)
		printAddedExtensions(vendorResult.AddedExtensions)
		printSkippedOperations(vendorResult.SkippedOperations)
		printSuccess("Vendor extensions added successfully")
	} else {
		printInfo("No vendor extension changes needed")
	}
}

func printVendorExtensionHeader(vendorResult *transform.VendorExtensionResult) {
	printHeader("Vendor Extension Processing Results", "🏷️")
	fmt.Printf("📄 %sProcessed files:%s %s%d%s\n",
		colorCyan, colorReset, colorGreen, len(vendorResult.ProcessedFiles), colorReset)
}

func printAddedExtensions(addedExtensions map[string][]string) {
	if len(addedExtensions) == 0 {
		return
	}

	fmt.Printf("\n✅ %sAdded Extensions%s\n", colorGreen, colorReset)
	for file, extensions := range addedExtensions {
		printFileHeader(file)
		strategies := groupExtensionsByStrategy(extensions)
		printGroupedExtensions(strategies)
	}
}

func groupExtensionsByStrategy(extensions []string) map[string][]string {
	strategies := make(map[string][]string)
	for _, ext := range extensions {
		strategy := extractStrategy(ext)
		strategies[strategy] = append(strategies[strategy], ext)
	}
	return strategies
}

func extractStrategy(ext string) string {
	switch {
	case strings.Contains(ext, "cursor"):
		return "Cursor pagination"
	case strings.Contains(ext, "offset"):
		return "Offset pagination"
	case strings.Contains(ext, "page"):
		return "Page pagination"
	case strings.Contains(ext, "checkpoint"):
		return "Checkpoint pagination"
	default:
		return "Other extensions"
	}
}

func printGroupedExtensions(strategies map[string][]string) {
	for strategy, extensions := range strategies {
		if len(extensions) > 0 {
			fmt.Printf("      %s🎯 %s:%s\n", colorPurple, strategy, colorReset)
			for _, ext := range extensions {
				printStrategyExtension(ext)
			}
		}
	}
}

func printStrategyExtension(ext string) {
	// Extract operation info from the extension string
	if strings.Contains(ext, "→") {
		fmt.Printf("        %s%s%s\n", colorGreen, ext, colorReset)
	} else {
		fmt.Printf("        %s→ %s%s\n", colorGreen, ext, colorReset)
	}
}

func printSkippedOperations(skippedOperations map[string][]string) {
	if len(skippedOperations) == 0 {
		return
	}

	totalSkipped := 0
	for _, skipped := range skippedOperations {
		totalSkipped += len(skipped)
	}

	if verbose {
		fmt.Printf("\n⏭️  %sSkipped Operations:%s %s%d%s\n", colorYellow, colorReset, colorBold, totalSkipped, colorReset)
		for file, operations := range skippedOperations {
			if len(operations) > 0 {
				printFileHeader(file)
				for _, op := range operations {
					printListItem(op, colorYellow)
				}
			}
		}
	} else {
		fmt.Printf("\n⏭️  %sSkipped Operations: %s%d%s (use --verbose for details)\n",
			colorYellow, colorBold, totalSkipped, colorReset)
	}
}

// Default values results printing
func printDefaultsResults(defaultsResult *transform.DefaultsResult) {
	if defaultsResult.Changed {
		printDefaultsHeader(defaultsResult)
		printAppliedDefaults(defaultsResult.AppliedDefaults)
		printSkippedTargets(defaultsResult.SkippedTargets)
		printSuccess("Default values added successfully")
	} else {
		printInfo("No default value changes needed")
	}
}

func printDefaultsHeader(defaultsResult *transform.DefaultsResult) {
	printHeader("Default Values Processing Results", "⚙️")
	fmt.Printf("📄 %sProcessed files:%s %s%d%s\n",
		colorCyan, colorReset, colorGreen, len(defaultsResult.ProcessedFiles), colorReset)
}

func printAppliedDefaults(appliedDefaults map[string][]string) {
	if len(appliedDefaults) == 0 {
		return
	}

	fmt.Printf("\n✅ %sApplied Defaults%s\n", colorGreen, colorReset)
	for file, defaults := range appliedDefaults {
		printFileHeader(file)
		for _, defaultInfo := range defaults {
			printListItem(defaultInfo, colorGreen)
		}
	}
}

func printSkippedTargets(skippedTargets map[string][]string) {
	if len(skippedTargets) == 0 {
		return
	}

	totalSkipped := 0
	for _, targets := range skippedTargets {
		totalSkipped += len(targets)
	}

	if verbose {
		fmt.Printf("\n⏭️  %sSkipped Targets:%s %s%d%s\n", colorYellow, colorReset, colorBold, totalSkipped, colorReset)
		for file, targets := range skippedTargets {
			if len(targets) > 0 {
				printFileHeader(file)
				for _, targetInfo := range targets {
					printListItem(targetInfo, colorYellow)
				}
			}
		}
	} else {
		fmt.Printf("\n⏭️  %sSkipped Targets: %s%d%s (use --verbose for details)\n",
			colorYellow, colorBold, totalSkipped, colorReset)
	}
}
