package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCLI_TransformDryRun(t *testing.T) {
	// Setup: create a test OpenAPI file
	dir := "../testdata/openapi"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	defer os.RemoveAll("../testdata")
	file := dir + "/test.yaml"
	if err := os.WriteFile(file, []byte("x-a: 1\nx-b: 2\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	// Create a mapping config
	cfg := "../testdata/mapping.yaml"
	if err := os.WriteFile(cfg, []byte("mappings:\n  x-a: x-z\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Debug: print all files in input dir before running CLI
	files, err := os.ReadDir(dir)
	if err == nil {
		var names []string
		for _, fi := range files {
			names = append(names, fi.Name())
		}
		t.Logf("Files in input dir before CLI: %v", names)
	}
	lsCmd := exec.Command("ls", "-l", dir)
	lsOut, _ := lsCmd.CombinedOutput()
	t.Logf("ls -l before CLI: %s", lsOut)

	cmd := exec.Command("go", "run", "../main.go", "--input", dir, "--config", cfg, "--dry-run")
	out, err := cmd.CombinedOutput()
	_ = err // suppress unused warning, handled below
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Transformed files") {
		t.Errorf("expected transformed files output, got: %s", out)
	}
	// File should not be changed
	data, _ := os.ReadFile(file)
	if strings.Contains(string(data), "x-z") {
		t.Error("file should not be changed in dry-run mode")
	}
}

func TestCLI_TransformBackup(t *testing.T) {
	dir := "../testdata/backup"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	defer os.RemoveAll("../testdata")
	f := dir + "/test-backup.yaml"
	file, err := os.Create(f)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if _, err := file.Write([]byte("x-a: 1\n")); err != nil {
		file.Close()
		t.Fatalf("failed to write file: %v", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatalf("failed to sync file: %v", err)
	}
	file.Close()

	// Debug: print files in input dir before CLI
	files, err := os.ReadDir(dir)
	if err == nil {
		var names []string
		for _, fi := range files {
			names = append(names, fi.Name())
		}
		t.Logf("Files in input dir before CLI: %v", names)
	}

	cmd := exec.Command("go", "run", "../main.go", "--input", dir, "--map", "x-a=x-z", "--backup")
	cmd.Dir = mustGetwd() // Ensure CLI runs in the same working directory as the test
	cmd.Env = append(os.Environ(), "OPENMORPH_DEBUG=1")
	out, err := cmd.CombinedOutput()
	_ = err // suppress unused warning, handled below
	t.Logf("CWD: %s", mustGetwd())
	t.Logf("CMD OUT: %s", out)

	// Debug: print files in input dir after CLI
	filesAfter, err := os.ReadDir(dir)
	if err == nil {
		var names []string
		for _, fi := range filesAfter {
			names = append(names, fi.Name())
		}
		t.Logf("Files in input dir after CLI: %v", names)
	}

	if err != nil {
		t.Fatalf("backup failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(f + ".bak"); err != nil {
		t.Errorf("expected backup file to exist")
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "(err)"
	}
	return wd
}
