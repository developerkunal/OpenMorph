package transform

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Options struct {
	Mappings map[string]string
	Exclude  []string
	DryRun   bool
	Backup   bool
}

// KeyChange represents a change in a key's mapping.
type KeyChange struct {
	File   string
	OldKey string
	NewKey string
	Line   int // Add line number for YAML changes, 0 for JSON
}

// Dir walks a directory and transforms all YAML/JSON files.
func Dir(dir string, opts Options) ([]string, error) {
	var changed []string
	var allFiles []string
	var dryRunChanges []KeyChange
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		allFiles = append(allFiles, path)
		if IsYAML(path) || IsJSON(path) {
			ok, err := FileWithChanges(path, opts, &dryRunChanges)
			if err != nil {
				return err
			}
			if ok {
				changed = append(changed, path)
			}
		}
		return nil
	})
	if opts.DryRun && len(dryRunChanges) > 0 {
		printDryRunSummary(dryRunChanges)
	}
	// Debug: print all files found
	if os.Getenv("OPENMORPH_DEBUG") == "1" {
		println("[DEBUG] All files found:", strings.Join(allFiles, ", "))
		// Print working directory for debug
		if wd, err := os.Getwd(); err == nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Current working directory: %s\n", wd)
		}
	}
	return changed, err
}

// FileWithChanges is like File, but collects key changes for dry-run summary.
func FileWithChanges(path string, opts Options, changes *[]KeyChange) (bool, error) {
	orig, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if IsJSON(path) {
		patched, changed := patchJSONKeysWithChanges(orig, opts, path, changes)
		if opts.DryRun {
			return changed, nil
		}
		if changed {
			if opts.Backup {
				_ = os.WriteFile(path+".bak", orig, 0600)
			}
			return true, os.WriteFile(path, patched, 0600)
		}
		return false, nil
	}
	// YAML: use yaml.Node approach (preserves order/comments for YAML)
	var node yaml.Node
	if err := yaml.Unmarshal(orig, &node); err != nil {
		return false, err
	}
	root := &node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		root = node.Content[0]
	}
	changed := transformMapNodeWithChanges(root, opts, path, changes)
	if !changed {
		return false, nil
	}
	out, err := yaml.Marshal(&node)
	if err != nil {
		return false, err
	}
	if opts.DryRun {
		return !equalBytes(orig, out), nil
	}
	if opts.Backup {
		absPath, _ := filepath.Abs(path + ".bak")
		fmt.Fprintf(os.Stderr, "[DEBUG] Writing backup file: %s\n", absPath)
		_ = os.WriteFile(path+".bak", orig, 0600)
	}
	return !equalBytes(orig, out), os.WriteFile(path, out, 0600)
}

// patchJSONKeysWithChanges is like patchJSONKeys, but records changes.
func patchJSONKeysWithChanges(orig []byte, opts Options, file string, changes *[]KeyChange) ([]byte, bool) {
	if len(opts.Mappings) == 0 {
		return orig, false
	}
	changed := false
	patched := orig
	for from, to := range opts.Mappings {
		// Only replace keys that are quoted and followed by a colon ("key":)
		needle := []byte("\"" + from + "\":")
		replacement := []byte("\"" + to + "\":")
		if bytes.Contains(patched, needle) {
			patched = bytes.ReplaceAll(patched, needle, replacement)
			changed = true
			if opts.DryRun && changes != nil {
				*changes = append(*changes, KeyChange{File: file, OldKey: from, NewKey: to})
			}
		}
	}
	return patched, changed
}

// transformMapNodeWithChanges is like transformMapNode, but records changes.
func transformMapNodeWithChanges(n *yaml.Node, opts Options, file string, changes *[]KeyChange) bool {
	changed := false
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			v := n.Content[i+1]
			if isExcludedKey(k.Value, opts.Exclude) {
				continue
			}
			if applyMappingAndRecordChange(k, opts, file, changes) {
				changed = true
			}
			if transformMapNodeWithChanges(v, opts, file, changes) {
				changed = true
			}
		}
	} else if n.Kind == yaml.SequenceNode {
		for _, v := range n.Content {
			if transformMapNodeWithChanges(v, opts, file, changes) {
				changed = true
			}
		}
	}
	return changed
}

// isExcludedKey returns true if the key is in the exclude list.
func isExcludedKey(key string, exclude []string) bool {
	if exclude == nil {
		return false
	}
	for _, ex := range exclude {
		if key == ex {
			return true
		}
	}
	return false
}

// applyMappingAndRecordChange applies a mapping to a key node and records the change if needed.
func applyMappingAndRecordChange(k *yaml.Node, opts Options, file string, changes *[]KeyChange) bool {
	if opts.Mappings == nil {
		return false
	}
	if to, ok := opts.Mappings[k.Value]; ok {
		if opts.DryRun && changes != nil {
			line := 0
			if k.Line > 0 {
				line = k.Line
			}
			*changes = append(*changes, KeyChange{File: file, OldKey: k.Value, NewKey: to, Line: line})
		}
		k.Value = to
		return true
	}
	return false
}

// printDryRunSummary prints a beautiful summary table of all key changes.
func printDryRunSummary(changes []KeyChange) {
	if len(changes) == 0 {
		fmt.Println("No changes would be made.")
		return
	}
	fmt.Println("\n────────────────────────────────────────────────────────────")
	fmt.Println("  DRY RUN: OpenMorph Key Transform Preview")
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Printf("Total changes: %d\n", len(changes))

	files := groupChangesByFile(changes)
	fmt.Printf("Files affected: %d\n", len(files))
	fmt.Println("────────────────────────────────────────────────────────────")

	for file, fileChanges := range files {
		fmt.Printf("\033[1;36mFile: %s\033[0m\n", file)
		fmt.Println(strings.Repeat("─", 40))
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("  (Could not read file: %v)\n", err)
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, c := range fileChanges {
			indices := findKeyLineIndices(lines, c.OldKey)
			if len(indices) == 0 {
				printNoMatchBlock(c)
				continue
			}
			for _, idx := range indices {
				blockStart, blockEnd := extractBlock(lines, idx)
				printDiffBlock(lines, blockStart, blockEnd, c.OldKey, c.NewKey)
			}
		}
		fmt.Println(strings.Repeat("─", 40))
	}
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Println("  No files were modified. Use without --dry-run to apply.")
	fmt.Println("────────────────────────────────────────────────────────────")
}

// groupChangesByFile groups KeyChange slices by file name.
func groupChangesByFile(changes []KeyChange) map[string][]KeyChange {
	files := make(map[string][]KeyChange)
	for _, c := range changes {
		files[c.File] = append(files[c.File], c)
	}
	return files
}

// findKeyLineIndices returns all line indices in lines that match the given key (with variants).
func findKeyLineIndices(lines []string, key string) []int {
	indices := []int{}
	keyVariants := []string{key, "\"" + key + "\"", "'" + key + "'"}
	for i, l := range lines {
		trimmed := strings.ReplaceAll(strings.ReplaceAll(l, " ", ""), "\t", "")
		for _, variant := range keyVariants {
			if strings.Contains(l, variant+":") || strings.Contains(trimmed, variant+":") {
				indices = append(indices, i)
				break
			}
		}
	}
	return indices
}

// extractBlock returns the start and end line indices of a block (array/object or single line) starting at idx.
func extractBlock(lines []string, idx int) (int, int) {
	l := lines[idx]
	blockStart := idx
	blockEnd := idx
	if strings.Contains(l, "[") || strings.Contains(l, "{") {
		open, close := '[', ']'
		if strings.Contains(l, "{") {
			open, close = '{', '}'
		}
		depth := 0
		for j := idx; j < len(lines); j++ {
			if strings.Contains(lines[j], string(open)) {
				depth++
			}
			if strings.Contains(lines[j], string(close)) {
				depth--
			}
			if depth == 0 {
				blockEnd = j
				break
			}
		}
	}
	return blockStart, blockEnd
}

// printDiffBlock prints the before/after block diff for a key change.
func printDiffBlock(lines []string, blockStart, blockEnd int, oldKey, newKey string) {
	fmt.Println("\033[1;31m-")
	for k := blockStart; k <= blockEnd; k++ {
		fmt.Println(lines[k])
	}
	fmt.Println("\033[0m")
	fmt.Println("\033[1;32m+")
	for k := blockStart; k <= blockEnd; k++ {
		if k == blockStart {
			fmt.Println(strings.ReplaceAll(lines[k], oldKey, newKey))
		} else {
			fmt.Println(lines[k])
		}
	}
	fmt.Println("\033[0m")
}

// printNoMatchBlock prints a message when no matching block is found for a key change.
func printNoMatchBlock(c KeyChange) {
	fmt.Printf("\033[1;31m- (no matching block found for key %s)\033[0m\n", c.OldKey)
	fmt.Printf("\033[1;32m+ (no matching block found for key %s)\033[0m\n", c.NewKey)
}

// Exported extension helpers for reuse in cmd/root.go
// IsYAML returns true if the file has a .yaml or .yml extension.
func IsYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// IsJSON returns true if the file has a .json extension.
func IsJSON(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".json"
}

func equalBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// Restore shouldExclude for tests
func shouldExclude(key string, exclude []string) bool {
	for _, ex := range exclude {
		if key == ex {
			return true
		}
	}
	return false
}

// Restore File for tests (as a wrapper for FileWithChanges)
func File(path string, opts Options) (bool, error) {
	return FileWithChanges(path, opts, nil)
}
