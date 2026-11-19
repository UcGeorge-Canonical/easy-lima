package tui

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

type storageLoaded struct {
	Path    string
	Entries []project.Entry
	Err     error
}
type locationsLoaded []storage.Location

type storageModel struct {
	ctx                                            context.Context
	cwd, home, path, result, input, inputKind, err string
	locations                                      []storage.Location
	entries                                        []project.Entry
	cursor, width, height                          int
	loading, folders, hidden, cancelled            bool
}

func (m storageModel) Init() tea.Cmd {
	return func() tea.Msg { return locationsLoaded(storage.Locations(m.ctx, m.cwd, m.home)) }
}
func (m storageModel) read(path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := project.Children(m.ctx, path, nil, project.TreeOptions{Hidden: m.hidden, Dependencies: true, Limit: 1000})
		return storageLoaded{path, entries, err}
	}
}
func (m storageModel) options() []string {
	if !m.folders {
		out := make([]string, 0, len(m.locations)+1)
		for _, l := range m.locations {
			out = append(out, fmt.Sprintf("%s · %.1f GiB free\n    %s", l.Label, float64(l.AvailableBytes)/(1<<30), l.Path))
		}
		return append(out, "Enter another path…")
	}
	out := []string{"Use this folder", "New storage folder here…", "↑ Parent folder", "← Drives and locations"}
	for _, e := range m.entries {
		if e.Error == "" {
			out = append(out, "▸ "+e.Name)
		} else {
			out = append(out, "! "+e.Name+" (unavailable)")
		}
	}
	return out
}
func (m storageModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case locationsLoaded:
		m.locations, m.loading = msg, false
		m.cursor = min(max(0, m.cursor), len(m.options())-1)
	case storageLoaded:
		if msg.Path != m.path {
			return m, nil
		}
		m.loading, m.entries = false, msg.Entries
		m.cursor = min(max(0, m.cursor), len(m.options())-1)
		if msg.Err != nil {
			m.err = "Cannot read this folder. Choose its parent or another location."
		}
	case tea.PasteMsg:
		if m.inputKind != "" && len(m.input)+len(msg.Content) <= 8192 {
			m.input += strings.Map(func(r rune) rune {
				if r < 32 || r == 127 {
					return -1
				}
				return r
			}, msg.Content)
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			m.cancelled = true
			return m, tea.Quit
		}
		if m.inputKind != "" {
			switch key {
			case "esc":
				m.inputKind, m.err = "", ""
			case "ctrl+u":
				m.input = ""
			case "backspace":
				r := []rune(m.input)
				if len(r) > 0 {
					m.input = string(r[:len(r)-1])
				}
			case "enter":
				p := strings.TrimSpace(m.input)
				if m.inputKind == "folder" {
					if p == "" || p == "." || p == ".." || filepath.Base(p) != p {
						m.err = "Enter a folder name, without slashes."
						return m, nil
					}
					p = filepath.Join(m.path, p)
				}
				resolved, err := config.ResolvePath(p, m.cwd)
				if err != nil {
					m.err = "Enter a valid absolute or relative directory path."
					return m, nil
				}
				m.result = resolved
				return m, tea.Quit
			default:
				if key == "space" {
					m.input += " "
				} else if len([]rune(key)) == 1 && len(m.input) < 8192 {
					m.input += key
				}
			}
			return m, nil
		}
		switch key {
		case "esc", "q":
			if m.folders {
				m.folders = false
				m.cursor = 0
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(m.options())-1, m.cursor+1)
		case "h":
			if m.folders && !m.loading {
				m.hidden = !m.hidden
				m.loading = true
				return m, m.read(m.path)
			}
		case "left", "backspace":
			if m.folders && !m.loading {
				m.path = filepath.Dir(m.path)
				m.cursor = 0
				m.loading = true
				return m, m.read(m.path)
			}
		case "enter", "right":
			if m.loading {
				break
			}
			m.err = ""
			if !m.folders {
				if m.cursor == len(m.locations) {
					m.inputKind = "path"
					m.input = ""
					break
				}
				m.path = m.locations[m.cursor].Path
				m.folders = true
				m.cursor = 1
				m.loading = true
				return m, m.read(m.path)
			}
			switch m.cursor {
			case 0:
				m.result = m.path
				return m, tea.Quit
			case 1:
				m.inputKind = "folder"
				m.input = "easy-lima-vms"
			case 2:
				m.path = filepath.Dir(m.path)
				m.cursor = 0
				m.loading = true
				return m, m.read(m.path)
			case 3:
				m.folders = false
				m.cursor = 0
			default:
				if m.cursor < 4 || m.cursor-4 >= len(m.entries) {
					m.cursor = 0
					break
				}
				e := m.entries[m.cursor-4]
				if e.Error != "" {
					m.err = "This folder is unavailable."
					break
				}
				m.path = e.Path
				m.cursor = 0
				m.loading = true
				return m, m.read(m.path)
			}
		}
	}
	return m, nil
}
func (m storageModel) View() tea.View {
	var b strings.Builder
	b.WriteString("\x1b[1;36measy-lima\x1b[0m  Choose VM storage\n\n")
	b.WriteString("VM disks, downloaded images, and large temporary files will live here.\n")
	b.WriteString("Nothing is created until you accept the setup preview.\n\n")
	if m.folders {
		b.WriteString(truncate(m.path, m.width-2) + "\n\n")
	}
	if m.inputKind != "" {
		label := "Directory path"
		if m.inputKind == "folder" {
			label = "New folder name"
		}
		fmt.Fprintf(&b, "%s\n› %s▏\n\nEnter Review location   Ctrl-U Clear   Esc Back\n", label, clean(m.input))
	} else if m.loading {
		b.WriteString("Reading locations…\n")
	} else {
		options := m.options()
		count := max(1, (m.height-11)/2)
		start := max(0, min(m.cursor-count/2, len(options)-count))
		for i := start; i < min(len(options), start+count); i++ {
			prefix := "  "
			if i == m.cursor {
				prefix = "› "
			}
			for j, line := range strings.Split(options[i], "\n") {
				if j > 0 {
					prefix = "  "
				}
				b.WriteString(truncate(prefix+line, m.width-2) + "\n")
			}
		}
		b.WriteString("\n↑↓ Choose   Enter Select / browse   ← Parent   h Hidden folders   Esc Back\n")
	}
	if m.err != "" {
		b.WriteString("\n" + clean(m.err) + "\n")
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func PickStorage(ctx context.Context, cwd, home string, in io.Reader, out io.Writer) (string, error) {
	m := storageModel{ctx: ctx, cwd: cwd, home: home, width: 100, height: 30, loading: true}
	result, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return "", err
	}
	s := result.(storageModel)
	if s.cancelled {
		return "", nil
	}
	return s.result, nil
}
