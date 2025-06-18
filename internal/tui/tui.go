package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/developerkunal/OpenMorph/internal/transform"
)

// FileDiff represents the diff for a single file, including the path, diff content,
// and whether it has changed. It also holds the key changes detected in the file.
type FileDiff struct {
	Path       string
	Diff       string
	Changed    bool
	KeyChanges []transform.KeyChange // Add key changes for this file
}

// changeItem represents a single before/after key change block for display in the TUI list.
type changeItem struct {
	oldKey, newKey   string // The old and new key names
	oldLine, newLine string // The full before/after block (may be multi-line)
}

func (i changeItem) Title() string       { return i.oldKey + " → " + i.newKey }
func (i changeItem) Description() string { return "- " + i.oldLine + "\n+ " + i.newLine }
func (i changeItem) FilterValue() string { return i.oldKey + i.newKey + i.oldLine }

// Model represents the state of the TUI for reviewing OpenAPI key changes.
// It tracks the list of files, navigation state, accepted/skipped files, and the Bubble Tea list model.
type Model struct {
	Files    []FileDiff      // All files with detected key changes
	Index    int             // Current file index
	Accepted map[string]bool // Files the user has accepted
	Skipped  map[string]bool // Files the user has skipped
	Quitting bool            // Whether the user has quit the TUI
	ShowHelp bool            // Whether to show the help/footer
	List     list.Model      // Bubble Tea list for navigating changes
}

var (
	changedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	headerStyle      = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("229")).Padding(0, 1)
	footerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	oldKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)  // red
	newKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true) // green
	progressBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
)

// changeDelegate implements list.ItemDelegate for rendering change items in the TUI list.
type changeDelegate struct{}

func (changeDelegate) Height() int                             { return 3 }
func (changeDelegate) Spacing() int                            { return 1 }
func (changeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (changeDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(changeItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	title := item.Title()
	desc := item.Description()
	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(true)
		descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	} else {
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
		descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	}
	fmt.Fprintln(w, titleStyle.Render(title))
	for _, line := range strings.Split(desc, "\n") {
		fmt.Fprintln(w, descStyle.Render(line))
	}
}

func NewModel(files []FileDiff) Model {
	items := []list.Item{}
	if len(files) > 0 {
		items = getChangeItems(files[0])
	}
	l := list.New(items, changeDelegate{}, 0, 10)
	l.Title = "Key Changes (old → new)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	return Model{
		Files:    files,
		Accepted: make(map[string]bool),
		Skipped:  make(map[string]bool),
		List:     l,
	}
}

func (Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "q":
			return m.quit()
		case "right", "l", "j":
			return m.navigate(1)
		case "left", "h", "k":
			return m.navigate(-1)
		case "a", "enter":
			return m.acceptCurrent()
		case "s":
			return m.skipCurrent()
		case "A":
			return m.bulkAccept()
		case "S":
			return m.bulkSkip()
		case "?":
			return m.toggleHelp()
		}
	}
	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

// quit sets the quitting flag and returns the quit command.
func (m Model) quit() (tea.Model, tea.Cmd) {
	m.Quitting = true
	return m, tea.Quit
}

// navigate moves the file index by delta and updates the list.
func (m Model) navigate(delta int) (tea.Model, tea.Cmd) {
	newIndex := m.Index + delta
	if newIndex < 0 || newIndex >= len(m.Files) {
		return m, nil
	}
	m.Index = newIndex
	m.updateListForCurrentFile()
	return m, nil
}

// acceptCurrent marks the current file as accepted and moves to the next file or quits.
func (m Model) acceptCurrent() (tea.Model, tea.Cmd) {
	m.Accepted[m.Files[m.Index].Path] = true
	if m.Index == len(m.Files)-1 {
		m.Quitting = true
		return m, tea.Quit
	}
	m.Index = min(m.Index+1, len(m.Files)-1)
	m.updateListForCurrentFile()
	return m, nil
}

// skipCurrent marks the current file as skipped and moves to the next file or quits.
func (m Model) skipCurrent() (tea.Model, tea.Cmd) {
	m.Skipped[m.Files[m.Index].Path] = true
	if m.Index == len(m.Files)-1 {
		m.Quitting = true
		return m, tea.Quit
	}
	m.Index = min(m.Index+1, len(m.Files)-1)
	m.updateListForCurrentFile()
	return m, nil
}

// bulkAccept marks all files as accepted and quits.
func (m Model) bulkAccept() (tea.Model, tea.Cmd) {
	for _, f := range m.Files {
		m.Accepted[f.Path] = true
	}
	m.Quitting = true
	return m, tea.Quit
}

// bulkSkip marks all files as skipped and quits.
func (m Model) bulkSkip() (tea.Model, tea.Cmd) {
	for _, f := range m.Files {
		m.Skipped[f.Path] = true
	}
	m.Quitting = true
	return m, tea.Quit
}

// toggleHelp toggles the help/footer display.
func (m Model) toggleHelp() (tea.Model, tea.Cmd) {
	m.ShowHelp = !m.ShowHelp
	return m, nil
}

// updateListForCurrentFile resets and sets the list items for the current file.
func (m *Model) updateListForCurrentFile() {
	m.List.ResetSelected()
	m.List.SetItems(getChangeItems(m.Files[m.Index]))
}

// extractKeyBlocks finds all before/after blocks for a single key change in the file lines.
func extractKeyBlocks(lines []string, keyChange transform.KeyChange) []changeItem {
	var items []changeItem
	for i, line := range lines {
		trimmed := strings.ReplaceAll(strings.ReplaceAll(line, " ", ""), "\t", "")
		keyVariants := []string{keyChange.OldKey, "\"" + keyChange.OldKey + "\"", "'" + keyChange.OldKey + "'"}
		for _, variant := range keyVariants {
			if strings.Contains(line, variant+":") || strings.Contains(trimmed, variant+":") {
				blockStart, blockEnd := i, i
				if strings.Contains(line, "[") || strings.Contains(line, "{") {
					open, close := '[', ']'
					if strings.Contains(line, "{") {
						open, close = '{', '}'
					}
					depth := 0
					for j := i; j < len(lines); j++ {
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
				oldBlock := make([]string, 0, blockEnd-blockStart+1)
				newBlock := make([]string, 0, blockEnd-blockStart+1)
				for k := blockStart; k <= blockEnd; k++ {
					if k == blockStart {
						oldBlock = append(oldBlock, lines[k])
						newBlock = append(newBlock, strings.ReplaceAll(lines[k], keyChange.OldKey, keyChange.NewKey))
					} else {
						oldBlock = append(oldBlock, lines[k])
						newBlock = append(newBlock, lines[k])
					}
				}
				items = append(items, changeItem{
					oldKey:  keyChange.OldKey,
					newKey:  keyChange.NewKey,
					oldLine: strings.Join(oldBlock, "\n"),
					newLine: strings.Join(newBlock, "\n"),
				})
			}
		}
	}
	return items
}

func getChangeItems(f FileDiff) []list.Item {
	if len(f.KeyChanges) == 0 {
		return []list.Item{
			changeItem{"(no key)", "(no key)", "(no matching line found)", "(no matching line found)"},
		}
	}

	data, err := os.ReadFile(f.Path)
	if err != nil {
		items := make([]list.Item, 0, len(f.KeyChanges))
		for _, c := range f.KeyChanges {
			items = append(items, changeItem{c.OldKey, c.NewKey, "(could not read file)", "(could not read file)"})
		}
		return items
	}

	lines := strings.Split(string(data), "\n")
	var items []list.Item

	for _, keyChange := range f.KeyChanges {
		blocks := extractKeyBlocks(lines, keyChange)
		if len(blocks) == 0 {
			items = append(items, changeItem{
				oldKey:  keyChange.OldKey,
				newKey:  keyChange.NewKey,
				oldLine: "(no matching block found)",
				newLine: "(no matching block found)",
			})
		} else {
			for _, block := range blocks {
				items = append(items, block)
			}
		}
	}

	if len(items) == 0 {
		return []list.Item{
			changeItem{"(no key)", "(no key)", "(no matching line found)", "(no matching line found)"},
		}
	}
	return items
}

func (m Model) View() string {
	if m.Quitting {
		return "Goodbye!"
	}
	if len(m.Files) == 0 {
		return headerStyle.Render("No files to review.")
	}
	var b strings.Builder
	// Progress bar/summary with status icons
	b.WriteString(progressBarStyle.Render("["))
	for i, f := range m.Files {
		switch {
		case m.Accepted[f.Path]:
			b.WriteString(newKeyStyle.Render("✔"))
		case m.Skipped[f.Path]:
			b.WriteString(oldKeyStyle.Render("✗"))
		default:
			b.WriteString("·")
		}
		if i < len(m.Files)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString(progressBarStyle.Render("] "))
	b.WriteString(progressBarStyle.Render(fmt.Sprintf("%d/%d Reviewed | Accepted: %d | Skipped: %d\n", m.Index+1, len(m.Files), len(m.Accepted), len(m.Skipped))))
	b.WriteString(headerStyle.Render(fmt.Sprintf("File %d/%d: %s", m.Index+1, len(m.Files), m.Files[m.Index].Path)))
	b.WriteString("\n\n")
	if m.Files[m.Index].Changed {
		b.WriteString(changedStyle.Render("[CHANGED] "))
	}
	b.WriteString(m.List.View())
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("[a]ccept  [s]kip  [A]ccept all  [S]kip all  [q]uit  [?]help  [arrows] nav"))
	if m.ShowHelp {
		b.WriteString("\n\n")
		b.WriteString(footerStyle.Render("Use arrows to navigate changes/files. 'a' to accept, 's' to skip, 'A' to accept all, 'S' to skip all, 'q' to quit. Press '?' to toggle this help."))
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RunTUI launches the Bubble Tea TUI for file diffs.
func RunTUI(files []FileDiff) (accepted, skipped map[string]bool, err error) {
	m := NewModel(files)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, nil, err
	}
	model, ok := final.(Model)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected model type")
	}
	return model.Accepted, model.Skipped, nil
}
