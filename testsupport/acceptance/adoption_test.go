//go:build acceptance

package acceptance

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/assets"
	"github.com/UcGeorge-Canonical/easy-lima/internal/app"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

func TestLegacyAdoption(t *testing.T) {
	root := os.Getenv("EASY_LIMA_ACCEPTANCE_ROOT")
	if root == "" || os.Getenv("EASY_LIMA_TEST_ADOPTION") != "1" {
		t.Skip("explicit disposable adoption fixture required")
	}
	if !filepath.IsAbs(root) || filepath.Base(root) != ".easy-lima-test" {
		t.Fatal("invalid disposable root")
	}
	temporaryRoot := os.TempDir()
	if runtime.GOOS == "darwin" {
		temporaryRoot = "/private/tmp"
	}
	home := os.Getenv("EASY_LIMA_ACCEPTANCE_HOME")
	if home == "" {
		home = filepath.Join(temporaryRoot, fmt.Sprintf("el-at-%d", os.Getuid()))
	}
	t.Setenv("HOME", home)
	paths, err := config.UserPaths()
	if err != nil {
		t.Fatal(err)
	}
	prototype := filepath.Join(root, "prototype")
	source := filepath.Join(prototype, "project")
	vmroot := filepath.Join(root, "vms")
	write := func(path string, data []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if err = os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	service := app.Service{Paths: paths, CWD: source, Binary: os.Getenv("EASY_LIMA_BINARY"), Runner: process.OS{}, In: bytes.NewReader(nil), Out: &output, Err: &output}
	ctx := context.Background()
	withGit := os.Getenv("EASY_LIMA_TEST_ADOPTION_GIT") == "1"
	keyPath := filepath.Join(root, "legacy-fixture-key")
	var fingerprint string
	if withGit {
		if _, e := os.Stat(keyPath); os.IsNotExist(e) {
			public, private, e := ed25519.GenerateKey(rand.Reader)
			if e != nil {
				t.Fatal(e)
			}
			block, e := ssh.MarshalPrivateKey(private, "disposable legacy fixture")
			if e != nil {
				t.Fatal(e)
			}
			write(keyPath, pem.EncodeToMemory(block))
			pub, e := ssh.NewPublicKey(public)
			if e != nil {
				t.Fatal(e)
			}
			write(keyPath+".pub", ssh.MarshalAuthorizedKey(pub))
		}
		key, e := identity.ParsePrivate(keyPath, nil)
		if e != nil {
			t.Fatal(e)
		}
		fingerprint = key.Fingerprint
		socket := filepath.Join(home, ".ssh", "canonical-work-agent.sock")
		if e = os.MkdirAll(filepath.Dir(socket), 0700); e != nil {
			t.Fatal(e)
		}
		if _, e = os.Lstat(socket); e == nil {
			t.Fatal("disposable legacy agent socket already exists; inspect the previous fixture before retry")
		}
		cmd := exec.Command("ssh-agent", "-D", "-a", socket)
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = os.Remove(socket) })
		var client *agent.Client
		for i := 0; i < 100; i++ {
			client, e = agent.Connect(ctx, socket)
			if e == nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if e != nil {
			t.Fatal(e)
		}
		if e = client.Agent.Add(sshagent.AddedKey{PrivateKey: key.Private}); e != nil {
			client.Close()
			t.Fatal(e)
		}
		client.Close()
	}
	run := func(r app.Request) model.Result {
		t.Helper()
		r.Yes = true
		r.PassphraseFD = -1
		result := service.Execute(ctx, r)
		if result.ExitCode != 0 {
			b, _ := json.Marshal(result)
			t.Fatalf("%s: %s", r.Command, b)
		}
		return result
	}
	run(app.Request{Command: "init", StorageRoot: vmroot, Limactl: "limactl", OpenAction: "ssh"})
	// The real launcher creates this inode on its first operation and retains it.
	lock := filepath.Join(prototype, ".local-state", "launcher.lock")
	if _, e := os.Stat(lock); os.IsNotExist(e) {
		write(lock, nil)
	}
	pending := run(app.Request{Command: "operations.list", Limit: 1000}).Data.(model.ListData[model.Operation])
	for _, op := range pending.Items {
		if op.Kind == "adopt" && op.Status != "completed" && op.Status != "failed_known" && op.Scope.Path != nil && *op.Scope.Path == source {
			run(app.Request{Command: "operations.resume", ID: op.ID})
		}
	}
	hash := sha256.Sum256([]byte(prototype + "\x00project"))
	name := "cl-project-" + hex.EncodeToString(hash[:])[:10]
	executable, err := config.Executable("limactl", source)
	if err != nil {
		t.Fatal(err)
	}
	adapter := lima.Adapter{Executable: executable, Root: vmroot, Temp: filepath.Join(vmroot, "_easy-lima", "temp", "adoption-test"), InitialBoot: true}
	listing := run(app.Request{Command: "list", Limit: 1000}).Data.(model.ListData[model.VM])
	var vm *model.VM
	for _, item := range listing.Items {
		if item.InstanceName == name {
			v := item
			vm = &v
		}
	}
	if vm == nil {
		observation, e := adapter.Observe(ctx, name)
		if e != nil {
			t.Fatal(e)
		}
		if observation.Status == "Absent" {
			ports := networkPorts(t, root, false)
			catalog, e := environment.LoadCatalog()
			if e != nil {
				t.Fatal(e)
			}
			image := catalog.Images[runtime.GOARCH]
			imagePath := storage.AssetPath(vmroot, image)
			if _, e = os.Stat(imagePath); e != nil {
				t.Fatal("run TestVMWorkflow first to populate the verified image cache:", e)
			}
			template, _ := assets.Read("ubuntu-24.04.yaml")
			native, _, e := lima.Prepare(template, config.Settings{Resources: config.Resources{CPUs: 2, Memory: "2GiB", Disk: "16GiB"}, Environment: config.Environment{Presets: []string{"base"}}, Ports: ports}, source, imagePath, image.SHA256)
			if e != nil {
				t.Fatal(e)
			}
			templatePath := filepath.Join(root, "legacy.yaml")
			write(templatePath, native)
			for _, action := range []string{"create", "start", "stop"} {
				result, e := adapter.Mutate(ctx, action, name, templatePath)
				if e != nil || result.ExitCode != 0 {
					t.Fatalf("legacy fixture %s: %v %s", action, e, result.Stderr)
				}
				if withGit && action == "start" {
					installed, e := (process.OS{}).Run(ctx, process.Request{Executable: executable, Args: []string{"shell", "--start=false", "--tty=false", name, "--", "sudo", "sh", "-c", "apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq git openssh-client python3"}, Env: process.Environment(map[string]string{"LIMA_HOME": vmroot}), Timeout: 5 * time.Minute, Limit: 8192})
					if e != nil || installed.ExitCode != 0 {
						t.Fatalf("legacy Git tools: %v %s", e, installed.Stderr)
					}
					configureLegacyGitFixture(t, executable, vmroot, name, keyPath)
				}
			}
		}
		fixture, e := os.ReadFile("../../testdata/prototype/v1/local.py")
		if e != nil {
			t.Fatal(e)
		}
		write(filepath.Join(prototype, ".local-launcher", "local.py"), fixture)
		gitConfig := "enabled = false\n"
		if withGit {
			gitConfig = fmt.Sprintf("enabled = true\nname = 'Legacy Fixture'\nemail = 'legacy@example.invalid'\ngithub_user = 'fixture-account'\nkey_path = %q\n", keyPath)
		}
		write(filepath.Join(prototype, "local-vms.toml"), []byte(fmt.Sprintf("[storage]\nlima_home = %q\n[git]\n%s", vmroot, gitConfig)))
		record, _ := json.Marshal(map[string]any{"name": name, "relative": "project", "source": source, "resources": map[string]any{"cpus": 2, "memory": "2GiB", "disk": "16GiB"}})
		write(filepath.Join(prototype, ".local-state", "projects", name+".json"), record)
		write(filepath.Join(prototype, ".local-state", "ssh", "config"), []byte(fmt.Sprintf("Include %q\n", filepath.Join(vmroot, name, "ssh.config"))))
		preview := run(app.Request{Command: "adopt", FromPrototype: prototype, Path: source, DryRun: true})
		if preview.Outcome != "planned" {
			t.Fatal("adoption did not preview")
		}
		run(app.Request{Command: "adopt", FromPrototype: prototype, Path: source})
		listing = run(app.Request{Command: "list", Limit: 1000}).Data.(model.ListData[model.VM])
		for _, item := range listing.Items {
			if item.InstanceName == name {
				v := item
				vm = &v
			}
		}
	}
	if vm == nil || vm.SSHAlias != "lima-"+name {
		t.Fatal("adoption lost the native name or alias")
	}
	for _, id := range vm.OperationIDs {
		run(app.Request{Command: "operations.resume", ID: id})
	}
	if _, e := os.Stat(filepath.Join(prototype, ".local-state", "projects", name+".json")); !os.IsNotExist(e) {
		t.Fatal("old owner was not retired")
	}
	disk := filepath.Join(vmroot, name, "disk")
	before, e := os.Stat(disk)
	if e != nil {
		t.Fatal(e)
	}
	run(app.Request{Command: "start", VMID: vm.ID})
	if withGit {
		fresh := run(app.Request{Command: "inspect", VMID: vm.ID}).Data.(map[string]any)["vm"].(model.VM)
		if fresh.ProfileID == nil || fresh.PendingIdentityRevisionID != nil {
			t.Fatal("adopted identity did not complete on start")
		}
		p := run(app.Request{Command: "profile.show", Label: *fresh.ProfileID}).Data.(map[string]any)["profile"].(model.Profile)
		if p.Fingerprint == nil || *p.Fingerprint != fingerprint || p.Git.Email != "legacy@example.invalid" || p.AgentManagement != "external" || !p.RememberEnabled {
			t.Fatal("adoption lost the legacy identity", p)
		}
		run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", `set -eu; p=$(mktemp -d); trap 'rm -rf "$p"' EXIT; git -C "$p" init -q; git -C "$p" commit --allow-empty -m adopted; git -C "$p" verify-commit HEAD`}})
	}
	run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", "test -d /workspace && test ! -e /var/lib/easy-lima/environments"}})
	verifyNetworks(t, service, vm, networkPorts(t, root, true))
	run(app.Request{Command: "stop", VMID: vm.ID})
	run(app.Request{Command: "start", VMID: vm.ID})
	run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", "test -d /workspace"}})
	run(app.Request{Command: "stop", VMID: vm.ID})
	after, e := os.Stat(disk)
	if e != nil || !os.SameFile(before, after) {
		t.Fatal("adoption replaced the disk")
	}
	run(app.Request{Command: "delete", VMID: vm.ID, ConfirmVM: vm.ID})
	if withGit {
		profiles := run(app.Request{Command: "profile.list"}).Data.(model.ListData[model.Profile])
		for _, p := range profiles.Items {
			if p.Fingerprint != nil && *p.Fingerprint == fingerprint {
				run(app.Request{Command: "profile.remove", Label: p.ID})
			}
		}
	}
}

func configureLegacyGitFixture(t *testing.T, executable, root, name, keyPath string) {
	t.Helper()
	// Use the captured prototype writer, not the new retirement implementation,
	// to create the legacy public settings that adoption must preserve/migrate.
	script, err := os.ReadFile("../../testdata/prototype/v1/git-guest.py")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(script)) != "6ee6e2a0ddf0d64f044c75efcb131a413f36705630476bf1c9c7c8868666ad1c" {
		t.Fatal("legacy Git fixture changed")
	}
	public, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	known, err := assets.Read("github-known-hosts")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"name": "Legacy Fixture", "email": "legacy@example.invalid", "github_user": "fixture-account", "public_key": strings.TrimSpace(string(public)), "known_hosts": string(known)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := (process.OS{}).Run(context.Background(), process.Request{Executable: executable, Args: []string{"shell", "--start=false", "--tty=false", name, "--", "python3", "-c", string(script), string(payload)}, Env: process.Environment(map[string]string{"LIMA_HOME": root}), Timeout: 90 * time.Second, Limit: 4096})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("legacy public Git setup: %v %s", err, result.Stderr)
	}
}
