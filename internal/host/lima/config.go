package lima

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"go.yaml.in/yaml/v3"
)

type GlobalInput struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	State  string `json:"state"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
}

func Parse(b []byte) (*yaml.Node, error) {
	if len(b) > 8<<20 {
		return nil, model.Invalid("native YAML", "exceeds 8 MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	var n yaml.Node
	if err := d.Decode(&n); err != nil && err != io.EOF {
		return nil, model.Invalid("native YAML", "invalid YAML")
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, model.Invalid("native YAML", "exactly one document is required")
	}
	var check func(*yaml.Node, int) error
	check = func(n *yaml.Node, depth int) error {
		if depth > 64 {
			return model.Invalid("native YAML", "nesting exceeds 64 levels")
		}
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return model.Invalid("native YAML", "aliases and anchors are unsupported")
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || seen[k.Value] {
					return model.Invalid("native YAML", "duplicate or non-scalar mapping key")
				}
				seen[k.Value] = true
			}
		}
		for _, c := range n.Content {
			if err := check(c, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := check(&n, 0); err != nil {
		return nil, err
	}
	if len(n.Content) == 0 {
		n.Kind = yaml.DocumentNode
		n.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := n.Content[0]
	if root.Tag == "!!null" {
		root.Kind = yaml.MappingNode
		root.Tag = "!!map"
		root.Value = ""
	}
	if root.Kind != yaml.MappingNode {
		return nil, model.Invalid("native YAML", "a mapping is required")
	}
	return &n, nil
}
func ReadGlobals(root string, creation bool) ([]GlobalInput, error) {
	out := []GlobalInput{}
	for _, name := range []string{"default.yaml", "override.yaml", "base.yaml"} {
		role := "start"
		if name == "base.yaml" {
			role = "creation"
			if !creation {
				continue
			}
		}
		p := filepath.Join(root, "_config", name)
		in := GlobalInput{Path: p, Role: role, State: "absent"}
		b, err := files.Read(p, 1<<20)
		if os.IsNotExist(err) {
			out = append(out, in)
			continue
		}
		if err != nil {
			return nil, model.Fail("config.drift", fmt.Sprintf("Namespace input %q cannot be read as a regular file.", p))
		}
		n, err := Parse(b)
		if err != nil {
			return nil, err
		}
		if len(n.Content[0].Content) > 0 {
			return nil, model.Fail("dependency.unsupported", fmt.Sprintf("Namespace input %q contains semantic settings; v1 accepts only absent, empty or comment-only globals.", p))
		}
		in.State = "present_empty"
		in.Size = len(b)
		in.Digest = files.Hash(b)
		out = append(out, in)
	}
	return out, nil
}
func CheckGlobals(root string, recorded []GlobalInput, creation bool) error {
	now, err := ReadGlobals(root, creation)
	if err != nil {
		return err
	}
	for _, a := range now {
		found := false
		for _, b := range recorded {
			if a.Path == b.Path {
				found = true
				if a != b {
					return model.Fail("config.drift", "A consumed Lima namespace file changed; preview an explicit environment update.")
				}
			}
		}
		if !found {
			return model.Fail("config.drift", "The namespace input manifest is incomplete.")
		}
	}
	return nil
}
func get(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
func set(n *yaml.Node, key string, v any) error {
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return err
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			node.HeadComment = n.Content[i+1].HeadComment
			node.LineComment = n.Content[i+1].LineComment
			n.Content[i+1] = &node
			return nil
		}
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &node)
	return nil
}
func remove(n *yaml.Node, key string) {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content = append(n.Content[:i], n.Content[i+2:]...)
			return
		}
	}
}
func encode(n *yaml.Node) ([]byte, error) {
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	err := e.Encode(n)
	e.Close()
	return b.Bytes(), err
}

type Script struct {
	Mode   string `json:"mode" yaml:"mode"`
	Script string `json:"script" yaml:"script"`
}
type Image struct {
	Location string `yaml:"location" json:"location"`
	Arch     string `yaml:"arch" json:"arch"`
	Digest   string `yaml:"digest" json:"digest"`
}

// Prepare retains native nodes/comments while making every owned field explicit.
// Native provision scripts are extracted into frozen, explicit setup stages.
func Prepare(template []byte, s config.Settings, source, imagePath, imageDigest string) ([]byte, []Script, error) {
	n, err := Parse(template)
	if err != nil {
		return nil, nil, err
	}
	root := n.Content[0]
	scripts := []Script{}
	for _, key := range []string{"base", "networks", "additionalDisks", "firmware", "kernel", "initrd", "param", "probes", "copyToHost"} {
		if v := get(root, key); v != nil && v.Tag != "!!null" && (len(v.Content) > 0 || v.Value != "") {
			return nil, nil, model.Fail("dependency.unsupported", fmt.Sprintf("Native field %q requires an asset or execution contract that is not supported by this release.", key))
		}
	}
	if p := get(root, "provision"); p != nil && p.Tag != "!!null" {
		for _, item := range p.Content {
			if item.Kind != yaml.MappingNode {
				return nil, nil, model.Invalid("provision", "each item must be a mapping")
			}
			for i := 0; i < len(item.Content); i += 2 {
				k := item.Content[i].Value
				if k != "mode" && k != "script" {
					return nil, nil, model.Invalid("provision", "unsupported script field: "+k)
				}
			}
		}
		if err = p.Decode(&scripts); err != nil {
			return nil, nil, model.Invalid("provision", "expected complete inline scripts")
		}
		for _, script := range scripts {
			if script.Mode != "system" && script.Mode != "user" && script.Mode != "" {
				return nil, nil, model.Invalid("provision.mode", "only one-time system/user scripts are supported")
			}
			if script.Script == "" || len(script.Script) > 65536 {
				return nil, nil, model.Invalid("provision.script", "a bounded inline script is required")
			}
		}
	}
	remove(root, "provision")
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	driver := "qemu"
	mountType := "9p"
	if runtime.GOOS == "darwin" {
		driver = "vz"
		mountType = "virtiofs"
	}
	mounts := []map[string]any{{"location": source, "mountPoint": "/workspace", "writable": true}}
	for _, m := range s.Mounts {
		mounts = append(mounts, map[string]any{"location": m.Source, "mountPoint": m.Target, "writable": m.Writable})
	}
	ports := []map[string]any{}
	for _, p := range s.Ports {
		ports = append(ports, map[string]any{"guestIP": p.GuestAddress, "guestPort": p.Guest, "hostIP": p.HostAddress, "hostPort": p.Local, "proto": p.Protocol})
	}
	ports = append(ports, map[string]any{"guestIP": "0.0.0.0", "guestIPMustBeZero": false, "proto": "any", "guestPortRange": []int{1, 65535}, "ignore": true})
	fields := []struct {
		k string
		v any
	}{{"minimumLimaVersion", ContractVersion}, {"vmType", driver}, {"arch", arch}, {"cpus", s.Resources.CPUs}, {"memory", s.Resources.Memory}, {"disk", s.Resources.Disk}, {"images", []Image{{Location: imagePath, Arch: arch, Digest: "sha256:" + imageDigest}}}, {"mountType", mountType}, {"mounts", mounts}, {"portForwards", ports}, {"containerd", map[string]bool{"system": false, "user": false}}, {"ssh", map[string]any{"forwardAgent": false, "loadDotSSHPubKeys": false, "overVsock": false, "localPort": 0}}, {"propagateProxyEnv", false}, {"rosetta", map[string]bool{"enabled": false, "binfmt": false}}}
	for _, f := range fields {
		if f.k == "mounts" {
			if err = patchMounts(root, mounts); err != nil {
				return nil, nil, err
			}
			continue
		}
		if err = set(root, f.k, f.v); err != nil {
			return nil, nil, err
		}
	}
	b, err := encode(n)
	return b, scripts, err
}
func Patch(data []byte, s config.Settings, source string) ([]byte, error) {
	n, err := Parse(data)
	if err != nil {
		return nil, err
	}
	root := n.Content[0]
	if err = set(root, "cpus", s.Resources.CPUs); err != nil {
		return nil, err
	}
	if err = set(root, "memory", s.Resources.Memory); err != nil {
		return nil, err
	}
	mounts := []map[string]any{{"location": source, "mountPoint": "/workspace", "writable": true}}
	for _, m := range s.Mounts {
		mounts = append(mounts, map[string]any{"location": m.Source, "mountPoint": m.Target, "writable": m.Writable})
	}
	if err = patchMounts(root, mounts); err != nil {
		return nil, err
	}
	ports := []map[string]any{}
	for _, p := range s.Ports {
		ports = append(ports, map[string]any{"guestIP": p.GuestAddress, "guestPort": p.Guest, "hostIP": p.HostAddress, "hostPort": p.Local, "proto": p.Protocol})
	}
	ports = append(ports, map[string]any{"guestIP": "0.0.0.0", "guestIPMustBeZero": false, "proto": "any", "guestPortRange": []int{1, 65535}, "ignore": true})
	if err = set(root, "portForwards", ports); err != nil {
		return nil, err
	}
	return encode(n)
}

// Retain native per-mount options when changing resources or a mount's basic
// properties. New and removed targets still follow the explicit requested list.
func patchMounts(root *yaml.Node, mounts []map[string]any) error {
	previous := get(root, "mounts")
	next := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, mount := range mounts {
		var item *yaml.Node
		if previous != nil {
			for _, candidate := range previous.Content {
				target := get(candidate, "mountPoint")
				if target != nil && target.Value == mount["mountPoint"] {
					item = candidate
					break
				}
			}
		}
		if item == nil {
			item = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		for _, field := range []string{"location", "mountPoint", "writable"} {
			if err := set(item, field, mount[field]); err != nil {
				return err
			}
		}
		next.Content = append(next.Content, item)
	}
	remove(root, "mounts")
	root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mounts"}, next)
	return nil
}
func NativeSettings(b []byte) (config.Settings, []Image, error) {
	n, err := Parse(b)
	if err != nil {
		return config.Settings{}, nil, err
	}
	root := n.Content[0]
	s := config.Settings{}
	var images []Image
	for _, v := range []struct {
		k   string
		dst any
	}{{"cpus", &s.Resources.CPUs}, {"memory", &s.Resources.Memory}, {"disk", &s.Resources.Disk}, {"images", &images}} {
		if p := get(root, v.k); p != nil && p.Tag != "!!null" {
			if err = p.Decode(v.dst); err != nil {
				return s, nil, model.Invalid(v.k, "invalid native value")
			}
		}
	}
	if p := get(root, "mounts"); p != nil && p.Tag != "!!null" {
		var mounts []struct {
			Location string `yaml:"location"`
			Target   string `yaml:"mountPoint"`
			Writable bool   `yaml:"writable"`
		}
		if err = p.Decode(&mounts); err != nil {
			return s, nil, err
		}
		s.Mounts = []config.Mount{}
		for _, m := range mounts {
			if m.Target == "/workspace" {
				continue
			}
			if m.Target == "" {
				m.Target = m.Location
			}
			s.Mounts = append(s.Mounts, config.Mount{Source: m.Location, Target: m.Target, Writable: m.Writable})
		}
	}
	if p := get(root, "portForwards"); p != nil && p.Tag != "!!null" {
		s.Ports = []config.Port{}
		for _, rule := range p.Content {
			if x := get(rule, "ignore"); x != nil && x.Value == "true" {
				continue
			}
			var port struct {
				Guest    int    `yaml:"guestPort"`
				Local    int    `yaml:"hostPort"`
				Host     string `yaml:"hostIP"`
				Address  string `yaml:"guestIP"`
				Protocol string `yaml:"proto"`
			}
			if err = rule.Decode(&port); err != nil {
				return s, nil, err
			}
			s.Ports = append(s.Ports, config.Port{Guest: port.Guest, Local: port.Local, HostAddress: port.Host, GuestAddress: port.Address, Protocol: port.Protocol})
		}
	}
	return s, images, nil
}
func CheckNative(b []byte) error {
	n, err := Parse(b)
	if err != nil {
		return err
	}
	root := n.Content[0]
	for _, k := range []string{"networks", "additionalDisks", "provision", "base", "probes", "copyToHost"} {
		v := get(root, k)
		if v != nil && v.Tag != "!!null" && (len(v.Content) > 0 || v.Value != "") {
			return model.Fail("dependency.unsupported", fmt.Sprintf("Native %s is not a frozen supported input.", k))
		}
	}
	ssh := get(root, "ssh")
	if ssh == nil {
		return model.Invalid("ssh", "explicit transport settings are required")
	}
	for _, k := range []string{"forwardAgent", "loadDotSSHPubKeys", "overVsock"} {
		v := get(ssh, k)
		if v == nil || v.Value != "false" {
			return model.Invalid("ssh."+k, "must be explicitly false")
		}
	}
	p := get(root, "portForwards")
	if p == nil || len(p.Content) == 0 {
		return model.Invalid("portForwards", "a final suppression rule is required")
	}
	for i, item := range p.Content {
		if i == len(p.Content)-1 {
			var m map[string]any
			if err = item.Decode(&m); err != nil {
				return err
			}
			v := get(item, "guestPortRange")
			if get(item, "ignore") == nil || get(item, "ignore").Value != "true" || get(item, "proto") == nil || get(item, "proto").Value != "any" || get(item, "guestIP") == nil || get(item, "guestIP").Value != "0.0.0.0" || get(item, "guestIPMustBeZero") == nil || get(item, "guestIPMustBeZero").Value != "false" || v == nil || len(v.Content) != 2 || v.Content[0].Value != "1" || v.Content[1].Value != "65535" {
				return model.Invalid("portForwards", "the last rule must disable automatic forwarding for both protocols")
			}
		} else {
			host := get(item, "hostIP")
			if host == nil || (host.Value != "127.0.0.1" && host.Value != "::1") {
				return model.Invalid("portForwards.hostIP", "must be explicit loopback")
			}
			for _, k := range []string{"guestPortRange", "hostPortRange", "guestSocket", "hostSocket"} {
				if get(item, k) != nil {
					return model.Invalid("portForwards", "ranges and socket mappings are not application port rules")
				}
			}
			if get(item, "guestPort") == nil || get(item, "hostPort") == nil {
				return model.Invalid("portForwards", "each allow rule must name exact guest and host ports")
			}
		}
	}
	return nil
}
func CheckPathLength(root, name string) error {
	// OpenSSH creates a temporary control socket beside ssh.sock. Lima 2.2.1
	// checks the same 16-character suffix before creating an instance.
	limit := 104
	if runtime.GOOS == "linux" {
		limit = 108
	}
	if len(filepath.Join(root, name, "ssh.sock.1234567890123456")) >= limit {
		return model.Invalid("storage.root", "the native Lima control socket path would exceed the platform limit")
	}
	return nil
}

func InstanceName(root, slug, id string) (string, error) {
	if !model.ValidID(id) {
		return "", model.Fail("input.invalid", "Invalid VM identifier.")
	}
	for len(slug) > 0 {
		name := "el-" + strings.TrimRight(slug, "-") + "-" + id[:10]
		if CheckPathLength(root, name) == nil {
			return name, nil
		}
		slug = slug[:len(slug)-1]
	}
	name := "el-" + id[:10]
	return name, CheckPathLength(root, name)
}
func HasPreset(s config.Settings, name string) bool {
	for _, p := range s.Environment.Presets {
		if p == name {
			return true
		}
	}
	return false
}
func SafeInstancePath(root, name string) bool {
	return validName(name) && !strings.HasPrefix(name, "_") && filepath.Dir(filepath.Join(root, name)) == filepath.Clean(root)
}

// PatchWorkspace preserves every native field and nested mount option except the owned source.
func PatchWorkspace(data []byte, source string) ([]byte, error) {
	n, err := Parse(data)
	if err != nil {
		return nil, err
	}
	mounts := get(n.Content[0], "mounts")
	found := false
	if mounts != nil {
		for _, m := range mounts.Content {
			target := get(m, "mountPoint")
			if target != nil && target.Value == "/workspace" {
				if found {
					return nil, model.Invalid("mounts", "duplicate workspace target")
				}
				found = true
				if err = set(m, "location", source); err != nil {
					return nil, err
				}
			}
		}
	}
	if !found {
		return nil, model.Invalid("mounts", "the owned /workspace mount is missing")
	}
	return encode(n)
}

// SameCreationConfig compares the parsed frozen document with Lima's created
// document, allowing formatting and comments but no unreviewed semantic edits.
func SameCreationConfig(frozen, actual []byte) error {
	a, err := Parse(frozen)
	if err != nil {
		return err
	}
	b, err := Parse(actual)
	if err != nil {
		return err
	}
	var av, bv any
	if err = a.Decode(&av); err != nil {
		return err
	}
	if err = b.Decode(&bv); err != nil {
		return err
	}
	if !reflect.DeepEqual(av, bv) {
		return model.Fail("config.drift", "The created instance differs from its frozen native configuration; preserve it and inspect the recorded operation.")
	}
	return nil
}
