//go:build contracts

package lima

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledLimaMergesOnlyFrozenLocalBases(t *testing.T) {
	cli, err := exec.LookPath("limactl")
	if err != nil {
		t.Skip("installed Lima required for native merge qualification")
	}
	if _, err = (Adapter{Executable: cli}).Probe(context.Background()); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	child := filepath.Join(dir, "child.yaml")
	for path, text := range map[string]string{base: "cpus: 2\nmemory: 2GiB\nprovision:\n- mode: user\n  script: echo base-script\n", child: "base:\n- base.yaml\ncpus: 3\n"} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := Template(context.Background(), cli, child)
	if err != nil {
		t.Fatal(err)
	}
	settings, _, err := NativeSettings(raw)
	if err != nil || settings.Resources.CPUs != 3 || settings.Resources.Memory != "2GiB" || !strings.Contains(string(raw), "base-script") {
		t.Fatalf("native merge lost base semantics: %v %s", err, raw)
	}
	if err := os.WriteFile(base, []byte("base:\n- child.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Template(context.Background(), cli, child); err == nil {
		t.Fatal("accepted a base cycle")
	}
}
