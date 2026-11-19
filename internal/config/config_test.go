package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictFilesAndAnchoring(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	path := filepath.Join(dir, "easy-lima.toml")
	for _, body := range []string{"schema_version = 1\n[resources]\ncpus = 0\n", "schema_version = 2\n", "schema_version=1\nunknown=true\n", "schema_version=1\nschema_version=1\n", "schema_version=1\n[resources]\ncpus=-1\n"} {
		if e := os.WriteFile(path, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := LoadProject(path); e == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	body := "schema_version=1\n[[mounts]]\nsource='./other'\ntarget='/opt/other'\n"
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := LoadProject(path)
	if e != nil {
		t.Fatal(e)
	}
	if f.Value.Mounts[0].Source != filepath.Join(dir, "other") {
		t.Fatal("relative source used CWD instead of file directory")
	}
}

func TestCanonicalDeclaringDirectoryAndStringLimit(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "easy-lima.toml")
	if err := os.WriteFile(path, []byte("schema_version=1\n[environment]\ntemplate='./base.yaml'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := LoadProject(filepath.Join(alias, "easy-lima.toml"))
	if err != nil {
		t.Fatal(err)
	}
	realDir, _ = filepath.EvalSymlinks(realDir)
	if f.Path != filepath.Join(realDir, "easy-lima.toml") || f.Value.Environment.Template != filepath.Join(realDir, "base.yaml") {
		t.Fatal("relative template did not anchor to canonical declaring directory", f)
	}
	if err := os.WriteFile(path, []byte("schema_version=1\nprofile='"+strings.Repeat("x", 65537)+"'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadProject(path); err == nil {
		t.Fatal("accepted oversized configuration string")
	}
}
func TestEmptyListsReplaceWithoutAliasing(t *testing.T) {
	base := Shipped()
	base.Ports = []Port{{Guest: 8080, Local: 8080}}
	got := Merge(base, Settings{Ports: []Port{}})
	if got.Ports == nil || len(got.Ports) != 0 || len(base.Ports) != 1 {
		t.Fatal("empty-list precedence changed")
	}
}
func TestDefaultUserRoundTripOmitsUnsetNumbers(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	b, e := Encode(DefaultUser())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = LoadUser(p); e != nil {
		t.Fatal(e)
	}
}
func TestUnsafePortsAndMounts(t *testing.T) {
	for _, s := range []Settings{
		{Ports: []Port{{Guest: 22, Local: 2022}}},
		{Ports: []Port{{Guest: 8080, Local: 8080, HostAddress: "0.0.0.0"}}},
		{Mounts: []Mount{{Source: "/tmp", Target: "/workspace/nested"}}},
		{Ports: []Port{{Guest: 8080, Local: 8080}, {Guest: 9090, Local: 8080}}},
	} {
		if e := NormalizeSettings(&s, t.TempDir(), false); e == nil {
			t.Fatalf("accepted unsafe settings: %#v", s)
		}
	}
}

func TestDocumentedProjectExamples(t *testing.T) {
	paths, err := filepath.Glob("../../docs/examples/*.toml")
	if err != nil || len(paths) < 3 {
		t.Fatal("missing public examples", err)
	}
	for _, path := range paths {
		path, err = filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		f, e := LoadProject(path)
		if e != nil {
			t.Fatalf("%s: %v", path, e)
		}
		s := Merge(Shipped(), Settings{Environment: f.Value.Environment, Resources: f.Value.Resources, Mounts: f.Value.Mounts, Ports: f.Value.Ports})
		if e = NormalizeSettings(&s, filepath.Dir(path), true); e != nil {
			t.Fatalf("%s: %v", path, e)
		}
	}
}
