package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/developerkunal/OpenMorph/internal/transform"
)

// RunSwaggerValidate shells out to swagger-cli validate for all YAML/JSON files in the input dir
func RunSwaggerValidate(inputDir string) error {
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

		if code := RunShellSilent(cmd); code != 0 {
			return fmt.Errorf("swagger-cli validate failed for %s", f)
		}
		fmt.Printf("   %s✅ %s is valid%s\n", colorGreen, f, colorReset)
	}
	return nil
}

// RunShellSilent runs a shell command silently and returns the exit code
func RunShellSilent(cmd string) int {
	c := os.Getenv("SHELL")
	if c == "" {
		c = "/bin/sh"
	}
	proc := ExecCommand(c, "-c", cmd)
	// Don't pipe stdout/stderr to avoid duplicate output
	if err := proc.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

// ExecCommand is a wrapper for exec.Command (for testability)
var ExecCommand = exec.Command
