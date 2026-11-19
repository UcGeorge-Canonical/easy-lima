package connection

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
)

func TestOpenSSHParsesManagedPathsWithSpaces(t *testing.T) {
	binary, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("OpenSSH unavailable")
	}
	dir := filepath.Join(t.TempDir(), "a path with spaces")
	e := Endpoint{Address: "127.0.0.1", Port: 2222, User: "developer", IdentityFile: filepath.Join(dir, "login key"), VMID: strings.Repeat("a", 32)}
	state := filepath.Join(dir, "state")
	args := append(SSHArgs(state, e), "-G", e.Address)
	r, err := (process.OS{}).Run(context.Background(), process.Request{Executable: binary, Args: args, Env: process.Environment(nil), Limit: 64 << 10})
	if err != nil || r.ExitCode != 0 {
		t.Fatalf("parse: %v %s", err, r.Stderr)
	}
	// Quoted paths must remain one filename, not multiple known_hosts inputs.
	expected := "userknownhostsfile " + TrustPath(state, e.VMID)
	if !strings.Contains(string(r.Stdout), expected) {
		t.Fatalf("wrong trust destination: %s", r.Stdout)
	}
	path := filepath.Join(t.TempDir(), "ssh_config")
	body := append(AliasText("/Applications/easy lima", state, "easy-lima-fixture", e), []byte("Host *\n  ServerAliveInterval 37\n")...)
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	r, err = (process.OS{}).Run(context.Background(), process.Request{Executable: binary, Args: []string{"-F", path, "-G", "easy-lima-fixture"}, Env: process.Environment(nil), Limit: 64 << 10})
	if err != nil || r.ExitCode != 0 || !strings.Contains(string(r.Stdout), "stricthostkeychecking true") {
		t.Fatalf("unsafe alias: %v %s %s", err, r.Stdout, r.Stderr)
	}
}
