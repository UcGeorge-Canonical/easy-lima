package lima

import (
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"strings"
	"testing"
)

func TestPreparedTemplateHasNoAmbientExposure(t *testing.T) {
	settings := config.Shipped()
	settings.Ports = []config.Port{{Guest: 8080, Local: 18080, Protocol: "tcp", HostAddress: "127.0.0.1", GuestAddress: "127.0.0.1"}}
	b, scripts, e := Prepare([]byte("# custom template\nprovision:\n  - mode: user\n    script: echo hello\n"), settings, "/projects/a", "/vm/assets/image", strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	if len(scripts) != 1 || scripts[0].Mode != "user" {
		t.Fatal("lost custom stage")
	}
	if e = CheckNative(b); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "provision:") || strings.Contains(string(b), "location: ~") {
		t.Fatal("retained automatic provision or home mount")
	}
	if !strings.Contains(string(b), "ignore: true") || !strings.Contains(string(b), "proto: any") {
		t.Fatal("automatic forwarding remains enabled")
	}
	edited := strings.Replace(string(b), "hostIP: 127.0.0.1", "hostIP: 0.0.0.0", 1)
	if e = CheckNative([]byte(edited)); e == nil {
		t.Fatal("accepted public listener")
	}
}
func TestRejectsExecutableAndAmbiguousNativeInputs(t *testing.T) {
	for _, b := range []string{"x: &anchor {}\ny: *anchor\n", "cpus: 2\ncpus: 3\n", "a: 1\n---\nb: 2\n"} {
		if _, e := Parse([]byte(b)); e == nil {
			t.Fatalf("accepted %q", b)
		}
	}
	for _, b := range []string{"base: https://example.com/remote.yaml\n", "kernel: /tmp/kernel\n", "provision:\n - mode: boot\n   script: echo untracked\n"} {
		if _, _, e := Prepare([]byte(b), config.Shipped(), "/project", "/image", strings.Repeat("b", 64)); e == nil {
			t.Fatalf("accepted %q", b)
		}
	}
}
func TestNativeSettingsDoNotResetWrapperIntent(t *testing.T) {
	got, _, e := NativeSettings([]byte("cpus: 6\nmemory: 12GiB\n"))
	if e != nil {
		t.Fatal(e)
	}
	if got.Environment.Template != "" || got.Environment.Presets != nil || got.Ports != nil || got.Mounts != nil || got.Resources.Disk != "" {
		t.Fatal("absent native settings replaced wrapper intent")
	}
}
func TestDecodeCompleteObservation(t *testing.T) {
	got, e := Decode([]byte(`{"name":"a","status":"Stopped","dir":"/root/a"}`))
	if e != nil || !got.Complete || len(got.Items) != 1 {
		t.Fatal(got, e)
	}
	got, e = Decode([]byte(`[{"name":"a","status":"FutureState","dir":"/root/a"}]`))
	if e != nil || got.Complete {
		t.Fatal("unknown native state accepted")
	}
	if _, e = Decode([]byte(`{"name":`)); e == nil {
		t.Fatal("partial JSON treated as absence")
	}
}

func TestInstanceNameAccountsForOpenSSHTemporarySocket(t *testing.T) {
	root := "/" + strings.Repeat("v", 51)
	id := strings.Repeat("a", 32)
	name, err := InstanceName(root, "a-very-long-project-name", id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, id[:10]) || CheckPathLength(root, name) != nil {
		t.Fatal("lost stable suffix or exceeded socket limit")
	}
	if _, err = InstanceName("/"+strings.Repeat("v", 100), "project", id); err == nil {
		t.Fatal("accepted an unusable storage root")
	}
}

func TestResourceUpdateAndRebindPreserveNativeMountOptions(t *testing.T) {
	input := []byte("cpus: 4\nmemory: 8GiB\nmounts:\n - location: /old\n   mountPoint: /workspace\n   writable: true\n   9p:\n     cache: mmap\n - location: /data\n   mountPoint: /data\n   writable: false\n   9p:\n     cache: none\n")
	moved, e := PatchWorkspace(input, "/new")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(moved), "cache: mmap") || !strings.Contains(string(moved), "cache: none") {
		t.Fatal("rebind discarded mount options")
	}
	settings := config.Shipped()
	settings.Resources.CPUs = 2
	settings.Mounts = []config.Mount{{Source: "/data", Target: "/data"}}
	updated, e := Patch(moved, settings, "/new")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(updated), "cache: mmap") || !strings.Contains(string(updated), "cache: none") {
		t.Fatal("resource update discarded mount options")
	}
	if e = SameCreationConfig(updated, append([]byte("# native formatting\n"), updated...)); e != nil {
		t.Fatal(e)
	}
	if e = SameCreationConfig(updated, []byte(strings.Replace(string(updated), "cpus: 2", "cpus: 3", 1))); e == nil {
		t.Fatal("unreviewed resource change accepted")
	}
}
