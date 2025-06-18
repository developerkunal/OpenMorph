package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/developerkunal/OpenMorph/internal/config"
	"github.com/developerkunal/OpenMorph/internal/transform"
	"github.com/developerkunal/OpenMorph/internal/tui"

	"github.com/spf13/cobra"
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

// version is set by GoReleaser at build time. Do not update manually.
var version = "dev"

// getVersion returns the current version, preferring build-time version,
// then falling back to .version file, then "dev"
func getVersion() string {
	// If version was set at build time (not "dev"), use it
	if version != "dev" {
		return version
	}

	// Try to read from .version file
	if content, err := os.ReadFile(".version"); err == nil {
		if v := strings.TrimSpace(string(content)); v != "" {
			return "v" + v
		}
	}

	// Fallback to "dev"
	return "dev"
}

var (
	inputDir   string
	configFile string
	inlineMaps []string

	dryRun                bool
	backup                bool
	exclude               []string
	validate              bool
	noConfig              bool
	interactive           bool
	paginationPriorityStr string
	flattenResponses      bool

	// Vendor extension flags
	vendorProviders []string
)

var rootCmd = &cobra.Command{
	Use:     "openmorph",
	Short:   "Transform OpenAPI vendor extension keys via mapping",
	Long:    `OpenMorph: Transform OpenAPI vendor extension keys in YAML/JSON files via mapping config or inline args.`,
	Version: getVersion(),
	Run: func(cmd *cobra.Command, _ []string) {
		if cmd.Flag("version") != nil && cmd.Flag("version").Changed {
			fmt.Println("OpenMorph version:", getVersion())
			return
		}
		cfg, err := config.LoadConfig(configFile, inlineMaps, inputDir, noConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Config error:", err)
			os.Exit(1)
		}
		// Merge CLI --exclude, --validate, --backup, and --flatten-responses with config
		if len(exclude) > 0 {
			cfg.Exclude = append(cfg.Exclude, exclude...)
		}
		if validate {
			cfg.Validate = true
		}
		if cmd.Flag("backup") != nil && cmd.Flag("backup").Changed {
			cfg.Backup = backup
		}
		if cmd.Flag("flatten-responses") != nil && cmd.Flag("flatten-responses").Changed {
			cfg.FlattenResponses = flattenResponses
		}
		if paginationPriorityStr != "" {
			// Parse comma-separated pagination priority
			priorities := strings.Split(paginationPriorityStr, ",")
			for i, p := range priorities {
				priorities[i] = strings.TrimSpace(p)
			}
			cfg.PaginationPriority = priorities
		}

		// Pretty-print config summary
		fmt.Printf("\n🔧 %sOpenMorph Configuration%s\n", colorBold, colorReset)
		fmt.Printf("═══════════════════════════════════\n")

		// Core settings
		fmt.Printf("📁 %sInput:%s         %s%s%s\n", colorCyan, colorReset, colorGreen, cfg.Input, colorReset)
		fmt.Printf("💾 %sBackup:%s        %s%v%s\n", colorCyan, colorReset, getStatusColor(cfg.Backup), cfg.Backup, colorReset)
		fmt.Printf("✅ %sValidate:%s      %s%v%s\n", colorCyan, colorReset, getStatusColor(cfg.Validate), cfg.Validate, colorReset)
		fmt.Printf("🔄 %sFlatten:%s       %s%v%s\n", colorCyan, colorReset, getStatusColor(cfg.FlattenResponses), cfg.FlattenResponses, colorReset)

		// Exclude list
		if len(cfg.Exclude) > 0 {
			fmt.Printf("🚫 %sExclude:%s       %s%v%s\n", colorCyan, colorReset, colorYellow, cfg.Exclude, colorReset)
		}

		// Pagination priority
		if len(cfg.PaginationPriority) > 0 {
			fmt.Printf("📊 %sPagination:%s    %s%v%s\n", colorCyan, colorReset, colorPurple, cfg.PaginationPriority, colorReset)
		}

		// Vendor extensions
		if cfg.VendorExtensions.Enabled {
			fmt.Printf("🏷️  %sVendor Ext:%s    %s%s%s\n", colorCyan, colorReset, colorGreen, "Enabled", colorReset)
			if len(vendorProviders) > 0 {
				fmt.Printf("   %sTarget:%s       %s%v%s\n", colorBlue, colorReset, colorGreen, vendorProviders, colorReset)
			} else {
				providerNames := make([]string, 0, len(cfg.VendorExtensions.Providers))
				for name := range cfg.VendorExtensions.Providers {
					providerNames = append(providerNames, name)
				}
				if len(providerNames) > 0 {
					fmt.Printf("   %sProviders:%s    %s%v%s\n", colorBlue, colorReset, colorGreen, providerNames, colorReset)
				}
			}
		}

		// Mappings
		if len(cfg.Mappings) > 0 {
			fmt.Printf("\n🔀 %sKey Mappings:%s\n", colorCyan, colorReset)
			for k, v := range cfg.Mappings {
				fmt.Printf("   %s%s%s %s→%s %s%s%s\n", colorYellow, k, colorReset, colorGreen, colorReset, colorBlue, v, colorReset)
			}
		}

		fmt.Printf("\n%s🚀 Starting transformation...%s\n", colorGreen, colorReset)

		// If interactive flag is set, launch TUI for preview/approval BEFORE any transformation
		if interactive {
			// Collect key changes for each file (but do not transform yet)
			inputFiles := []string{}
			_ = filepath.Walk(cfg.Input, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				if transform.IsYAML(path) || transform.IsJSON(path) {
					inputFiles = append(inputFiles, path)
				}
				return nil
			})
			fileKeyChanges := make(map[string][]transform.KeyChange)
			for _, f := range inputFiles {
				var changes []transform.KeyChange
				_, _ = transform.FileWithChanges(f, transform.Options{
					Mappings: cfg.Mappings,
					Exclude:  cfg.Exclude,
					DryRun:   true,
					Backup:   false,
				}, &changes)
				if len(changes) > 0 {
					fileKeyChanges[f] = changes
				}
			}
			var fileDiffs []tui.FileDiff
			// Generate a simple inline diff for each file (for TUI display)
			for _, f := range inputFiles {
				if len(fileKeyChanges[f]) > 0 {
					var diff strings.Builder
					for _, c := range fileKeyChanges[f] {
						line := "-"
						if c.Line > 0 {
							line = fmt.Sprintf("%d", c.Line)
						}
						diff.WriteString(fmt.Sprintf("- %s → + %s (line %s)\n", c.OldKey, c.NewKey, line))
					}
					fileDiffs = append(fileDiffs, tui.FileDiff{
						Path:       f,
						Diff:       diff.String(),
						Changed:    true,
						KeyChanges: fileKeyChanges[f],
					})
				}
			}
			if len(fileDiffs) == 0 {
				fmt.Println("\n\033[1;33mNo OpenAPI files required transformation.\033[0m")
				fmt.Println("Nothing to review. All files are up to date!")
				return
			}
			fmt.Printf("\033[1;36mInput file(s):\033[0m %s\n", cfg.Input)
			fmt.Printf("\033[1;36mFiles with changes: %d\033[0m\n", len(fileDiffs))
			fmt.Println("\033[1;36mLaunching interactive review...\033[0m")
			accepted, skipped, err := tui.RunTUI(fileDiffs)
			if err != nil {
				fmt.Fprintln(os.Stderr, "TUI error:", err)
				os.Exit(4)
			}
			// Only transform accepted files
			var actuallyChanged []string
			for _, f := range inputFiles {
				if accepted[f] {
					ok, err := transform.File(f, transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   false,
						Backup:   cfg.Backup,
					})
					if err != nil {
						fmt.Fprintf(os.Stderr, "Transform error for %s: %v\n", f, err)
					} else if ok {
						actuallyChanged = append(actuallyChanged, f)
						// If backup is requested, ensure backup file is created
						if cfg.Backup {
							if _, err := os.Stat(f + ".bak"); err != nil {
								fmt.Fprintf(os.Stderr, "[WARNING] Backup file not found for %s after transform.\n", f)
							}
						}
					}
				}
			}
			// Print a user-friendly summary of accepted/skipped/transformed files
			fmt.Printf("\n\033[1;32mAccepted files:\033[0m ")
			if len(accepted) == 0 {
				fmt.Print("(none)")
			} else {
				first := true
				for f := range accepted {
					if !first {
						fmt.Print(", ")
					}
					fmt.Print(f)
					first = false
				}
			}
			fmt.Println()
			fmt.Printf("\033[1;33mSkipped files:\033[0m ")
			if len(skipped) == 0 {
				fmt.Print("(none)")
			} else {
				first := true
				for f := range skipped {
					if !first {
						fmt.Print(", ")
					}
					fmt.Print(f)
					first = false
				}
			}
			fmt.Println()
			if len(actuallyChanged) == 0 {
				fmt.Printf("ℹ️  %sNo files were transformed%s\n", colorYellow, colorReset)
			} else {
				fmt.Printf("✅ %sTransformed files:%s %s%v%s\n", colorGreen, colorReset, colorBold, actuallyChanged, colorReset)
			}

			// Process pagination if priority is configured (for interactive mode)
			if len(cfg.PaginationPriority) > 0 && len(actuallyChanged) > 0 {
				fmt.Printf("\n🔄 %sProcessing pagination with priority:%s %s%v%s\n",
					colorCyan, colorReset, colorPurple, cfg.PaginationPriority, colorReset)
				paginationOpts := transform.PaginationOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   false,
						Backup:   cfg.Backup,
					},
					PaginationPriority: cfg.PaginationPriority,
				}
				paginationResult, err := transform.ProcessPaginationInDir(cfg.Input, paginationOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Pagination processing error:", err)
					os.Exit(2)
				}

				printPaginationResults(paginationResult)
			}

			// Process response flattening if configured (for interactive mode)
			if cfg.FlattenResponses && len(actuallyChanged) > 0 {
				fmt.Printf("\033[1;36mProcessing response flattening...\033[0m\n")
				flattenOpts := transform.FlattenOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   false,
						Backup:   cfg.Backup,
					},
					FlattenResponses: cfg.FlattenResponses,
				}
				flattenResult, err := transform.ProcessFlatteningInDir(cfg.Input, flattenOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Response flattening error:", err)
					os.Exit(2)
				}

				printFlattenResults(flattenResult)
			}

			// Process vendor extensions if configured (for interactive mode)
			if cfg.VendorExtensions.Enabled && len(actuallyChanged) > 0 {
				fmt.Printf("\n🏷️  %sProcessing vendor extensions...%s\n", colorCyan, colorReset)
				vendorOpts := transform.VendorExtensionOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   false,
						Backup:   cfg.Backup,
					},
					VendorExtensions: cfg.VendorExtensions,
					EnabledProviders: vendorProviders,
				}
				vendorResult, err := transform.ProcessVendorExtensionsInDir(cfg.Input, vendorOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Vendor extensions processing error:", err)
					os.Exit(2)
				}

				printVendorExtensionResults(vendorResult)
			}

			// Run validation if requested (for interactive mode)
			if cfg.Validate {
				fmt.Printf("\n🔍 %sValidating OpenAPI specifications...%s\n", colorCyan, colorReset)
				if err := runSwaggerValidate(cfg.Input); err != nil {
					fmt.Fprintf(os.Stderr, "%s❌ Validation failed:%s %v\n", colorRed, colorReset, err)
					os.Exit(3)
				}
				fmt.Printf("%s✅ Validation passed successfully%s\n", colorGreen, colorReset)
			}
			return
		}

		// Non-interactive path: Call the transformer
		opts := transform.Options{
			Mappings: cfg.Mappings,
			Exclude:  cfg.Exclude,
			DryRun:   dryRun,
			Backup:   cfg.Backup, // use config value (merged with CLI)
		}
		changed, err := transform.Dir(cfg.Input, opts)
		fmt.Printf("Files detected for transform: %v\n", changed)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Transform error:", err)
			os.Exit(2)
		}
		fmt.Printf("Transformed files: %v\n", changed)

		// In dry-run mode, show what would be changed for pagination and flattening
		if dryRun {
			fmt.Printf("\033[1;33m╭─────────────────────────────────────────────────────────────╮\033[0m\n")
			fmt.Printf("\033[1;33m│                    DRY-RUN PREVIEW MODE                     │\033[0m\n")
			fmt.Printf("\033[1;33m╰─────────────────────────────────────────────────────────────╯\033[0m\n")
			fmt.Printf("\033[1;31m⚠️  IMPORTANT: Dry-run shows INDEPENDENT previews of each step.\033[0m\n")
			fmt.Printf("\033[1;31m   In actual execution, steps are CUMULATIVE (each builds on the previous).\033[0m\n")
			fmt.Printf("\033[1;31m   Flattening results will differ significantly in real execution!\033[0m\n\n")

			if len(cfg.PaginationPriority) > 0 {
				fmt.Printf("\033[1;36m[STEP 1] Pagination changes with priority: %v\033[0m\n", cfg.PaginationPriority)
				dryRunPaginationOpts := transform.PaginationOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   true, // Force dry-run for preview
						Backup:   cfg.Backup,
					},
					PaginationPriority: cfg.PaginationPriority,
				}
				paginationResult, err := transform.ProcessPaginationInDir(cfg.Input, dryRunPaginationOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Pagination dry-run error:", err)
				} else {
					printPaginationResults(paginationResult)
				}
				fmt.Println()
			}
			if cfg.VendorExtensions.Enabled {
				fmt.Printf("\033[1;36m[STEP 2] Vendor extensions changes\033[0m\n")
				dryRunVendorOpts := transform.VendorExtensionOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   true, // Force dry-run for preview
						Backup:   cfg.Backup,
					},
					VendorExtensions: cfg.VendorExtensions,
					EnabledProviders: vendorProviders,
				}
				vendorResult, err := transform.ProcessVendorExtensionsInDir(cfg.Input, dryRunVendorOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Vendor extensions dry-run error:", err)
				} else {
					printVendorExtensionResults(vendorResult)
				}
				fmt.Println()
			}
			if cfg.FlattenResponses {
				stepNum := 2
				if cfg.VendorExtensions.Enabled {
					stepNum = 3
				}
				fmt.Printf("\033[1;36m[STEP %d] Response flattening changes\033[0m\n", stepNum)
				fmt.Printf("\033[1;31m⚠️  CRITICAL: This preview operates on the ORIGINAL file.\033[0m\n")
				fmt.Printf("\033[1;31m   Real execution will show SIGNIFICANTLY MORE changes\033[0m\n")
				fmt.Printf("\033[1;31m   because pagination creates new schemas to flatten!\033[0m\n")
				dryRunFlattenOpts := transform.FlattenOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   true, // Force dry-run for preview
						Backup:   cfg.Backup,
					},
					FlattenResponses: cfg.FlattenResponses,
				}
				flattenResult, err := transform.ProcessFlatteningInDir(cfg.Input, dryRunFlattenOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Response flattening dry-run error:", err)
				} else {
					printFlattenResults(flattenResult)
				}
				fmt.Println()
			}
			if cfg.Validate {
				stepNum := 3
				if cfg.VendorExtensions.Enabled {
					stepNum = 4
				}
				fmt.Printf("\033[1;36m[STEP %d] Validation\033[0m\n", stepNum)
				fmt.Printf("\033[1;33m⏭️  Skipping validation in dry-run mode\033[0m\n\n")
			}

			fmt.Printf("\033[1;36m╭─────────────────────────────────────────────────────────────╮\033[0m\n")
			fmt.Printf("\033[1;36m│ 💡 TIP: Use --interactive mode to see exact cumulative     │\033[0m\n")
			fmt.Printf("\033[1;36m│    effects of all transformations applied sequentially.    │\033[0m\n")
			fmt.Printf("\033[1;36m╰─────────────────────────────────────────────────────────────╯\033[0m\n")

			fmt.Printf("\n\033[1;33m📊 DRY-RUN SUMMARY:\033[0m\n")
			fmt.Printf("   • Mapping changes: Applied to original file\n")
			if len(cfg.PaginationPriority) > 0 {
				fmt.Printf("   • Pagination changes: Based on original file state\n")
			}
			if cfg.VendorExtensions.Enabled {
				fmt.Printf("   • Vendor extension changes: Applied after pagination cleanup\n")
			}
			if cfg.FlattenResponses {
				fmt.Printf("   • Flattening changes: Based on original file (will be much more extensive in real execution)\n")
			}
			fmt.Printf("\n\033[1;32m✅ For accurate cumulative results, use:\033[0m\n")
			fmt.Printf("   • \033[1;36m--interactive\033[0m mode for step-by-step review\n")
			fmt.Printf("   • Run without \033[1;36m--dry-run\033[0m on a backup/test file\n")
		}

		// Process pagination if priority is configured (skip in dry-run mode)
		if len(cfg.PaginationPriority) > 0 && !dryRun {
			fmt.Printf("\033[1;36mProcessing pagination with priority: %v\033[0m\n", cfg.PaginationPriority)
			paginationOpts := transform.PaginationOptions{
				Options:            opts,
				PaginationPriority: cfg.PaginationPriority,
			}
			paginationResult, err := transform.ProcessPaginationInDir(cfg.Input, paginationOpts)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Pagination processing error:", err)
				os.Exit(2)
			}

			printPaginationResults(paginationResult)
		}

		// Process response flattening if configured (skip in dry-run mode)
		if cfg.FlattenResponses && !dryRun {
			fmt.Printf("\033[1;36mProcessing response flattening...\033[0m\n")
			flattenOpts := transform.FlattenOptions{
				Options:          opts,
				FlattenResponses: cfg.FlattenResponses,
			}
			flattenResult, err := transform.ProcessFlatteningInDir(cfg.Input, flattenOpts)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Response flattening error:", err)
				os.Exit(2)
			}

			printFlattenResults(flattenResult)
		}

		// Process vendor extensions if configured (skip in dry-run mode)
		if cfg.VendorExtensions.Enabled && !dryRun {
			fmt.Printf("\n🏷️  %sProcessing vendor extensions...%s\n", colorCyan, colorReset)
			vendorOpts := transform.VendorExtensionOptions{
				Options:          opts,
				VendorExtensions: cfg.VendorExtensions,
				EnabledProviders: vendorProviders,
			}
			vendorResult, err := transform.ProcessVendorExtensionsInDir(cfg.Input, vendorOpts)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Vendor extensions processing error:", err)
				os.Exit(2)
			}

			printVendorExtensionResults(vendorResult)
		}

		// Run validation if requested
		if cfg.Validate && !dryRun {
			fmt.Printf("\n🔍 %sValidating OpenAPI specifications...%s\n", colorCyan, colorReset)
			if err := runSwaggerValidate(cfg.Input); err != nil {
				fmt.Fprintf(os.Stderr, "%s❌ Validation failed:%s %v\n", colorRed, colorReset, err)
				os.Exit(3)
			}
			fmt.Printf("%s✅ Validation passed successfully%s\n", colorGreen, colorReset)
		}

		// Final completion message
		fmt.Printf("\n%s🎉 OpenMorph transformation completed successfully!%s\n", colorGreen, colorReset)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&inputDir, "input", "i", "", "Directory containing OpenAPI specs (YAML/JSON)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Mapping config file (.yaml or .json)")
	rootCmd.PersistentFlags().StringArrayVar(&inlineMaps, "map", nil, "Inline key mappings (from=to), repeatable")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would change without writing files (Note: multi-step transformations shown independently, use --interactive for cumulative preview)")
	rootCmd.PersistentFlags().BoolVar(&backup, "backup", false, "Save a .bak copy before overwriting")
	rootCmd.PersistentFlags().StringArrayVar(&exclude, "exclude", nil, "Keys to exclude from transformation (repeatable)")
	rootCmd.PersistentFlags().BoolVar(&validate, "validate", false, "Run swagger-cli validate after transforming")
	rootCmd.PersistentFlags().BoolVar(&interactive, "interactive", false, "Launch a TUI for interactive preview and approval")
	rootCmd.PersistentFlags().BoolVar(&noConfig, "no-config", false, "Ignore all config files and use only CLI flags")
	rootCmd.PersistentFlags().StringVar(&paginationPriorityStr, "pagination-priority", "", "Pagination strategy priority order (e.g., checkpoint,offset,page,cursor,none)")
	rootCmd.PersistentFlags().BoolVar(&flattenResponses, "flatten-responses", false, "Flatten oneOf/anyOf/allOf with single $ref after pagination processing")

	// Vendor extension flags
	rootCmd.PersistentFlags().StringArrayVar(&vendorProviders, "vendor-providers", nil, "Specific vendor providers to apply (e.g., fern,speakeasy). If empty, applies all configured providers")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runSwaggerValidate shells out to swagger-cli validate for all YAML/JSON files in the input dir
func runSwaggerValidate(inputDir string) error {
	files := []string{}
	err := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if transform.IsYAML(path) || transform.IsJSON(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, f := range files {
		cmd := fmt.Sprintf("swagger-cli validate %q", f)
		fmt.Printf("   %s🔍 Validating:%s %s\n", colorBlue, colorReset, f)

		if code := runShellSilent(cmd); code != 0 {
			return fmt.Errorf("swagger-cli validate failed for %s", f)
		}
		fmt.Printf("   %s✅ %s is valid%s\n", colorGreen, f, colorReset)
	}
	return nil
}

// runShellSilent runs a shell command silently and returns the exit code
func runShellSilent(cmd string) int {
	c := os.Getenv("SHELL")
	if c == "" {
		c = "/bin/sh"
	}
	proc := execCommand(c, "-c", cmd)
	// Don't pipe stdout/stderr to avoid duplicate output
	if err := proc.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

// getStatusColor returns green for true, red for false
func getStatusColor(status bool) string {
	if status {
		return colorGreen
	}
	return colorRed
}

// printPaginationResults prints the pagination processing results
func printPaginationResults(paginationResult *transform.PaginationResult) {
	if paginationResult.Changed {
		fmt.Printf("\n🔄 %sPagination Processing Results%s\n", colorBold, colorReset)
		fmt.Printf("═══════════════════════════════════════\n")

		// Summary stats
		fmt.Printf("📄 %sProcessed files:%s %s%d%s\n",
			colorCyan, colorReset, colorGreen, len(paginationResult.ProcessedFiles), colorReset)

		// Removed parameters section
		if len(paginationResult.RemovedParams) > 0 {
			fmt.Printf("\n🗑️  %sRemoved Parameters%s\n", colorYellow, colorReset)
			for endpoint, params := range paginationResult.RemovedParams {
				fmt.Printf("   %s• %s%s%s\n", colorYellow, colorBold, endpoint, colorReset)
				for _, param := range params {
					fmt.Printf("     %s- %s%s\n", colorRed, param, colorReset)
				}
			}
		}

		// Removed responses section
		if len(paginationResult.RemovedResponses) > 0 {
			fmt.Printf("\n📋 %sRemoved Response Codes%s\n", colorYellow, colorReset)
			for endpoint, responses := range paginationResult.RemovedResponses {
				fmt.Printf("   %s• %s%s%s\n", colorYellow, colorBold, endpoint, colorReset)
				for _, response := range responses {
					fmt.Printf("     %s- %s%s\n", colorRed, response, colorReset)
				}
			}
		}

		// Cleanup section
		if len(paginationResult.UnusedComponents) > 0 {
			fmt.Printf("\n🧹 %sRemoved Unused Components%s\n", colorPurple, colorReset)
			for _, component := range paginationResult.UnusedComponents {
				fmt.Printf("   %s- %s%s\n", colorRed, component, colorReset)
			}
		}

		fmt.Printf("\n%s✅ Pagination cleanup completed successfully%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("⏭️  %sNo pagination changes needed%s\n", colorYellow, colorReset)
	}
}

func printFlattenResults(flattenResult *transform.FlattenResult) {
	if flattenResult == nil {
		fmt.Printf("  %sNo flattening result to display%s\n", colorRed, colorReset)
		return
	}

	fmt.Println("🛠️  Processing response flattening...")

	if !flattenResult.Changed {
		fmt.Printf("  %sNo response flattening changes needed.%s\n", colorYellow, colorReset)
		return
	}

	fmt.Printf("%s✅ Response flattening completed%s\n", colorGreen, colorReset)
	fmt.Printf("  📄 Processed files: %s%d%s\n", colorGreen, len(flattenResult.ProcessedFiles), colorReset)

	for file, refs := range flattenResult.FlattenedRefs {
		fmt.Printf("\n🔍 Flattened references in: %s%s%s\n", colorBold, file, colorReset)

		var oneOfs, anyOfs, allOfs, remaps []string
		for _, ref := range refs {
			switch {
			case strings.Contains(ref, "oneOf"):
				oneOfs = append(oneOfs, ref)
			case strings.Contains(ref, "anyOf"):
				anyOfs = append(anyOfs, ref)
			case strings.Contains(ref, "allOf"):
				allOfs = append(allOfs, ref)
			default:
				remaps = append(remaps, ref)
			}
		}

		printCategory := func(label string, color string, items []string) {
			if len(items) == 0 {
				return
			}
			fmt.Printf("  ── %s%s%s:\n", color, label, colorReset)
			for _, item := range items {
				parts := strings.SplitN(item, "->", 2)
				if len(parts) == 2 {
					fmt.Printf("      - %s%s%s\n        %s→%s %s\n",
						colorBold, strings.TrimSpace(parts[0]), colorReset,
						colorGreen, colorReset, strings.TrimSpace(parts[1]),
					)
				} else {
					fmt.Printf("      - %s\n", item)
				}
			}
		}

		printCategory("oneOf replacements", colorYellow, oneOfs)
		printCategory("anyOf replacements", colorCyan, anyOfs)
		printCategory("allOf replacements", colorPurple, allOfs)
		printCategory("$ref remappings", colorBlue, remaps)
	}
}

// printVendorExtensionResults prints the vendor extension processing results
func printVendorExtensionResults(vendorResult *transform.VendorExtensionResult) {
	if vendorResult.Changed {
		printVendorExtensionHeader(vendorResult)
		printAddedExtensions(vendorResult.AddedExtensions)
		printSkippedOperations(vendorResult.SkippedOperations)
		fmt.Printf("\n%s✅ Vendor extensions added successfully%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("⏭️  %sNo vendor extension changes needed%s\n", colorYellow, colorReset)
	}
}

// printVendorExtensionHeader prints the header and summary stats
func printVendorExtensionHeader(vendorResult *transform.VendorExtensionResult) {
	fmt.Printf("\n🏷️  %sVendor Extension Processing Results%s\n", colorBold, colorReset)
	fmt.Printf("═══════════════════════════════════════════\n")
	fmt.Printf("📄 %sProcessed files:%s %s%d%s\n",
		colorCyan, colorReset, colorGreen, len(vendorResult.ProcessedFiles), colorReset)
}

// printAddedExtensions prints the added extensions section
func printAddedExtensions(addedExtensions map[string][]string) {
	if len(addedExtensions) == 0 {
		return
	}

	fmt.Printf("\n✅ %sAdded Extensions%s\n", colorGreen, colorReset)
	for file, extensions := range addedExtensions {
		fmt.Printf("   %s📁 %s%s%s\n", colorBlue, colorBold, file, colorReset)
		strategies := groupExtensionsByStrategy(extensions)
		printGroupedExtensions(strategies)
	}
}

// groupExtensionsByStrategy groups extensions by their strategy type
func groupExtensionsByStrategy(extensions []string) map[string][]string {
	strategies := make(map[string][]string)
	for _, ext := range extensions {
		strategy := extractStrategy(ext)
		strategies[strategy] = append(strategies[strategy], ext)
	}
	return strategies
}

// extractStrategy extracts the strategy from an extension string
func extractStrategy(ext string) string {
	if strings.Contains(ext, "(") && strings.Contains(ext, " strategy)") {
		start := strings.LastIndex(ext, "(") + 1
		end := strings.LastIndex(ext, " strategy)")
		if start > 0 && end > start {
			return ext[start:end]
		}
	}
	return "other"
}

// printGroupedExtensions prints extensions grouped by strategy
func printGroupedExtensions(strategies map[string][]string) {
	for strategy, exts := range strategies {
		if strategy != "other" {
			fmt.Printf("      %s🎯 %s pagination:%s\n", colorPurple, strings.ToUpper(strategy[:1])+strategy[1:], colorReset)
		}
		for _, ext := range exts {
			if strategy == "other" {
				fmt.Printf("      %s• %s%s\n", colorGreen, ext, colorReset)
			} else {
				printStrategyExtension(ext)
			}
		}
	}
}

// printStrategyExtension prints a single strategy extension
func printStrategyExtension(ext string) {
	parts := strings.Split(ext, ":")
	if len(parts) > 0 {
		endpoint := strings.TrimSpace(parts[0])
		fmt.Printf("        %s→ %s%s\n", colorGreen, endpoint, colorReset)
	}
}

// printSkippedOperations prints the skipped operations section
func printSkippedOperations(skippedOperations map[string][]string) {
	if len(skippedOperations) == 0 {
		return
	}

	totalSkipped := 0
	for _, skipped := range skippedOperations {
		totalSkipped += len(skipped)
	}
	fmt.Printf("\n⏭️  %sSkipped Operations: %s%d%s (use --verbose for details)\n",
		colorYellow, colorBold, totalSkipped, colorReset)
}

// execCommand is a wrapper for exec.Command (for testability)
var execCommand = exec.Command
