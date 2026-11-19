// Package tui provides a bounded, lazy terminal browser. Selecting a folder
// never allocates a VM; explicit actions are returned to the shared coordinator.
package tui

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
)

type Selection struct {
	Path, VMID, Action string
	Cancelled          bool
}
type row struct {
	Entry            project.Entry
	Depth            int
	Expanded, Loaded bool
}
type loaded struct {
	Epoch   int
	Path    string
	Entries []project.Entry
	Err     error
}
type detailsLoaded struct {
	ID    string
	Epoch int
	Lines []string
}
type detailLoader func(context.Context, string) []string

var directorySlots = make(chan struct{}, 2)

type Model struct {
	VMIndex        map[string]*model.VM
	Aliases        map[string]string
	ProfileRecords map[string]model.Profile
	Details        func(context.Context, string) []string
	DetailLines    []string
	DetailID       string
	DetailEpoch    int
	detailCancel   context.CancelFunc

	Context                            context.Context
	Root                               string
	OpenAction                         string
	Epoch                              int
	Rows                               []row
	Cursor                             int
	Width, Height                      int
	Options                            project.TreeOptions
	VMs                                []model.VM
	Profiles                           map[string]string
	Unavailable                        bool
	Registry, Filtering, Help, Actions bool
	Filter, Error                      string
	Notice                             string
	ActionIndex                        int
	Result                             Selection
}

func New(ctx context.Context, root string, vms []model.VM, profiles []model.Profile, unavailable, hidden, dependencies bool) Model {
	names := map[string]string{}
	for _, p := range profiles {
		names[p.ID] = p.Label
	}
	return Model{VMIndex: vmIndex(vms), Aliases: map[string]string{}, ProfileRecords: profileIndex(profiles), Context: ctx, Root: root, Rows: []row{{Entry: project.Entry{Path: root, Name: filepath.Base(root)}, Expanded: true}}, VMs: vms, Profiles: names, Unavailable: unavailable, Options: project.TreeOptions{Hidden: hidden, Dependencies: dependencies, Limit: 10000}, Width: 100, Height: 30}
}
func (m Model) Init() tea.Cmd {
	if len(m.Rows) > 0 && !m.Rows[0].Loaded {
		return m.load(m.Root)
	}
	return nil
}
func (m Model) load(path string) tea.Cmd {
	ancestors := []string{}
	for p := path; ; p = filepath.Dir(p) {
		ancestors = append(ancestors, p)
		if p == filepath.Dir(p) {
			break
		}
	}
	return func() tea.Msg {
		select {
		case directorySlots <- struct{}{}:
		case <-m.Context.Done():
			return loaded{m.Epoch, path, nil, m.Context.Err()}
		}
		defer func() { <-directorySlots }()
		children, err := project.Children(m.Context, path, ancestors, m.Options)
		return loaded{m.Epoch, path, children, err}
	}
}
func (m Model) currentRows() []row {
	rows := m.Rows
	if m.Registry {
		rows = nil
		for _, v := range m.VMs {
			rows = append(rows, row{Entry: project.Entry{Path: v.SourcePath, Name: v.SourcePath}})
		}
	}
	if m.Filter == "" {
		return rows
	}
	out := []row{}
	seen := map[string]bool{}
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Entry.Path), strings.ToLower(m.Filter)) {
			out = append(out, r)
			seen[r.Entry.Path] = true
		}
	}
	for _, v := range m.VMs {
		if !seen[v.SourcePath] && strings.Contains(strings.ToLower(v.SourcePath), strings.ToLower(m.Filter)) {
			out = append(out, row{Entry: project.Entry{Path: v.SourcePath, Name: v.SourcePath}})
		}
	}
	return out
}
func (m Model) vm(path string) *model.VM {
	if canonical := m.Aliases[path]; canonical != "" {
		path = canonical
	}
	return m.VMIndex[path]
}
func vmIndex(vms []model.VM) map[string]*model.VM {
	out := map[string]*model.VM{}
	for i := range vms {
		out[vms[i].SourcePath] = &vms[i]
	}
	return out
}
func profileIndex(profiles []model.Profile) map[string]model.Profile {
	out := map[string]model.Profile{}
	for _, p := range profiles {
		out[p.ID] = p
	}
	return out
}
func (m Model) selectedVMID() string {
	rows := m.currentRows()
	if m.Cursor >= 0 && m.Cursor < len(rows) {
		if v := m.vm(rows[m.Cursor].Entry.Path); v != nil {
			return v.ID
		}
	}
	return ""
}
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if d, ok := message.(detailsLoaded); ok {
		if d.Epoch == m.DetailEpoch && d.ID == m.DetailID {
			m.DetailLines = d.Lines
		}
		return m, nil
	}
	updated, cmd := m.update(message)
	next := updated.(Model)
	id := next.selectedVMID()
	if id != next.DetailID {
		if next.detailCancel != nil {
			next.detailCancel()
		}
		next.DetailEpoch++
		next.DetailID = id
		next.DetailLines = nil
		if id != "" && next.Details != nil {
			ctx, cancel := context.WithCancel(next.Context)
			next.detailCancel = cancel
			epoch := next.DetailEpoch
			loader := next.Details
			details := func() tea.Msg {
				timer := time.NewTimer(150 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return detailsLoaded{id, epoch, nil}
				case <-timer.C:
				}
				bounded, done := context.WithTimeout(ctx, 4*time.Second)
				defer done()
				return detailsLoaded{id, epoch, loader(bounded, id)}
			}
			return next, tea.Batch(cmd, details)
		}
	}
	return next, cmd
}
func (m Model) actions(r row) [][2]string {
	if v := m.vm(r.Entry.Path); v != nil {
		actions := [][2]string{{"code", "Open in VS Code"}, {"ssh", "Open SSH shell"}, {"start", "Start"}, {"stop", "Stop"}, {"restart", "Restart"}, {"inspect", "Details"}, {"doctor", "Diagnostics"}, {"logs", "Operational logs"}, {"environment.update", "Preview environment update"}, {"repair", "Repairs"}, {"rebind", "Rebind moved project"}, {"delete", "Delete VM and disk"}}
		if m.OpenAction == "ssh" {
			actions[0], actions[1] = actions[1], actions[0]
		}
		actions = append(actions[:9], append([][2]string{{"profile.assign", "Change Git profile"}}, actions[9:]...)...)
		if len(v.OperationIDs) > 0 {
			actions = append([][2]string{{"resume", "Resume pending operation"}}, actions...)
		}
		return actions
	}
	return [][2]string{{"create-open", "Create and open"}, {"up", "Create VM"}, {"config.explain", "Preview configuration"}}
}
func (m Model) update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	case loaded:
		if msg.Epoch != m.Epoch {
			return m, nil
		}
		selected := ""
		before := m.currentRows()
		if m.Cursor >= 0 && m.Cursor < len(before) {
			selected = before[m.Cursor].Entry.Path
		}
		for i := range m.Rows {
			if m.Rows[i].Entry.Path != msg.Path {
				continue
			}
			if !m.Rows[i].Expanded || m.Rows[i].Loaded {
				break
			}
			if msg.Err != nil {
				m.Error = "Cannot read this directory."
				break
			}
			m.Rows[i].Loaded = true
			children := make([]row, len(msg.Entries))
			for j, e := range msg.Entries {
				if e.Canonical != "" {
					m.Aliases[e.Path] = e.Canonical
				}
				children[j] = row{Entry: e, Depth: m.Rows[i].Depth + 1}
			}
			next := append([]row{}, m.Rows[:i+1]...)
			next = append(next, children...)
			next = append(next, m.Rows[i+1:]...)
			m.Rows = next
			if len(msg.Entries) >= m.Options.Limit {
				m.Error = "Directory limit reached; use a narrower root to see more entries."
			}
			break
		}
		for i, row := range m.currentRows() {
			if row.Entry.Path == selected {
				m.Cursor = i
				break
			}
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			m.Result.Cancelled = true
			return m, tea.Quit
		}
		if m.Filtering {
			switch key {
			case "enter", "esc":
				m.Filtering = false
			case "backspace":
				r := []rune(m.Filter)
				if len(r) > 0 {
					m.Filter = string(r[:len(r)-1])
				}
			default:
				if len([]rune(key)) == 1 && len(m.Filter) < 256 {
					m.Filter += key
				}
			}
			m.Cursor = 0
			return m, nil
		}
		if m.Help {
			m.Help = false
			return m, nil
		}
		rows := m.currentRows()
		if m.Cursor >= len(rows) {
			m.Cursor = max(0, len(rows)-1)
		}
		if m.Actions {
			if len(rows) == 0 {
				m.Actions = false
				return m, nil
			}
			actions := m.actions(rows[m.Cursor])
			m.ActionIndex = min(max(0, m.ActionIndex), len(actions)-1)
			switch key {
			case "esc", "q", "left":
				m.Actions = false
			case "up", "k":
				m.ActionIndex = max(0, m.ActionIndex-1)
			case "down", "j":
				m.ActionIndex = min(len(actions)-1, m.ActionIndex+1)
			case "enter":
				selected := rows[m.Cursor]
				m.Result = Selection{Path: selected.Entry.Path, Action: actions[m.ActionIndex][0]}
				if v := m.vm(selected.Entry.Path); v != nil {
					m.Result.VMID = v.ID
				}
				return m, tea.Quit
			}
			return m, nil
		}
		switch key {
		case "q", "esc":
			m.Result.Cancelled = true
			return m, tea.Quit
		case "up", "k":
			m.Cursor = max(0, m.Cursor-1)
		case "down", "j":
			m.Cursor = min(max(0, len(rows)-1), m.Cursor+1)
		case "home", "g":
			m.Cursor = 0
		case "end", "G":
			m.Cursor = max(0, len(rows)-1)
		case "pgup":
			m.Cursor = max(0, m.Cursor-max(1, m.Height-10))
		case "pgdown":
			m.Cursor = min(max(0, len(rows)-1), m.Cursor+max(1, m.Height-10))
		case "tab", "v":
			m.Registry = !m.Registry
			m.Cursor = 0
			m.Filter = ""
		case "/":
			m.Filtering = true
		case "?":
			m.Help = true
		case "n":
			m.Result.Action = "more"
			return m, tea.Quit
		case "p":
			m.Result.Action = "profiles"
			return m, tea.Quit
		case "o":
			m.Result.Action = "operations"
			return m, tea.Quit
		case "h", "d":
			if key == "h" {
				m.Options.Hidden = !m.Options.Hidden
			} else {
				m.Options.Dependencies = !m.Options.Dependencies
			}
			m.Epoch++
			m.Rows = []row{{Entry: project.Entry{Path: m.Root, Name: filepath.Base(m.Root)}, Expanded: true}}
			m.Cursor = 0
			return m, m.load(m.Root)
		case "enter":
			if len(rows) > 0 && !rows[m.Cursor].Entry.Cycle && rows[m.Cursor].Entry.Error == "" {
				m.Actions = true
				m.ActionIndex = 0
			}
		case "right", "l", "left":
			if len(rows) == 0 || m.Registry || m.Filter != "" {
				break
			}
			selected := rows[m.Cursor]
			if key == "left" {
				if selected.Expanded {
					m.Rows[m.Cursor].Expanded = false
					end := m.Cursor + 1
					for end < len(m.Rows) && m.Rows[end].Depth > selected.Depth {
						end++
					}
					m.Rows = append(m.Rows[:m.Cursor+1], m.Rows[end:]...)
					m.Rows[m.Cursor].Loaded = false
				} else {
					for i := m.Cursor - 1; i >= 0; i-- {
						if m.Rows[i].Depth < selected.Depth {
							m.Cursor = i
							break
						}
					}
				}
			} else if !selected.Expanded && !selected.Entry.Cycle && selected.Entry.Error == "" {
				m.Rows[m.Cursor].Expanded = true
				return m, m.load(selected.Entry.Path)
			}
		}
	}
	return m, nil
}
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}
func truncate(s string, n int) string {
	r := []rune(clean(s))
	if n < 1 {
		return ""
	}
	if len(r) > n {
		return string(r[:max(0, n-1)]) + "…"
	}
	return string(r)
}
func (m Model) View() tea.View {
	var b strings.Builder
	tab := "Projects"
	if m.Registry {
		tab = "All VMs"
	}
	fmt.Fprintf(&b, "\x1b[1;36measy-lima\x1b[0m  %s  \x1b[1m[%s]\x1b[0m\n", truncate(m.Root, m.Width-30), tab)
	fmt.Fprintf(&b, "Hidden: %t   Dependencies: %t   Profiles [p]   Operations [o]\n\n", m.Options.Hidden, m.Options.Dependencies)
	if m.Help {
		b.WriteString("↑/↓ move   → expand   ← collapse or parent\nEnter project actions   Tab all VMs   / filter loaded tree and registrations\nh toggle hidden folders   d toggle dependencies\np profiles   o operations   n load more registry records   q quit\n\nVM state labels are last recorded state. Doctor observes current state.\nClosing this browser leaves VMs, SSH agents, and identity tunnels available.\nPress any key to return.\n")
	} else {
		rows := m.currentRows()
		height := max(3, m.Height-18)
		start := max(0, m.Cursor-height/2)
		if start+height > len(rows) {
			start = max(0, len(rows)-height)
		}
		end := min(len(rows), start+height)
		if m.Actions && len(rows) > 0 {
			start = m.Cursor
			end = m.Cursor + 1
		}
		for i := start; i < end; i++ {
			r := rows[i]
			prefix := "  "
			if i == m.Cursor {
				prefix = "› "
			}
			branch := "▸"
			if r.Expanded {
				branch = "▾"
			}
			if r.Entry.Cycle {
				branch = "↻"
			}
			name := strings.Repeat("  ", r.Depth) + branch + " " + r.Entry.Name
			if r.Entry.Symlink {
				name += " ↗"
			}
			label := ""
			if vm := m.vm(r.Entry.Path); vm != nil {
				profile := "no profile"
				if vm.ProfileID != nil {
					profile = m.Profiles[*vm.ProfileID]
				}
				label = "  [" + vm.State.Recorded + " · " + profile + "]"
				if len(vm.OperationIDs) > 0 {
					label += " recovery pending"
				}
			}
			if r.Entry.Error != "" {
				label += " [unavailable]"
			}
			line := truncate(prefix+name+label, m.Width-2)
			if i == m.Cursor {
				fmt.Fprintf(&b, "\x1b[7m%s\x1b[0m\n", line)
			} else {
				b.WriteString(line + "\n")
			}
		}
		if len(rows) == 0 {
			b.WriteString("No matching folders or registered VMs.\n")
		}
		if m.Actions && len(rows) > 0 {
			b.WriteString("\n\x1b[1mProject actions\x1b[0m\n")
			actions := m.actions(rows[m.Cursor])
			visible := max(1, m.Height-10)
			first := max(0, m.ActionIndex-visible/2)
			first = min(first, max(0, len(actions)-visible))
			for i := first; i < min(len(actions), first+visible); i++ {
				a := actions[i]
				prefix := "  "
				if i == m.ActionIndex {
					prefix = "› "
				}
				b.WriteString(prefix + a[1] + "\n")
			}
		} else if len(rows) > 0 {
			if v := m.vm(rows[m.Cursor].Entry.Path); v != nil {
				fmt.Fprintf(&b, "\nSSH: %s\nStorage: %s\n", clean(v.SSHAlias), truncate(v.Storage.InstanceDirectory, m.Width-12))
				if v.Resources.CPUs != nil && v.Resources.MemoryBytes != nil {
					fmt.Fprintf(&b, "Resources: %d CPUs · %.1f GiB RAM", *v.Resources.CPUs, float64(*v.Resources.MemoryBytes)/(1<<30))
					if v.Resources.DiskMaxBytes != nil {
						fmt.Fprintf(&b, " · %.1f GiB disk maximum", float64(*v.Resources.DiskMaxBytes)/(1<<30))
					}
					b.WriteString("\n")
				}
				if v.ProfileID != nil {
					p := m.ProfileRecords[*v.ProfileID]
					fmt.Fprintf(&b, "Identity: %s · %s · %s\n", clean(p.Label), p.AgentManagement, p.LockState)
				} else {
					b.WriteString("Identity: no Git profile\n")
				}
				if len(v.OperationIDs) > 0 {
					b.WriteString("Next: resume the pending operation from Actions.\n")
				}
				if len(m.DetailLines) == 0 {
					b.WriteString("Checking current state…\n")
				} else {
					for _, line := range m.DetailLines {
						b.WriteString(truncate(line, m.Width-2) + "\n")
					}
				}
			}
		}
	}
	if m.Filtering || m.Filter != "" {
		fmt.Fprintf(&b, "\nFilter: %s", clean(m.Filter))
		if m.Filtering {
			b.WriteString("▏")
		}
		b.WriteString("\n")
	}
	if m.Unavailable {
		b.WriteString("\nRegistry unavailable. Inventory, if shown, is diagnostic only.\n")
	}
	if m.Error != "" {
		b.WriteString("\n" + clean(m.Error) + "\n")
	}
	if m.Notice != "" {
		b.WriteString("\n" + clean(m.Notice) + "\n")
	}
	b.WriteString("\n↑↓ Move  → Expand  ← Collapse  Enter Actions  / Filter  Tab Views  ? Help  q Quit\n")
	view := tea.NewView(b.String())
	view.AltScreen = true
	return view
}
func Browse(ctx context.Context, root string, vms []model.VM, profiles []model.Profile, unavailable, hidden, dependencies bool, openAction string, in io.Reader, out io.Writer, notice string) (Selection, error) {
	m := New(ctx, root, vms, profiles, unavailable, hidden, dependencies)
	m.OpenAction = openAction
	m.Notice = notice
	final, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return Selection{}, err
	}
	return final.(Model).Result, nil
}

type choiceModel struct {
	Title     string
	Choices   []string
	Index     int
	Result    int
	Cancelled bool
	Height    int
}

func (c choiceModel) Init() tea.Cmd { return nil }
func (c choiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		c.Height = size.Height
		return c, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc", "q":
			c.Cancelled = true
			return c, tea.Quit
		case "up", "k":
			c.Index = max(0, c.Index-1)
		case "down", "j":
			c.Index = min(len(c.Choices)-1, c.Index+1)
		case "enter":
			c.Result = c.Index
			return c, tea.Quit
		}
	}
	return c, nil
}
func (c choiceModel) View() tea.View {
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[1;36measy-lima\x1b[0m  %s\n\n", clean(c.Title))
	count := max(1, c.Height-7)
	if c.Height == 0 {
		count = 20
	}
	start := max(0, min(c.Index-count/2, len(c.Choices)-count))
	for i := start; i < min(len(c.Choices), start+count); i++ {
		option := c.Choices[i]
		prefix := "  "
		if i == c.Index {
			prefix = "› "
		}
		b.WriteString(prefix + clean(option) + "\n")
	}
	b.WriteString("\n↑↓ Choose   Enter Select   Esc Cancel\n")
	return tea.NewView(b.String())
}
func Choose(ctx context.Context, title string, choices []string, in io.Reader, out io.Writer) (int, error) {
	if len(choices) == 0 {
		return -1, nil
	}
	result, err := tea.NewProgram(choiceModel{Title: title, Choices: choices, Result: -1}, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return -1, err
	}
	c := result.(choiceModel)
	if c.Cancelled {
		return -1, nil
	}
	return c.Result, nil
}

// BrowseSession keeps navigation preferences and expanded rows across actions.
func BrowseSession(ctx context.Context, previous *Model, root string, vms []model.VM, profiles []model.Profile, unavailable, hidden, deps bool, openAction string, in io.Reader, out io.Writer, notice string, details func(context.Context, string) []string) (Model, error) {
	var m Model
	if previous == nil {
		m = New(ctx, root, vms, profiles, unavailable, hidden, deps)
	} else {
		m = *previous
		m.Context = ctx
		m.VMs = vms
		m.VMIndex = vmIndex(vms)
		m.ProfileRecords = profileIndex(profiles)
		m.Profiles = map[string]string{}
		for _, p := range profiles {
			m.Profiles[p.ID] = p.Label
		}
		m.Unavailable = unavailable
		m.Result = Selection{}
		m.Actions = false
		m.DetailID = ""
		m.DetailLines = nil
	}
	m.OpenAction = openAction
	m.Notice = notice
	m.Details = details
	// Request details even when restoring a selected VM and no key is pressed.
	next, command := m.Update(tea.WindowSizeMsg{Width: m.Width, Height: m.Height})
	m = next.(Model)
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out))
	if command != nil {
		go func() { program.Send(command()) }()
	}
	final, err := program.Run()
	if err != nil {
		return m, err
	}
	m = final.(Model)
	if m.detailCancel != nil {
		m.detailCancel()
	}
	return m, nil
}
