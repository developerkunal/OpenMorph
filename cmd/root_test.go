package cmd

import (
	"os"
	"os/exec"
	"testing"
)

func TestCLI_Help(t *testing.T) {
	cmd := exec.Command("go", "run", "../main.go", "--help")
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err != nil && !testing.Short() {
		t.Fatalf("help failed: %v\n%s", err, out)
	}
	if len(out) == 0 || string(out) == "" {
		t.Error("expected help output")
	}
}

// More CLI integration tests can be added for real-world scenarios.
