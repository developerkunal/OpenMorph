package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	files := []FileDiff{{Path: "foo.yaml", Diff: "diff", Changed: true}}
	m := NewModel(files)
	if len(m.Files) != 1 || m.Files[0].Path != "foo.yaml" {
		t.Errorf("unexpected model files: %+v", m.Files)
	}
}

func TestModelNavigation(t *testing.T) {
	files := []FileDiff{{Path: "a", Diff: "d1", Changed: true}, {Path: "b", Diff: "d2", Changed: false}}
	m := NewModel(files)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j"), Alt: false})
	// nolint:errcheck
	m = m2.(Model)
	if m.Index != 1 {
		t.Errorf("expected index 1, got %d", m.Index)
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k"), Alt: false})
	// nolint:errcheck
	m = m2.(Model)
	if m.Index != 0 {
		t.Errorf("expected index 0, got %d", m.Index)
	}
}

func TestModelAcceptSkip(t *testing.T) {
	files := []FileDiff{{Path: "a", Diff: "d1", Changed: true}, {Path: "b", Diff: "d2", Changed: false}}
	m := NewModel(files)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a"), Alt: false})
	// nolint:errcheck
	m = m2.(Model)
	if !m.Accepted["a"] {
		t.Errorf("expected 'a' to be accepted")
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s"), Alt: false})
	// nolint:errcheck
	m = m2.(Model)
	if !m.Skipped["b"] {
		t.Errorf("expected 'b' to be skipped")
	}
}

func TestModelQuit(t *testing.T) {
	files := []FileDiff{{Path: "a", Diff: "d1", Changed: true}}
	m := NewModel(files)
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q"), Alt: false})
	// nolint:errcheck
	m = m2.(Model)
	if !m.Quitting {
		t.Errorf("expected quitting to be true")
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit command")
	}
}

func TestModelView(t *testing.T) {
	files := []FileDiff{{Path: "a", Diff: "d1", Changed: true}}
	m := NewModel(files)
	view := m.View()
	if len(view) == 0 {
		t.Errorf("expected non-empty view")
	}
}
