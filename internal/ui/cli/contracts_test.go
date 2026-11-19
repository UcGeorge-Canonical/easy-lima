package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func TestCommandGrammar(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	cases := []struct {
		args    []string
		command string
		code    int
	}{
		{[]string{"init", "--storage-root", "/chosen disk/vms", "--yes"}, "init", 0},
		{[]string{"up", "project", "--profile", "work", "--presets", "development,docker"}, "up", 0},
		{[]string{"delete", "--vm", id, "--confirm-vm", id, "--yes"}, "delete", 0},
		{[]string{"forward", "project", "8080", "--local", "18080"}, "forward", 0},
		{[]string{"exec", "project", "--start", "--", "printf", "%s", "literal; $(touch /nope)"}, "exec", 0},
		{[]string{"profile", "apply", "work", "--vm", id, "--vm", "11111111111111111111111111111111"}, "profile.apply", 0},
		{[]string{"state", "inventory", "refresh", "--vm", id}, "state.inventory.refresh", 0},
		{[]string{"environment", "update", "--vm", id, "--memory", "12GiB", "--apply", "--yes"}, "environment.update", 0},
		{[]string{"up", "--cpus", "0"}, "up", 2}, {[]string{"up", "--cpus", "4", "--cpus", "8"}, "up", 2},
		{[]string{"stop", "--profile", "work"}, "stop", 2}, {[]string{"exec", "--json", "--", "true"}, "exec", 2},
		{[]string{"profile", "import", "work", "--passphrase-fd", "0"}, "profile.import", 2},
		{[]string{"profile", "import", "work", "--passphrase", "secret"}, "", 2},
		{[]string{"up", "p", "--vm", id}, "up", 2}, {[]string{"up", "--profile", "work", "--no-profile"}, "up", 2},
		{[]string{"up", "--yes", "extra", "extra"}, "up", 2}, {[]string{"list", "--capture-output"}, "list", 2},
		{[]string{"environment", "update", "--from-project", "--memory", "4GiB"}, "environment.update", 2},
		{[]string{"logs", "--follow", "--json"}, "logs", 2}, {[]string{"state", "checkpoint", "--timeout", "61s"}, "state.checkpoint", 2},
	}
	for _, tc := range cases {
		t.Run(tc.command+"/"+stringMustJSON(tc.args), func(t *testing.T) {
			p, err := Parse(tc.args, t.TempDir())
			if tc.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if p.Request.Command != tc.command {
					t.Fatalf("command %q", p.Request.Command)
				}
			} else if err == nil || model.AsError(err).Exit != tc.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
func TestExecPreservesArgv(t *testing.T) {
	args := []string{"printf", "%s\n", "space ; $value", "--json", "--help"}
	p, err := Parse(append([]string{"exec", "--"}, args...), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, p.Request.GuestArgs) || p.Help || p.Request.JSON {
		t.Fatal("guest flags were interpreted by the host")
	}
}
func TestEveryCommandHasContract(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/interface/v1/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string][]string
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for name := range Specs {
		args, ok := cases[name]
		if !ok {
			t.Errorf("missing command fixture: %s", name)
			continue
		}
		p, e := Parse(args, t.TempDir())
		if e != nil {
			t.Errorf("%s: %v", name, e)
		} else if p.Request.Command != name {
			t.Errorf("%s parsed as %s", name, p.Request.Command)
		}
	}
}
func TestMountFileAnchorsAtItsDirectory(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	file := filepath.Join(dir, "mounts.toml")
	if err := os.WriteFile(file, []byte("schema_version = 1\n[[mounts]]\nsource = 'source with space:colon'\ntarget = '/extra'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Parse([]string{"up", "project", "--mounts-from", file}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if p.Request.Settings.Mounts[0].Source != filepath.Join(dir, "source with space:colon") {
		t.Fatal("mount did not anchor at its input file")
	}
}
func stringMustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestExplicitPathsAndBrowserSelectors(t *testing.T) {
	dir := t.TempDir()
	p, e := Parse([]string{"inspect", "--", "-folder with spaces"}, dir)
	if e != nil || p.Request.Path != "-folder with spaces" {
		t.Fatal(p, e)
	}
	p, e = Parse([]string{"--config", "/test/config.toml", "--limactl", "/test/limactl"}, dir)
	if e != nil || !p.Browse || p.Request.UserConfig != "/test/config.toml" {
		t.Fatal(p, e)
	}
	for _, command := range []string{"up", "start", "restart", "code", "ssh", "exec", "forward"} {
		args := []string{command, "--passphrase-fd", "3"}
		if command == "exec" {
			args = append(args, "--", "true")
		}
		if command == "forward" {
			args = append(args, "8080", "--local", "18080")
		}
		if _, e = Parse(args, dir); e != nil {
			t.Errorf("%s: %v", command, e)
		}
	}
	if _, e = Parse([]string{"up", "--passphrase-fd=-1"}, dir); e == nil {
		t.Fatal("explicit invalid fd accepted")
	}
	p, e = Parse([]string{"profile", "edit", "work", "--github-user="}, dir)
	if e != nil || !p.Request.GitHubUserSet || p.Request.GitHubUser != "" {
		t.Fatal("cannot explicitly clear expected account", e)
	}
}
