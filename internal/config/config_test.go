package config

import (
	"os"
	"testing"
)

func TestLoadConfig_FileAndInline(t *testing.T) {
	f := "test.yaml"
	if err := os.WriteFile(f, []byte("input: bar\nmappings:\n  x-a: x-b\n  x-c: x-d\n"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	cfg, err := LoadConfig(f, nil, "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Input != "bar" {
		t.Errorf("input override failed: %v", cfg.Input)
	}
	if cfg.Mappings["x-a"] != "x-b" || cfg.Mappings["x-c"] != "x-d" {
		t.Errorf("mappings merge failed: %+v", cfg.Mappings)
	}
}

func TestLoadConfig_RC(t *testing.T) {
	f := ".openapirc.yaml"
	cfgYaml := "input: foo\nmappings:\n  x-a: x-b\n"
	if err := os.WriteFile(f, []byte(cfgYaml), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	cfg, err := LoadConfig("", nil, "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Input != "foo" {
		t.Errorf("rc input failed: %v", cfg.Input)
	}
	if cfg.Mappings["x-a"] != "x-b" {
		t.Errorf("rc mappings failed: %+v", cfg.Mappings)
	}
}

func TestLoadConfig_Required(t *testing.T) {
	_, err := LoadConfig("", nil, "", "", false)
	if err == nil {
		t.Error("expected error for missing input")
	}
}

func TestLoadConfig_OutputSupport(t *testing.T) {
	// Test config file with output
	f := "test_output.yaml"
	if err := os.WriteFile(f, []byte("input: test.yaml\noutput: output.yaml\nmappings:\n  x-a: x-b\n"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	cfg, err := LoadConfig(f, nil, "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "output.yaml" {
		t.Errorf("output from config failed: %v", cfg.Output)
	}

	// Test CLI override
	cfg2, err := LoadConfig(f, nil, "", "cli_output.yaml", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.Output != "cli_output.yaml" {
		t.Errorf("CLI output override failed: %v", cfg2.Output)
	}
}

func TestSplitMap(t *testing.T) {
	if got := splitMap("foo=bar"); got[0] != "foo" || got[1] != "bar" {
		t.Errorf("splitMap failed: %v", got)
	}
	if splitMap("foobar") != nil {
		t.Error("expected nil for invalid map")
	}
}
