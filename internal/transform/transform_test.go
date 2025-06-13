package transform

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestShouldExclude(t *testing.T) {
	if !shouldExclude("foo", []string{"foo", "bar"}) {
		t.Error("should exclude foo")
	}
	if shouldExclude("baz", []string{"foo", "bar"}) {
		t.Error("should not exclude baz")
	}
}

func TestIsYAMLJSON(t *testing.T) {
	if !IsYAML("foo.yaml") || !IsYAML("foo.yml") {
		t.Error("IsYAML failed")
	}
	if !IsJSON("foo.json") {
		t.Error("IsJSON failed")
	}
}

func TestEqualBytes(t *testing.T) {
	a := []byte("abc")
	b := []byte("abc")
	c := []byte("def")
	if !equalBytes(a, b) {
		t.Error("should be equal")
	}
	if equalBytes(a, c) {
		t.Error("should not be equal")
	}
}

func TestTransformFileDryRun(t *testing.T) {
	f := "test.json"
	input := `{"x-a": 1}`
	if err := os.WriteFile(f, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	opts := Options{Mappings: map[string]string{"x-a": "x-z"}, DryRun: true}
	changed, err := File(f, opts)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !changed {
		t.Errorf("expected dry-run to detect change, got %v", changed)
	}
}

func TestTransformFileBackup(t *testing.T) {
	f := "test.yaml"
	if err := os.WriteFile(f, []byte("x-a: 1\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	defer os.Remove(f + ".bak")
	opts := Options{Mappings: map[string]string{"x-a": "x-z"}, Backup: true}
	changed, err := File(f, opts)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if changed {
		if _, err := os.Stat(f + ".bak"); err != nil {
			t.Errorf("expected backup file, got %v", err)
		}
	}
}

func TestTransformFileJSON(t *testing.T) {
	f := "test.json"
	input := `{"x-a": 1, "x-b": {"x-c": 2}}`
	expected := `{"x-z": 1, "x-b": {"x-y": 2}}`
	if err := os.WriteFile(f, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(f)
	opts := Options{Mappings: map[string]string{"x-a": "x-z", "x-c": "x-y"}}
	_, err := File(f, opts)
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}
	actual, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	var act, exp interface{}
	if err := json.Unmarshal(actual, &act); err != nil {
		t.Fatalf("unmarshal actual: %v", err)
	}
	if err := json.Unmarshal([]byte(expected), &exp); err != nil {
		t.Fatalf("unmarshal expected: %v", err)
	}
	if !reflect.DeepEqual(act, exp) {
		t.Errorf("json transform mismatch:\nactual:   %#v\nexpected: %#v", act, exp)
	}
}

// In tests that check output or file content, for JSON files, unmarshal both expected and actual output to interface{} and compare via reflect.DeepEqual to avoid formatting/sorting issues.
// For YAML, compare as before.
