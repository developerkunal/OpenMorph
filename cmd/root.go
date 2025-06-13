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

// version is set by GoReleaser at build time. Do not update manually.
var version = "dev"

var (
	inputDir   string
	configFile string
	inlineMaps []string

	dryRun   bool
	backup   bool
	exclude  []string
	validate bool
	noConfig bool
)

var rootCmd = &cobra.Command{
	Use:     "openmorph",
	Short:   "Transform OpenAPI vendor extension keys via mapping",
	Long:    `OpenMorph: Transform OpenAPI vendor extension keys in YAML/JSON files via mapping config or inline args.`,
	Version: version,
	Run: func(cmd *cobra.Command, _ []string) {
		if cmd.Flag("version") != nil && cmd.Flag("version").Changed {
			fmt.Println("OpenMorph version:", version)
			return
		}
		fmt.Printf("[DEBUG] backup flag value at start of Run: %v\n", backup)
		cfg, err := config.LoadConfig(configFile, inlineMaps, inputDir, noConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Config error:", err)
			os.Exit(1)
		}
		// Merge CLI --exclude and --validate with config
		if len(exclude) > 0 {
			cfg.Exclude = append(cfg.Exclude, exclude...)
		}
		if validate {
			cfg.Validate = true
		}

		// Pretty-print config summary
		fmt.Println("\n\033[1;36mLoaded config:\033[0m")
		fmt.Printf("  \033[1;34mInput:   \033[0m%s\n", cfg.Input)
		fmt.Printf("  \033[1;34mBackup:  \033[0m%v\n", cfg.Backup)
		fmt.Printf("  \033[1;34mValidate:\033[0m %v\n", cfg.Validate)
		fmt.Printf("  \033[1;34mExclude: \033[0m%v\n", cfg.Exclude)
		fmt.Printf("  \033[1;34mMappings:\033[0m\n")
		for k, v := range cfg.Mappings {
			fmt.Printf("    %s \033[1;32m→\033[0m %s\n", k, v)
		}

		fmt.Printf("Input dir: %s\n", cfg.Input)

		// If interactive flag is set, launch TUI for preview/approval BEFORE any transformation
		if cmd.Flag("interactive") != nil && cmd.Flag("interactive").Changed {
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
						Backup:   backup,
					})
					if err != nil {
						fmt.Fprintf(os.Stderr, "Transform error for %s: %v\n", f, err)
					} else if ok {
						actuallyChanged = append(actuallyChanged, f)
						// If backup is requested, ensure backup file is created
						if backup {
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
			return
		}

		// Non-interactive path: Call the transformer
		opts := transform.Options{
			Mappings: cfg.Mappings,
			Exclude:  cfg.Exclude,
			DryRun:   dryRun,
			Backup:   backup, // always use CLI flag value
		}
		changed, err := transform.Dir(cfg.Input, opts)
		fmt.Printf("Files detected for transform: %v\n", changed)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Transform error:", err)
			os.Exit(2)
		}
		fmt.Printf("Transformed files: %v\n", changed)

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
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would change without writing files")
	rootCmd.PersistentFlags().BoolVar(&backup, "backup", false, "Save a .bak copy before overwriting")
	rootCmd.PersistentFlags().StringArrayVar(&exclude, "exclude", nil, "Keys to exclude from transformation (repeatable)")
	rootCmd.PersistentFlags().BoolVar(&validate, "validate", false, "Run swagger-cli validate after transforming")
	rootCmd.PersistentFlags().Bool("interactive", false, "Launch a TUI for interactive preview and approval")
	rootCmd.PersistentFlags().BoolVar(&noConfig, "no-config", false, "Ignore all config files and use only CLI flags")
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

// execCommand is a wrapper for exec.Command (for testability)
var execCommand = func(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
