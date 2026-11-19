package tui

import (
	"context"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"io"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type TaskHooks struct {
	Output   io.Writer
	Progress func(string)
	Confirm  func(string, string) (bool, error)
	Secret   func(string) ([]byte, error)
}
type taskLine struct {
	Text     string
	Progress bool
}
type taskAnswer struct {
	Approved bool
	Secret   []byte
}
type taskPrompt struct {
	Title, Body string
	Secret      bool
	Answer      chan taskAnswer
}
type taskDone model.Result
type taskModel struct {
	Title                 string
	Width, Height, Scroll int
	Lines                 []string
	Stage                 string
	Prompt                *taskPrompt
	Input                 []rune
	Approve, Cancelling   bool
	Result                model.Result
	cancel                context.CancelFunc
}

func (m taskModel) Init() tea.Cmd { return nil }
func (m taskModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
	case taskLine:
		if msg.Progress {
			m.Stage = clean(msg.Text)
		} else {
			for _, line := range strings.Split(msg.Text, "\n") {
				if line != "" {
					m.Lines = append(m.Lines, truncate(line, max(1, m.Width-4)))
				}
			}
			if len(m.Lines) > 50 {
				m.Lines = append([]string{}, m.Lines[len(m.Lines)-50:]...)
			}
		}
	case taskPrompt:
		m.Prompt = &msg
		m.Input = nil
		m.Approve = false
		m.Scroll = 0
	case taskDone:
		m.Result = model.Result(msg)
		return m, tea.Quit
	case tea.PasteMsg:
		if m.Prompt != nil && m.Prompt.Secret {
			for _, r := range msg.Content {
				if r != '\n' && r != '\r' && r != 0 && len(m.Input) < 65536 {
					m.Input = append(m.Input, r)
				}
			}
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			clear(m.Input)
			m.Input = nil
			m.Cancelling = true
			m.cancel()
			if m.Prompt != nil {
				m.Prompt.Answer <- taskAnswer{}
				m.Prompt = nil
			}
			return m, nil
		}
		if m.Prompt == nil {
			return m, nil
		}
		if m.Prompt.Secret {
			switch key {
			case "esc":
				clear(m.Input)
				m.Input = nil
				m.Prompt.Answer <- taskAnswer{}
				m.Prompt = nil
			case "enter":
				m.Prompt.Answer <- taskAnswer{Approved: true, Secret: []byte(string(m.Input))}
				for i := range m.Input {
					m.Input[i] = 0
				}
				m.Input = nil
				m.Prompt = nil
			case "backspace":
				if len(m.Input) > 0 {
					m.Input[len(m.Input)-1] = 0
					m.Input = m.Input[:len(m.Input)-1]
				}
			case "ctrl+u":
				for i := range m.Input {
					m.Input[i] = 0
				}
				m.Input = nil
			default:
				if len([]rune(key)) == 1 && len(m.Input) < 65536 {
					m.Input = append(m.Input, []rune(key)[0])
				} else if key == "space" {
					m.Input = append(m.Input, ' ')
				}
			}
		} else {
			switch key {
			case "up":
				m.Scroll = max(0, m.Scroll-1)
			case "down":
				m.Scroll++
			case "pgup":
				m.Scroll = max(0, m.Scroll-max(1, m.Height-10))
			case "pgdown":
				m.Scroll += max(1, m.Height-10)
			case "tab", "left", "right":
				m.Approve = !m.Approve
			case "y":
				m.Approve = true
			case "n":
				m.Approve = false
			case "esc":
				m.Prompt.Answer <- taskAnswer{}
				m.Prompt = nil
			case "enter":
				m.Prompt.Answer <- taskAnswer{Approved: m.Approve}
				m.Prompt = nil
			}
		}
	}
	return m, nil
}
func (m taskModel) View() tea.View {
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[1;36measy-lima\x1b[0m  %s\n\n", clean(m.Title))
	if m.Prompt != nil {
		fmt.Fprintf(&b, "%s\n\n", clean(m.Prompt.Title))
		if m.Prompt.Secret {
			fmt.Fprintf(&b, "Passphrase: %s▏\n\nEnter Continue   Ctrl-U Clear   Esc Cancel\n", strings.Repeat("•", min(len(m.Input), 60)))
		} else {
			lines := []string{}
			for _, line := range strings.Split(m.Prompt.Body, "\n") {
				lines = append(lines, strings.Split(ansi.Wrap(clean(line), max(1, m.Width-2), ""), "\n")...)
			}
			count := max(1, m.Height-10)
			start := min(m.Scroll, max(0, len(lines)-count))
			for _, line := range lines[start:min(len(lines), start+count)] {
				b.WriteString(line + "\n")
			}
			if len(lines) > count {
				fmt.Fprintf(&b, "\nLines %d–%d of %d · ↑↓ / PgUp/PgDn scroll\n", start+1, min(len(lines), start+count), len(lines))
			}
			choice := "[No]  Yes"
			if m.Approve {
				choice = " No  [Yes]"
			}
			fmt.Fprintf(&b, "\nApply this plan? %s\nTab / ←→ Choose   Enter Confirm   Esc Decline\n", choice)
		}
	} else {
		if m.Cancelling {
			b.WriteString("Cancelling foreground work; reconciling any dispatched action…\n")
		} else {
			b.WriteString("Working…\n")
		}
		if m.Stage != "" {
			b.WriteString(m.Stage + "\n")
		}
		b.WriteString("\nRecent output (saved only with --capture-output):\n")
		count := max(1, m.Height-12)
		for _, line := range m.Lines[max(0, len(m.Lines)-count):] {
			b.WriteString("  " + line + "\n")
		}
		b.WriteString("\nCtrl-C Cancel · running VMs and profile agents remain available\n")
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

type taskWriter struct {
	p  *tea.Program
	mu sync.Mutex
}

func (w *taskWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	original := len(b)
	for len(b) > 0 {
		n := min(len(b), 8192)
		w.p.Send(taskLine{Text: string(b[:n])})
		b = b[n:]
	}
	return original, nil
}

// RunTask owns terminal input throughout an action, including consent and secrets.
// Cancellation waits for the coordinator's final observation before leaving.
func RunTask(ctx context.Context, title string, in io.Reader, out io.Writer, execute func(context.Context, TaskHooks) model.Result) (model.Result, error) {
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := taskModel{Title: title, Width: 100, Height: 30, cancel: cancel}
	// The work context controls effects; do not terminate the UI before reconciliation finishes.
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
	ask := func(title, body string, secret bool) (taskAnswer, error) {
		reply := make(chan taskAnswer, 1)
		p.Send(taskPrompt{title, body, secret, reply})
		select {
		case a := <-reply:
			if !a.Approved {
				return a, model.Fail("operation.cancelled", "The action was declined.")
			}
			return a, nil
		case <-workCtx.Done():
			return taskAnswer{}, model.Fail("operation.cancelled", "The action was cancelled.")
		}
	}
	hooks := TaskHooks{Output: &taskWriter{p: p}, Progress: func(line string) { p.Send(taskLine{line, true}) }, Confirm: func(title, body string) (bool, error) { a, e := ask(title, body, false); return a.Approved, e }, Secret: func(label string) ([]byte, error) { a, e := ask(label, "", true); return a.Secret, e }}
	done := make(chan model.Result, 1)
	go func() { result := execute(workCtx, hooks); done <- result; p.Send(taskDone(result)) }()
	final, err := p.Run()
	if err != nil {
		cancel()
		return <-done, err
	}
	return final.(taskModel).Result, nil
}
