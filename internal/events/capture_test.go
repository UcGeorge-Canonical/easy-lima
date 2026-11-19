package events

import (
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureRedactsAcrossWritesAndBoundsOutput(t *testing.T) {
	dir := t.TempDir()
	id := model.NewID()
	c, e := Open(dir, id)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"normal output\npass", "word=do-not-store\n", "-----BEGIN OPENSSH PRIVATE KEY-----\nprivate-key-bytes\n-----END OPENSSH PRIVATE KEY-----\n", "safe again\n", strings.Repeat("z", 100000) + "\n"} {
		if _, e = c.Write([]byte(p)); e != nil {
			t.Fatal(e)
		}
	}
	if e = c.Close(); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(dir, id+".raw.log"))
	if e != nil {
		t.Fatal(e)
	}
	text := string(b)
	if !strings.Contains(text, "normal output\n") || !strings.Contains(text, "safe again\n") {
		t.Fatal("lost eligible output")
	}
	for _, secret := range []string{"do-not-store", "private-key-bytes", strings.Repeat("z", 100)} {
		if strings.Contains(text, secret) {
			t.Fatalf("retained %q", secret[:10])
		}
	}
	if len(b) > 200 {
		t.Fatalf("unbounded line: %d", len(b))
	}
}
