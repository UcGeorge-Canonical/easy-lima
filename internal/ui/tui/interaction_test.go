package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoragePickerSelectsWithoutCreatingAndCanGoBack(t *testing.T) {
	root := t.TempDir()
	m := storageModel{ctx: context.Background(), cwd: root, width: 90, height: 25, locations: []storage.Location{{Label: "Test drive", Path: root, AvailableBytes: 30 << 30}}}
	step := func(msg tea.Msg) tea.Cmd { next, cmd := m.Update(msg); m = next.(storageModel); return cmd }
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	step(enter) // location -> folders; defaults to New storage folder.
	step(storageLoaded{Path: root})
	step(enter)
	step(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.inputKind != "" || !m.folders {
		t.Fatal("Esc did not return to folder choices")
	}
	step(enter)
	step(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	step(tea.PasteMsg{Content: "vm disks"})
	step(enter)
	if m.result != filepath.Join(root, "vm disks") {
		t.Fatalf("selected %q", m.result)
	}
	if _, err := os.Stat(m.result); !os.IsNotExist(err) {
		t.Fatal("picker created storage before preview approval")
	}
}

func TestQueuedTreeLoadPreservesSelectedActionTarget(t *testing.T) {
	id := model.NewID()
	m := New(context.Background(), "/root", []model.VM{{ID: id, SourcePath: "/root/B"}}, nil, false, false, false)
	m.Rows = []row{{Entry: project.Entry{Path: "/root/A"}, Expanded: true}, {Entry: project.Entry{Path: "/root/B"}}}
	m.Cursor, m.Actions = 1, true
	next, _ := m.Update(loaded{Epoch: m.Epoch, Path: "/root/A", Entries: []project.Entry{{Path: "/root/A/child"}}})
	m = next.(Model)
	if m.currentRows()[m.Cursor].Entry.Path != "/root/B" {
		t.Fatal("queued load changed selected project")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next.(Model).Result.VMID != id {
		t.Fatal("action targeted another VM")
	}
}
func TestPlanPreviewWrapsEveryExposure(t *testing.T) {
	body := "/mounted/volume/" + strings.Repeat("long-source/", 15) + "END_FIRST\n/second/writable/mount END_SECOND"
	m := taskModel{Width: 80, Height: 40, Prompt: &taskPrompt{Title: "Preview", Body: body}}
	view := m.View().Content
	if !strings.Contains(view, "END_FIRST") || !strings.Contains(view, "END_SECOND") {
		t.Fatal("preview hid an exposure", view)
	}
	if !strings.Contains(view, "END_FIRST\n/second") {
		t.Fatal("lost separate plan field lines", view)
	}
}

func TestStorageReloadClampsSelection(t *testing.T) {
	m := storageModel{ctx: context.Background(), path: "/test", folders: true, cursor: 6, hidden: true, entries: []project.Entry{{Name: "a"}, {Name: "b"}, {Name: ".hidden"}}}
	next, _ := m.Update(storageLoaded{Path: "/test", Entries: []project.Entry{{Name: "a"}}})
	m = next.(storageModel)
	if m.cursor != 4 {
		t.Fatal("selection remained outside visible folders", m.cursor)
	}
	next, _ = m.Update(storageLoaded{Path: "/test"})
	m = next.(storageModel)
	if m.cursor != 3 {
		t.Fatal("selection outside empty folder controls", m.cursor)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if next.(storageModel).folders {
		t.Fatal("clamped location control did not work")
	}
}

func TestTaskConsentAndSecretPaste(t *testing.T) {
	cancelled := false
	m := taskModel{Title: "Create VM", Width: 90, Height: 25, cancel: func() { cancelled = true }}
	step := func(msg tea.Msg) { next, _ := m.Update(msg); m = next.(taskModel) }
	answer := make(chan taskAnswer, 1)
	step(taskPrompt{Title: "Review changes", Body: "Disk: /test/vms", Answer: answer})
	step(tea.KeyPressMsg{Code: tea.KeyEnter})
	if (<-answer).Approved {
		t.Fatal("destructive consent defaulted to yes")
	}
	step(taskPrompt{Title: "Unlock", Secret: true, Answer: answer})
	step(tea.PasteMsg{Content: "fixture passphrase"})
	if strings.Contains(m.View().Content, "fixture passphrase") {
		t.Fatal("secret rendered in terminal")
	}
	step(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := <-answer; string(got.Secret) != "fixture passphrase" || !got.Approved {
		t.Fatal("paste lost")
	}
	if len(m.Input) != 0 {
		t.Fatal("secret input retained after submission")
	}
	step(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !cancelled || !m.Cancelling {
		t.Fatal("cancel did not reach coordinator")
	}
}
