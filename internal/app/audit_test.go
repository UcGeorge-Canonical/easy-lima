package app

import (
	"context"
	"fmt"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func shortTestRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "el-init-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

type probeRunner struct{ version string }

func (p probeRunner) Run(_ context.Context, r process.Request) (process.Result, error) {
	if len(r.Args) == 1 && r.Args[0] == "--version" {
		return process.Result{Stdout: []byte("limactl version " + p.version + "\n"), Exited: true}, nil
	}
	return process.Result{}, fmt.Errorf("unexpected subprocess %q", r.Args)
}
func TestInitPreflightAndConsentLeaveNoFiles(t *testing.T) {
	for _, tc := range []struct {
		name, version, open string
		decline             bool
		code                string
	}{
		{"unsupported", "0.1.0", "ssh", false, "dependency.unsupported"},
		{"invalid preference", "2.2.1", "invalid", false, "config.invalid"},
		{"declined", "2.2.1", "ssh", true, "operation.cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := shortTestRoot(t)
			paths := config.Paths{Home: root, Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache")}
			service := Service{Paths: paths, CWD: root, Runner: probeRunner{tc.version}, Out: io.Discard, Err: io.Discard, Confirm: func(model.Plan) (bool, error) { return false, nil }}
			result := service.Execute(context.Background(), Request{Command: "init", StorageRoot: filepath.Join(root, "vms"), Limactl: "/usr/bin/true", OpenAction: tc.open, Yes: !tc.decline, Interactive: tc.decline})
			if result.ExitCode == 0 {
				t.Fatal("invalid setup succeeded")
			}
			if len(result.Errors) != 1 || result.Errors[0].Code != tc.code {
				t.Fatalf("wrong setup failure: %+v", result.Errors)
			}
			for _, p := range []string{paths.Config, paths.State, filepath.Join(root, "vms")} {
				if _, e := os.Stat(p); !os.IsNotExist(e) {
					t.Fatalf("preflight changed %s", p)
				}
			}
		})
	}
}
func TestInitPreservesSelectedDirectoryPermissionsAndRecoversConfig(t *testing.T) {
	root := shortTestRoot(t)
	vmroot := filepath.Join(root, "vms")
	if e := os.Mkdir(vmroot, 0755); e != nil {
		t.Fatal(e)
	}
	paths := config.Paths{Home: root, Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache")}
	service := Service{Paths: paths, CWD: root, Runner: probeRunner{"2.2.1"}, Out: io.Discard, Err: io.Discard}
	r := Request{Command: "init", StorageRoot: vmroot, Limactl: "/usr/bin/true", OpenAction: "ssh", Yes: true}
	run := func() {
		result := service.Execute(context.Background(), r)
		if result.ExitCode != 0 {
			t.Fatal(result.Errors)
		}
	}
	run()
	info, _ := os.Stat(vmroot)
	if info.Mode().Perm() != 0755 {
		t.Fatal("changed existing directory permissions")
	}
	if e := os.Remove(filepath.Join(paths.Config, "config.toml")); e != nil {
		t.Fatal(e)
	}
	run()
	f, e := config.LoadUser(filepath.Join(paths.Config, "config.toml"))
	if e != nil || f.Value.Storage.Root != vmroot {
		t.Fatal("partial setup could not recover", e)
	}
}
func TestChangedIdentityFieldsPreserveOtherMemberValues(t *testing.T) {
	before := model.GitIdentity{Name: "Default", Email: "default@example.invalid", SignCommits: true, SignTags: true}
	after := before
	after.SignTags = false
	member := before
	member.Name = "Selective"
	member.Email = "selective@example.invalid"
	got := mergeIdentity(before, after, member)
	if got.Name != member.Name || got.Email != member.Email || got.SignTags || !got.SignCommits {
		t.Fatal(got)
	}
}
func TestPersistentPortAdmissionDetectsHostOccupancy(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	settings := []config.Port{{Local: port, Protocol: "tcp", HostAddress: "127.0.0.1"}}
	if e = availablePorts(settings); e == nil {
		t.Fatal("occupied port accepted")
	}
	listener.Close()
	if e = availablePorts(settings); e != nil {
		t.Fatal("free port rejected", e)
	}
}

func TestInitUnwritableExistingRootDoesNotPinConfiguration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission refusal requires an unprivileged process")
	}
	root := shortTestRoot(t)
	vmroot := filepath.Join(root, "read-only")
	if err := os.Mkdir(vmroot, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(vmroot, 0700) })
	paths := config.Paths{Home: root, Config: filepath.Join(root, "config"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache")}
	s := Service{Paths: paths, CWD: root, Runner: probeRunner{"2.2.1"}, Out: io.Discard, Err: io.Discard}
	r := s.Execute(context.Background(), Request{Command: "init", StorageRoot: vmroot, Limactl: "/usr/bin/true", Yes: true})
	if len(r.Errors) != 1 || r.Errors[0].Code != "storage.unavailable" {
		t.Fatal(r.Errors)
	}
	if _, err := os.Stat(filepath.Join(paths.Config, "config.toml")); !os.IsNotExist(err) {
		t.Fatal("unusable root was saved", err)
	}
	r = s.Execute(context.Background(), Request{Command: "init", StorageRoot: filepath.Join(root, "writable"), Limactl: "/usr/bin/true", Yes: true})
	if r.ExitCode != 0 {
		t.Fatal("retry with explicit writable root failed", r.Errors)
	}
}
