package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/developerkunal/OpenMorph/internal/config"
	"github.com/developerkunal/OpenMorph/internal/transform"
	"github.com/developerkunal/OpenMorph/internal/tui"

	"github.com/spf13/cobra"
)

var (
	inputDir   string
	configFile string
	inlineMaps []string
	outputFile string

	dryRun                bool
	backup                bool
	exclude               []string
	validate              bool
	noConfig              bool
	interactive           bool
	paginationPriorityStr string
	flattenResponses      bool
	verbose               bool

	// Vendor extension flags
	vendorProviders []string

	// Default values flags
	setDefaults bool
)

var rootCmd = &cobra.Command{
	Use:     "openmorph [flags]",
	Short:   "Transform OpenAPI vendor extension keys via mapping",
	Long:    `OpenMorph: Transform OpenAPI vendor extension keys in YAML/JSON files via mapping config or inline args. Features vendor extensions, default values, response flattening, and more.`,
	Version: GetVersion(),
	Run: func(cmd *cobra.Command, _ []string) {
		if cmd.Flag("version") != nil && cmd.Flag("version").Changed {
			fmt.Println("OpenMorph version:", GetVersion())
			return
		}
		cfg, err := config.LoadConfig(configFile, inlineMaps, inputDir, outputFile, noConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Config error:", err)
			os.Exit(1)
		}

		var actualInputPath string
		if inputDir != "" {
			actualInputPath = inputDir
		} else if cfg.Input != "" {
			actualInputPath = cfg.Input
		} else {
			fmt.Fprintln(os.Stderr, "Error: No input path specified.")
			fmt.Fprintln(os.Stderr, "Provide input via:")
			fmt.Fprintln(os.Stderr, "  • --input flag: openmorph --input <path>")
			fmt.Fprintln(os.Stderr, "  • Config file with 'input: <path>'")
			fmt.Fprintln(os.Stderr, "  • .openapirc.yaml with 'input: <path>'")
			os.Exit(1)
		}

		// Use output from config if not provided via CLI
		var actualOutputFile string
		if outputFile != "" {
			actualOutputFile = outputFile
		} else if cfg.Output != "" {
			actualOutputFile = cfg.Output
		}

		// Validate output file usage
		if actualOutputFile != "" {
			// When output file is specified, input must be a single file, not a directory
			info, err := os.Stat(actualInputPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking input path: %v\n", err)
				os.Exit(1)
			}
			if info.IsDir() {
				fmt.Fprintln(os.Stderr, "Error: --output flag can only be used with a single input file, not a directory")
				fmt.Fprintln(os.Stderr, "Use --input to specify a single OpenAPI file when using --output")
				os.Exit(1)
			}
			if interactive {
				fmt.Fprintln(os.Stderr, "Error: --output flag cannot be used with --interactive mode")
				os.Exit(1)
			}
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
		if cmd.Flag("set-defaults") != nil && cmd.Flag("set-defaults").Changed {
			cfg.DefaultValues.Enabled = setDefaults
		}
		if paginationPriorityStr != "" {
			// Parse comma-separated pagination priority
			priorities := strings.Split(paginationPriorityStr, ",")
			for i, p := range priorities {
				priorities[i] = strings.TrimSpace(p)
			}
			cfg.PaginationPriority = priorities
		}

		// Print config summary
		printConfigSummary(cfg, vendorProviders, actualOutputFile)

		// If interactive flag is set, launch TUI for preview/approval BEFORE any transformation
		if interactive {
			// Collect key changes for each file (but do not transform yet)
			inputFiles := []string{}
			_ = filepath.Walk(actualInputPath, func(path string, info os.FileInfo, err error) error {
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

			// Process remaining transformations using unified pipeline (for interactive mode)
			if len(actuallyChanged) > 0 {
				fmt.Printf("\n🔄 %sProcessing additional transformations...%s\n", colorCyan, colorReset)

				// Use unified pipeline for remaining transformations
				pipeline := transform.NewTransformationPipeline(cfg, vendorProviders, false, cfg.Backup, "")
				results, err := pipeline.ExecuteFullPipeline(cfg.Input)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Additional transformations error:", err)
					os.Exit(2)
				}

				// Print results for each transformation step
				if results.PaginationResult != nil {
					printPaginationResults(results.PaginationResult)
				}
				if results.FlattenResult != nil {
					printFlattenResultsImproved(results.FlattenResult)
				}
				if results.VendorResult != nil {
					printVendorExtensionResults(results.VendorResult)
				}
				if results.DefaultsResult != nil {
					printDefaultsResults(results.DefaultsResult)
				}
			}

			// Run validation if requested (for interactive mode)
			if cfg.Validate {
				fmt.Printf("\n🔍 %sValidating OpenAPI specifications...%s\n", colorCyan, colorReset)
				if err := RunSwaggerValidate(cfg.Input); err != nil {
					fmt.Fprintf(os.Stderr, "%s❌ Validation failed:%s %v\n", colorRed, colorReset, err)
					os.Exit(3)
				}
				fmt.Printf("%s✅ Validation passed successfully%s\n", colorGreen, colorReset)
			}
			return
		}

		// Non-interactive path: Use unified transformation pipeline

		// In dry-run mode, skip the first execution and go directly to detailed preview
		if dryRun {
			fmt.Printf("\033[1;33m╭─────────────────────────────────────────────────────────────╮\033[0m\n")
			fmt.Printf("\033[1;33m│                    DRY-RUN PREVIEW MODE                     │\033[0m\n")
			fmt.Printf("\033[1;33m╰─────────────────────────────────────────────────────────────╯\033[0m\n")
			fmt.Printf("\033[1;31m⚠️  IMPORTANT: Dry-run shows INDEPENDENT previews of each step.\033[0m\n")
			fmt.Printf("\033[1;31m   In actual execution, steps are CUMULATIVE (each builds on the previous).\033[0m\n")
			fmt.Printf("\033[1;31m   Flattening results will differ significantly in real execution!\033[0m\n\n")

			// Use unified pipeline for dry-run preview
			dryRunPipeline := transform.NewTransformationPipeline(cfg, vendorProviders, true, cfg.Backup, "")
			dryRunResults, err := dryRunPipeline.ExecuteFullPipeline(actualInputPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Dry-run preview error:", err)
				os.Exit(2)
			}

			// Print results for each transformation step
			if dryRunResults.PaginationResult != nil {
				fmt.Printf("\033[1;36m[STEP 1] Pagination changes with priority: %v\033[0m\n", cfg.PaginationPriority)
				printPaginationResults(dryRunResults.PaginationResult)
				fmt.Println()
			}
			if dryRunResults.VendorResult != nil {
				fmt.Printf("\033[1;36m[STEP 2] Vendor extensions changes\033[0m\n")
				printVendorExtensionResults(dryRunResults.VendorResult)
				fmt.Println()
			}
			if dryRunResults.DefaultsResult != nil {
				stepNum := 2
				if cfg.VendorExtensions.Enabled {
					stepNum = 3
				}
				fmt.Printf("\033[1;36m[STEP %d] Default values changes\033[0m\n", stepNum)
				printDefaultsResults(dryRunResults.DefaultsResult)
				fmt.Println()
			}
			if dryRunResults.FlattenResult != nil {
				stepNum := 2
				if cfg.VendorExtensions.Enabled {
					stepNum = 3
				}
				if cfg.DefaultValues.Enabled {
					stepNum = 4
				}
				fmt.Printf("\033[1;36m[STEP %d] Response flattening changes\033[0m\n", stepNum)
				fmt.Printf("\033[1;31m⚠️  CRITICAL: This preview operates on the ORIGINAL file.\033[0m\n")
				fmt.Printf("\033[1;31m   Real execution will show SIGNIFICANTLY MORE changes\033[0m\n")
				fmt.Printf("\033[1;31m   because pagination creates new schemas to flatten!\033[0m\n")
				printFlattenResultsImproved(dryRunResults.FlattenResult)
				fmt.Println()
			}

			fmt.Printf("\033[1;36m[STEP %d] Validation\033[0m\n", 5)
			fmt.Printf("⏭️  %sSkipping validation in dry-run mode%s\n", colorYellow, colorReset)
			fmt.Println()

			fmt.Printf("\033[1;33m╭─────────────────────────────────────────────────────────────╮\033[0m\n")
			fmt.Printf("\033[1;33m│ 💡 TIP: Use --interactive mode to see exact cumulative     │\033[0m\n")
			fmt.Printf("\033[1;33m│    effects of all transformations applied sequentially.    │\033[0m\n")
			fmt.Printf("\033[1;33m╰─────────────────────────────────────────────────────────────╯\033[0m\n")
			fmt.Println()

			fmt.Printf("\033[1;36m📊 DRY-RUN SUMMARY:\033[0m\n")
			fmt.Printf("   • Mapping changes: Applied to original file\n")
			fmt.Printf("   • Pagination changes: Based on original file state\n")
			fmt.Printf("   • Vendor extension changes: Applied after pagination cleanup\n")
			fmt.Printf("   • Flattening changes: Based on original file (will be much more extensive in real execution)\n")
			fmt.Println()
			fmt.Printf("\033[1;32m✅ For accurate cumulative results, use:\033[0m\n")
			fmt.Printf("   • --interactive mode for step-by-step review\n")
			fmt.Printf("   • Run without --dry-run on a backup/test file\n")
			fmt.Println()
			printSuccess("OpenMorph transformation completed successfully!")
			return
		}

		pipeline := transform.NewTransformationPipeline(cfg, vendorProviders, false, cfg.Backup, actualOutputFile)

		if actualOutputFile != "" {
			fmt.Printf("Input file: %s\n", actualInputPath)
			fmt.Printf("Output file: %s\n", actualOutputFile)
		}

		results, transformErr := pipeline.ExecuteFullPipeline(actualInputPath)
		if transformErr != nil {
			fmt.Fprintln(os.Stderr, "Transform error:", transformErr)
			os.Exit(2)
		}

		if actualOutputFile != "" {
			if len(results.Changed) > 0 {
				fmt.Printf("✅ %sTransformation completed successfully%s\n", colorGreen, colorReset)

				// Display detailed transformation results for single file output (same as directory mode)
				if !dryRun {
					// Print detailed results for each transformation step using the same functions as directory mode
					if results.PaginationResult != nil {
						printPaginationResults(results.PaginationResult)
					}
					if results.FlattenResult != nil {
						printFlattenResultsImproved(results.FlattenResult)
					}
					if results.VendorResult != nil {
						printVendorExtensionResults(results.VendorResult)
					}
					if results.DefaultsResult != nil {
						printDefaultsResults(results.DefaultsResult)
					}
				}
			} else {
				fmt.Printf("ℹ️  %sNo transformations needed%s\n", colorYellow, colorReset)
			}
		} else {
			fmt.Printf("Files detected for transform: %v\n", results.Changed)
			fmt.Printf("Transformed files: %v\n", results.Changed)

			// Print results for directory processing
			if results.PaginationResult != nil {
				printPaginationResults(results.PaginationResult)
			}
			if results.FlattenResult != nil {
				printFlattenResultsImproved(results.FlattenResult)
			}
			if results.VendorResult != nil {
				printVendorExtensionResults(results.VendorResult)
			}
			if results.DefaultsResult != nil {
				printDefaultsResults(results.DefaultsResult)
			}
		}

		// Run validation if requested
		if cfg.Validate && !dryRun {
			fmt.Printf("\n🔍 %sValidating OpenAPI specifications...%s\n", colorCyan, colorReset)
			var validationPath string
			if actualOutputFile != "" {
				validationPath = actualOutputFile
			} else {
				validationPath = actualInputPath
			}
			if validationErr := RunSwaggerValidate(validationPath); validationErr != nil {
				fmt.Fprintf(os.Stderr, "%s❌ Validation failed:%s %v\n", colorRed, colorReset, validationErr)
				os.Exit(3)
			}
			fmt.Printf("%s✅ Validation passed successfully%s\n", colorGreen, colorReset)
		}

		// Final completion message
		fmt.Printf("\n%s🎉 OpenMorph transformation completed successfully!%s\n", colorGreen, colorReset)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&inputDir, "input", "i", "", "Directory containing OpenAPI specs (optional - can be specified in config file)")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "Output file path (optional - if not provided, files are modified in place)")
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output including skipped targets and operations")

	// Vendor extension flags
	rootCmd.PersistentFlags().StringArrayVar(&vendorProviders, "vendor-providers", nil, "Specific vendor providers to apply (e.g., fern,speakeasy). If empty, applies all configured providers")

	// Default values flags
	rootCmd.PersistentFlags().BoolVar(&setDefaults, "set-defaults", false, "Enable default value setting (requires configuration via config file)")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
