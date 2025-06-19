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
	Use:     "openmorph",
	Short:   "Transform OpenAPI vendor extension keys via mapping",
	Long:    `OpenMorph: Transform OpenAPI vendor extension keys in YAML/JSON files via mapping config or inline args. Features vendor extensions, default values, response flattening, and more.`,
	Version: GetVersion(),
	Run: func(cmd *cobra.Command, _ []string) {
		if cmd.Flag("version") != nil && cmd.Flag("version").Changed {
			fmt.Println("OpenMorph version:", GetVersion())
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
		printConfigSummary(cfg, vendorProviders)

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

				printFlattenResultsImproved(flattenResult)
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

			// Process default values if configured (for interactive mode)
			if cfg.DefaultValues.Enabled && len(actuallyChanged) > 0 {
				fmt.Printf("\n⚙️  %sProcessing default values...%s\n", colorCyan, colorReset)
				defaultsOpts := transform.DefaultsOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   false,
						Backup:   cfg.Backup,
					},
					DefaultValues: cfg.DefaultValues,
				}
				defaultsResult, err := transform.ProcessDefaultsInDir(cfg.Input, defaultsOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Default values processing error:", err)
					os.Exit(2)
				}

				printDefaultsResults(defaultsResult)
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
			if cfg.DefaultValues.Enabled {
				stepNum := 2
				if cfg.VendorExtensions.Enabled {
					stepNum = 3
				}
				fmt.Printf("\033[1;36m[STEP %d] Default values changes\033[0m\n", stepNum)
				dryRunDefaultsOpts := transform.DefaultsOptions{
					Options: transform.Options{
						Mappings: cfg.Mappings,
						Exclude:  cfg.Exclude,
						DryRun:   true, // Force dry-run for preview
						Backup:   cfg.Backup,
					},
					DefaultValues: cfg.DefaultValues,
				}
				defaultsResult, err := transform.ProcessDefaultsInDir(cfg.Input, dryRunDefaultsOpts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Default values dry-run error:", err)
				} else {
					printDefaultsResults(defaultsResult)
				}
				fmt.Println()
			}
			if cfg.FlattenResponses {
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
					printFlattenResultsImproved(flattenResult)
				}
				fmt.Println()
			}
			if cfg.Validate {
				stepNum := 3
				if cfg.VendorExtensions.Enabled {
					stepNum = 4
				}
				if cfg.DefaultValues.Enabled {
					stepNum++
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

			printFlattenResultsImproved(flattenResult)
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

		// Process default values if configured (skip in dry-run mode)
		if cfg.DefaultValues.Enabled && !dryRun {
			fmt.Printf("\n⚙️  %sProcessing default values...%s\n", colorCyan, colorReset)
			defaultsOpts := transform.DefaultsOptions{
				Options:       opts,
				DefaultValues: cfg.DefaultValues,
			}
			defaultsResult, err := transform.ProcessDefaultsInDir(cfg.Input, defaultsOpts)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Default values processing error:", err)
				os.Exit(2)
			}

			printDefaultsResults(defaultsResult)
		}

		// Run validation if requested
		if cfg.Validate && !dryRun {
			fmt.Printf("\n🔍 %sValidating OpenAPI specifications...%s\n", colorCyan, colorReset)
			if err := RunSwaggerValidate(cfg.Input); err != nil {
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
