//go:build acceptance

// These tests create real, disposable VMs only in an explicitly supplied root.
package acceptance

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/app"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func TestVMWorkflow(t *testing.T) {
	root := os.Getenv("EASY_LIMA_ACCEPTANCE_ROOT")
	if root == "" {
		t.Skip("set EASY_LIMA_ACCEPTANCE_ROOT to a short, disposable directory")
	}
	if !filepath.IsAbs(root) || filepath.Base(root) != ".easy-lima-test" {
		t.Fatal("root must be absolute and end with .easy-lima-test")
	}
	if err := os.MkdirAll(filepath.Join(root, "project"), 0700); err != nil {
		t.Fatal(err)
	}
	// launchd requires a filesystem with enforced file ownership for its plist.
	// The disposable home is small; all VM data remains under the explicit root.
	temporaryRoot := os.TempDir()
	if runtime.GOOS == "darwin" {
		temporaryRoot = "/private/tmp"
	}
	t.Setenv("HOME", filepath.Join(temporaryRoot, fmt.Sprintf("el-at-%d", os.Getuid())))
	if os.Getenv("EASY_LIMA_ACCEPTANCE_HOME") != "" {
		t.Setenv("HOME", os.Getenv("EASY_LIMA_ACCEPTANCE_HOME"))
	}
	paths, e := config.UserPaths()
	if e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	s := app.Service{Paths: paths, CWD: filepath.Join(root, "project"), Binary: os.Getenv("EASY_LIMA_BINARY"), Runner: process.OS{}, In: bytes.NewReader(nil), Out: &output, Err: &output, Progress: func(stage string) { t.Log(stage) }}
	t.Setenv("HOME", s.Paths.Home)
	ctx := context.Background()
	run := func(r app.Request) model.Result {
		t.Helper()
		r.Yes = true
		r.PassphraseFD = -1
		out := s.Execute(ctx, r)
		if out.ExitCode != 0 {
			b, _ := json.Marshal(out)
			trace := output.String()
			if len(trace) > 4096 {
				trace = trace[len(trace)-4096:]
			}
			t.Fatalf("%s: %s\nGuest output: %s", r.Command, b, trace)
		}
		return out
	}
	run(app.Request{Command: "init", StorageRoot: filepath.Join(root, "vms"), Limactl: "limactl", OpenAction: "ssh"})
	var profileID string
	if os.Getenv("EASY_LIMA_TEST_PROFILE") == "1" {
		pending := run(app.Request{Command: "operations.list", Limit: 1000}).Data.(model.ListData[model.Operation])
		for _, op := range pending.Items {
			if op.Kind == "profile.import" && op.Status != "completed" && op.Status != "failed_known" {
				run(app.Request{Command: "operations.resume", ID: op.ID})
			}
		}
		profiles := run(app.Request{Command: "profile.list"}).Data.(model.ListData[model.Profile])
		for _, p := range profiles.Items {
			if p.Label == "acceptance" {
				profileID = p.ID
			}
		}
	}
	if os.Getenv("EASY_LIMA_TEST_PROFILE") == "1" && profileID == "" {
		public, private, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		block, e := ssh.MarshalPrivateKey(private, "easy-lima acceptance")
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(root, "test-key")
		if e = os.WriteFile(path, pem.EncodeToMemory(block), 0600); e != nil {
			t.Fatal(e)
		}
		pub, e := ssh.NewPublicKey(public)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path+".pub", ssh.MarshalAuthorizedKey(pub), 0600); e != nil {
			t.Fatal(e)
		}
		result := run(app.Request{Command: "profile.import", Label: "acceptance", Key: path, GitName: "Acceptance Developer", GitEmail: "acceptance@example.invalid"})
		profileID = result.Data.(model.MutationData).Profile.ID
	}
	var vm *model.VM
	listing := run(app.Request{Command: "list"}).Data.(model.ListData[model.VM])
	var existingMain *model.VM
	for _, item := range listing.Items {
		if item.SourcePath == s.CWD || strings.HasPrefix(filepath.Base(item.SourcePath), "moved-") {
			v := item
			existingMain = &v
			break
		}
	}
	if existingMain != nil {
		existing := *existingMain
		vm = &existing
		s.CWD = existing.SourcePath
		repairKey := os.Getenv("EASY_LIMA_TEST_VERIFIED_HOST_KEY")
		for _, id := range existing.OperationIDs {
			result := s.Execute(ctx, app.Request{Command: "operations.resume", ID: id, Yes: true, PassphraseFD: -1})
			if result.ExitCode != 0 && !(repairKey != "" && len(result.Errors) == 1 && result.Errors[0].Code == "trust.mismatch") {
				b, _ := json.Marshal(result)
				t.Fatalf("resume: %s", b)
			}
		}
		if repairKey != "" {
			// Explicit recovery fixture only: the public key is supplied by the
			// tester, never enrolled again by ordinary start or connection.
			run(app.Request{Command: "repair", VMID: existing.ID, RepairKind: "host-key", HostKeyFile: repairKey, Apply: true})
			run(app.Request{Command: "exec", VMID: existing.ID, GuestArgs: []string{"sudo", "python3", "-c", `from pathlib import Path
p=Path('/etc/cloud/cloud.cfg.d/99-easy-lima-host-keys.cfg')
if p.exists() and p.read_bytes()!=b'ssh_deletekeys: false\n': raise RuntimeError('conflict')
p.write_bytes(b'ssh_deletekeys: false\n')`}})
			run(app.Request{Command: "environment.update", VMID: existing.ID, Apply: true, RefreshTools: true})
		}
		run(app.Request{Command: "up", VMID: existing.ID})
	} else {
		presets := []string{"base"}
		if raw := os.Getenv("EASY_LIMA_TEST_PRESETS"); raw != "" {
			presets = strings.Split(raw, ",")
		}
		result := run(app.Request{Command: "up", Profile: profileID, NoProfile: profileID == "", CreationFlags: true, Settings: config.Settings{Resources: config.Resources{CPUs: 2, Memory: "6GiB", Disk: "40GiB"}, Environment: config.Environment{Presets: presets}}})
		vm = result.Data.(model.MutationData).VM
	}
	if vm == nil {
		t.Fatal("creation omitted VM")
	}
	if os.Getenv("EASY_LIMA_TEST_REFRESH") == "1" {
		run(app.Request{Command: "environment.update", VMID: vm.ID, Apply: true, RefreshTools: true})
	}
	output.Reset()
	run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", "test -d /workspace && test \"$(id -un)\" = developer && printf verified"}})
	if output.String() != "verified" {
		t.Fatalf("exec output: %q", output.String())
	}
	output.Reset()
	guestFailure := s.Execute(ctx, app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", "printf exact-output; exit 23"}, PassphraseFD: -1})
	if guestFailure.ExitCode != 23 || output.String() != "exact-output" {
		t.Fatalf("guest exit/stream passthrough: %d %q", guestFailure.ExitCode, output.String())
	}
	// A project/grouping folder is valid without a Git repository.
	verifyReadOnly(t, s, vm)
	run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"git", "init", "-q", "/workspace"}})
	verifyReadOnly(t, s, vm)
	if profileID != "" {
		run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"git", "-C", "/workspace", "config", "user.signingkey", "/fixture/wrong-public-key"}})
		check := s.Execute(ctx, app.Request{Command: "doctor", VMID: vm.ID, PassphraseFD: -1})
		if check.ExitCode != 7 {
			t.Fatalf("local signing override was not diagnosed: %+v", check)
		}
		started := run(app.Request{Command: "start", VMID: vm.ID})
		identityUnavailable := false
		for _, c := range started.Checks {
			if c.Code == "identity.readiness" && c.Status == "unavailable" {
				identityUnavailable = true
			}
		}
		if !identityUnavailable {
			t.Fatal("VM-only start hid the conflicting identity status")
		}
		// A conflicting repository setting gates identity readiness. The checkout
		// is host-mounted, so restore this test-owned override from the host.
		cleared, e := s.Runner.Run(ctx, process.Request{Executable: "git", Args: []string{"-C", s.CWD, "config", "--unset", "user.signingkey"}, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 4096})
		if e != nil || cleared.ExitCode != 0 {
			t.Fatal("remove test override", e)
		}
	}
	if profileID != "" {
		run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", `set -eu
p=$(mktemp -d)
trap 'rm -rf "$p"' EXIT
git -C "$p" init -q
git -C "$p" commit --allow-empty -m acceptance
git -C "$p" verify-commit HEAD
git -C "$p" tag -s -m acceptance verified
git -C "$p" verify-tag verified`}})
	}
	if profileID != "" {
		// Exercise the actual OpenSSH alias/ProxyCommand, including paths with spaces.
		alias, e := s.Runner.Run(ctx, process.Request{Executable: "ssh", Args: []string{"-F", filepath.Join(s.Paths.Home, ".ssh", "config"), "-o", "BatchMode=yes", vm.SSHAlias, "git config user.email"}, Env: process.Environment(nil), Timeout: 30 * time.Second, Limit: 4096})
		if e != nil || alias.ExitCode != 0 || strings.TrimSpace(string(alias.Stdout)) != "acceptance@example.invalid" {
			t.Fatalf("alias: %v %s %s", e, alias.Stdout, alias.Stderr)
		}
	}
	if strings.Contains(os.Getenv("EASY_LIMA_TEST_PRESETS"), "docker") {
		t.Log("Docker build, network, Compose, and restart")
		run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", `set -eu
p=$(mktemp -d)
project=elacceptance
trap 'docker compose --project-name "$project" -f "$p/compose.yaml" down --remove-orphans >/dev/null 2>&1 || true; docker image rm easy-lima-acceptance >/dev/null 2>&1 || true; rm -rf "$p"' EXIT
docker run --rm alpine:3.21 sh -c 'nslookup example.com >/dev/null && wget -qO /dev/null https://example.com'
printf 'FROM alpine:3.21\nRUN printf build-verified\n' > "$p/Dockerfile"
docker build -t easy-lima-acceptance "$p"
printf 'services:\n  check:\n    image: easy-lima-acceptance\n    command: ["true"]\n' > "$p/compose.yaml"
docker compose --project-name "$project" -f "$p/compose.yaml" up --abort-on-container-exit --exit-code-from check
sudo systemctl restart docker
docker run --rm alpine:3.21 true`}})
	}
	if strings.Contains(os.Getenv("EASY_LIMA_TEST_PRESETS"), "development") {
		t.Log("LXD container, network, and service restart")
		run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", `set -eu
name=el-acceptance
trap 'lxc delete --force "$name" >/dev/null 2>&1 || true' EXIT
lxc launch ubuntu:24.04 "$name"
ready=false
for attempt in $(seq 1 20); do
  if lxc exec "$name" -- sh -c 'getent hosts example.com >/dev/null && curl -fsS --max-time 10 https://example.com >/dev/null'; then ready=true; break; fi
  sleep 3
done
$ready
sudo snap restart lxd
ready=false
for attempt in $(seq 1 20); do
  if lxc exec "$name" -- sh -c 'getent hosts example.com >/dev/null && curl -fsS --max-time 10 https://example.com >/dev/null'; then ready=true; break; fi
  sleep 3
done
$ready
juju version`}})
	}
	if strings.Contains(os.Getenv("EASY_LIMA_TEST_PRESETS"), "development") && os.Getenv("EASY_LIMA_TEST_JUJU") == "1" {
		t.Log("Juju bootstrap, deploy, and cleanup")
		run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"bash", "-c", `set -eu
controller=el-acceptance
trap 'timeout --foreground 5m juju destroy-controller "$controller" --destroy-all-models --destroy-storage --no-prompt --force >/dev/null 2>&1 || true' EXIT
arch=$(dpkg --print-architecture)
juju bootstrap localhost "$controller" --bootstrap-base=ubuntu@24.04 --bootstrap-constraints="arch=$arch cores=1 mem=2G" --config test-mode=true
juju add-model acceptance
juju deploy ubuntu --channel=latest/stable --base=ubuntu@24.04 --constraints="arch=$arch cores=1 mem=512M"
juju wait-for application ubuntu --query='status=="active"' --timeout=10m
juju exec --application ubuntu -- hostname
juju destroy-model acceptance --destroy-storage --no-prompt
timeout --foreground 5m juju destroy-controller "$controller" --destroy-all-models --destroy-storage --no-prompt
trap - EXIT`}})
	}
	if profileID != "" && os.Getenv("EASY_LIMA_TEST_SHARED_PROFILE") == "1" {
		t.Log("Shared profile: two VMs, signing, lock/unlock, deferred identity update")
		secondPath := filepath.Join(root, "second-project")
		if e := os.MkdirAll(secondPath, 0700); e != nil {
			t.Fatal(e)
		}
		var second *model.VM
		current := run(app.Request{Command: "list", Limit: 1000}).Data.(model.ListData[model.VM])
		for _, item := range current.Items {
			if item.SourcePath == secondPath {
				v := item
				second = &v
				break
			}
		}
		if second != nil {
			for _, id := range second.OperationIDs {
				run(app.Request{Command: "operations.resume", ID: id})
			}
			run(app.Request{Command: "up", VMID: second.ID})
		} else {
			result := run(app.Request{Command: "up", Path: secondPath, Profile: profileID, CreationFlags: true, Settings: config.Settings{Resources: config.Resources{CPUs: 2, Memory: "2GiB", Disk: "16GiB"}, Environment: config.Environment{Presets: []string{"base"}}}})
			second = result.Data.(model.MutationData).VM
		}
		if second == nil {
			t.Fatal("second VM missing")
		}
		for _, id := range []string{vm.ID, second.ID} {
			run(app.Request{Command: "exec", VMID: id, GuestArgs: []string{"sh", "-c", `set -eu; p=$(mktemp -d); trap 'rm -rf "$p"' EXIT; git -C "$p" init -q; git -C "$p" commit --allow-empty -m shared; git -C "$p" verify-commit HEAD`}})
		}
		run(app.Request{Command: "profile.lock", Label: profileID})
		locked := s.Execute(ctx, app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"true"}, PassphraseFD: -1})
		if len(locked.Errors) != 1 || locked.Errors[0].Code != "profile.explicitly_locked" {
			t.Fatalf("locked profile: %+v", locked)
		}
		run(app.Request{Command: "profile.unlock", Label: profileID})
		for _, id := range []string{vm.ID, second.ID} {
			run(app.Request{Command: "exec", VMID: id, GuestArgs: []string{"git", "config", "user.email"}})
		}
		run(app.Request{Command: "stop", VMID: second.ID})
		run(app.Request{Command: "profile.edit", Label: profileID, GitName: "Updated Developer"})
		applied := s.Execute(ctx, app.Request{Command: "profile.apply", Label: profileID, Yes: true, PassphraseFD: -1})
		if applied.ExitCode != 0 && applied.ExitCode != 6 {
			t.Fatalf("profile apply: %+v", applied)
		}
		run(app.Request{Command: "start", VMID: second.ID})
		started := run(app.Request{Command: "inspect", VMID: second.ID})
		startedVM := started.Data.(map[string]any)["vm"].(model.VM)
		if startedVM.PendingIdentityRevisionID != nil {
			t.Fatal("start left an approved identity update pending")
		}
		for _, id := range []string{vm.ID, second.ID} {
			run(app.Request{Command: "exec", VMID: id, GuestArgs: []string{"sh", "-c", `test "$(git config user.name)" = 'Updated Developer'`}})
		}
		run(app.Request{Command: "delete", VMID: second.ID, ConfirmVM: second.ID})
	}
	run(app.Request{Command: "stop", VMID: vm.ID})
	run(app.Request{Command: "start", VMID: vm.ID})
	preview := run(app.Request{Command: "environment.update", VMID: vm.ID, Settings: config.Settings{Resources: config.Resources{CPUs: 3}}, CreationFlags: true})
	if preview.Outcome != "planned" {
		t.Fatalf("update preview outcome: %s", preview.Outcome)
	}
	run(app.Request{Command: "environment.update", VMID: vm.ID, Settings: config.Settings{Resources: config.Resources{CPUs: 3}}, CreationFlags: true, Apply: true})
	run(app.Request{Command: "stop", VMID: vm.ID})
	newPath := filepath.Join(root, "moved-"+vm.ID)
	if s.CWD != newPath {
		if err := os.Rename(s.CWD, newPath); err != nil {
			t.Fatal(err)
		}
	}
	run(app.Request{Command: "rebind", VMID: vm.ID, To: newPath})
	s.CWD = newPath
	run(app.Request{Command: "start", VMID: vm.ID})
	run(app.Request{Command: "exec", VMID: vm.ID, GuestArgs: []string{"sh", "-c", "test -d /workspace"}})
	run(app.Request{Command: "delete", VMID: vm.ID, ConfirmVM: vm.ID})
	if profileID != "" {
		run(app.Request{Command: "profile.remove", Label: profileID})
	}
}
