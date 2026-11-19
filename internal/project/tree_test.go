package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLazyChildrenFilteringAndCycles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"project", "node_modules", ".hidden"} {
		if e := os.Mkdir(filepath.Join(root, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Symlink(root, filepath.Join(root, "cycle")); e != nil {
		t.Fatal(e)
	}
	rows, e := Children(context.Background(), root, []string{root}, TreeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2 || !rows[0].Cycle || rows[1].Name != "project" {
		t.Fatalf("unexpected children: %#v", rows)
	}
	rows, e = Children(context.Background(), root, []string{root}, TreeOptions{Hidden: true, Dependencies: true})
	if e != nil || len(rows) != 4 {
		t.Fatal(rows, e)
	}
}
