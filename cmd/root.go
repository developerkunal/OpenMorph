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
		fmt.Printf("[DEBUG] backup flag value at start of Run: %v\n", backup)
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
		fmt.Println("\n\033[1;36mLoaded config:\033[0m")
		fmt.Printf("  \033[1;34mInput:   \033[0m%s\n", cfg.Input)
		fmt.Printf("  \033[1;34mBackup:  \033[0m%v\n", cfg.Backup)
		fmt.Printf("  \033[1;34mValidate:\033[0m %v\n", cfg.Validate)
		fmt.Printf("  \033[1;34mFlatten Responses:\033[0m %v\n", cfg.FlattenResponses)
		fmt.Printf("  \033[1;34mExclude: \033[0m%v\n", cfg.Exclude)
		if len(cfg.PaginationPriority) > 0 {
			fmt.Printf("  \033[1;34mPagination Priority:\033[0m %v\n", cfg.PaginationPriority)
		}
		fmt.Printf("  \033[1;34mMappings:\033[0m\n")
		for k, v := range cfg.Mappings {
			fmt.Printf("    %s \033[1;32m→\033[0m %s\n", k, v)
		}

		fmt.Printf("Input dir: %s\n", cfg.Input)

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
				fmt.Println("\033[1;33mNo files were transformed.\033[0m")
			} else {
				fmt.Printf("\033[1;32mTransformed files:\033[0m %v\n", actuallyChanged)
			}

			// Process pagination if priority is configured (for interactive mode)
			if len(cfg.PaginationPriority) > 0 && len(actuallyChanged) > 0 {
				fmt.Printf("\033[1;36mProcessing pagination with priority: %v\033[0m\n", cfg.PaginationPriority)
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

			// Run validation if requested (for interactive mode)
			if cfg.Validate {
				if err := runSwaggerValidate(cfg.Input); err != nil {
					fmt.Fprintln(os.Stderr, "Validation error:", err)
					os.Exit(3)
				}
				fmt.Println("Validation passed.")
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
			if cfg.FlattenResponses {
				fmt.Printf("\033[1;36m[STEP 2] Response flattening changes\033[0m\n")
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
				fmt.Printf("\033[1;36m[STEP 3] Validation\033[0m\n")
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

		// Run validation if requested
		if cfg.Validate && !dryRun {
			if err := runSwaggerValidate(cfg.Input); err != nil {
				fmt.Fprintln(os.Stderr, "Validation error:", err)
				os.Exit(3)
			}
			fmt.Println("Validation passed.")
		}
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
		fmt.Println("Validating:", f)
		if code := runShell(cmd); code != 0 {
			return fmt.Errorf("swagger-cli validate failed for %s", f)
		}
	}
	return nil
}

// runShell runs a shell command and returns the exit code
func runShell(cmd string) int {
	c := os.Getenv("SHELL")
	if c == "" {
		c = "/bin/sh"
	}
	proc := execCommand(c, "-c", cmd)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	if err := proc.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

// printPaginationResults prints the pagination processing results
func printPaginationResults(paginationResult *transform.PaginationResult) {
	if paginationResult.Changed {
		fmt.Printf("Pagination processing completed:\n")
		fmt.Printf("  \033[1;32mProcessed files:\033[0m %d\n", len(paginationResult.ProcessedFiles))
		if len(paginationResult.RemovedParams) > 0 {
			fmt.Printf("  \033[1;33mRemoved parameters:\033[0m\n")
			for endpoint, params := range paginationResult.RemovedParams {
				fmt.Printf("    %s: %v\n", endpoint, params)
			}
		}
		if len(paginationResult.RemovedResponses) > 0 {
			fmt.Printf("  \033[1;33mRemoved responses:\033[0m\n")
			for endpoint, responses := range paginationResult.RemovedResponses {
				fmt.Printf("    %s: %v\n", endpoint, responses)
			}
		}
		if len(paginationResult.UnusedComponents) > 0 {
			fmt.Printf("  \033[1;31mRemoved unused components:\033[0m %v\n", paginationResult.UnusedComponents)
		}
	} else {
		fmt.Println("  \033[1;33mNo pagination changes needed\033[0m")
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

// execCommand is a wrapper for exec.Command (for testability)
var execCommand = func(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
