// Package config validates TOML intent without initializing or mutating state.
package config

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Environment struct {
	Template string   `toml:"template,omitempty" json:"template"`
	Presets  []string `toml:"presets" json:"presets"`
}
type Resources struct {
	CPUs   int    `toml:"cpus,omitzero" json:"cpus"`
	Memory string `toml:"memory,omitempty" json:"memory"`
	Disk   string `toml:"disk,omitempty" json:"disk"`
}
type Mount struct {
	Source   string `toml:"source" json:"source"`
	Target   string `toml:"target" json:"target"`
	Writable bool   `toml:"writable" json:"writable"`
}
type Port struct {
	Guest        int    `toml:"guest" json:"guest"`
	Local        int    `toml:"local" json:"local"`
	Protocol     string `toml:"protocol" json:"protocol"`
	HostAddress  string `toml:"host_address" json:"host_address"`
	GuestAddress string `toml:"guest_address" json:"guest_address"`
}
type Settings struct {
	Environment Environment `toml:"environment" json:"environment"`
	Resources   Resources   `toml:"resources" json:"resources"`
	Mounts      []Mount     `toml:"mounts" json:"mounts"`
	Ports       []Port      `toml:"ports" json:"ports"`
}
type User struct {
	SchemaVersion int `toml:"schema_version"`
	Storage       struct {
		Root string `toml:"root"`
	} `toml:"storage"`
	Runtime struct {
		Limactl string `toml:"limactl"`
	} `toml:"runtime"`
	Defaults Settings `toml:"defaults"`
	UI       struct {
		OpenAction       string `toml:"open_action"`
		ShowHidden       bool   `toml:"show_hidden"`
		ShowDependencies bool   `toml:"show_dependencies"`
	} `toml:"ui"`
	Editor struct {
		Executable string   `toml:"executable"`
		Args       []string `toml:"args"`
	} `toml:"editor"`
}
type Project struct {
	SchemaVersion int         `toml:"schema_version"`
	Environment   Environment `toml:"environment"`
	Resources     Resources   `toml:"resources"`
	Identity      struct {
		ProfileLabel string `toml:"profile_label"`
	} `toml:"identity"`
	Mounts []Mount `toml:"mounts"`
	Ports  []Port  `toml:"ports"`
}
type Profile struct {
	SchemaVersion int               `toml:"schema_version"`
	ID            string            `toml:"id"`
	Label         string            `toml:"label"`
	Git           model.GitIdentity `toml:"git"`
	GitHub        struct {
		Username string `toml:"username"`
	} `toml:"github"`
	Key struct {
		Path        string `toml:"path"`
		PublicPath  string `toml:"public_path"`
		Fingerprint string `toml:"fingerprint"`
	} `toml:"key"`
	Unlock struct {
		Remember      bool   `toml:"remember"`
		CredentialRef string `toml:"credential_ref"`
	} `toml:"unlock"`
}
type File[T any] struct {
	Value        T
	Path, Digest string
	Metadata     toml.MetaData
}

func Read[T any](path string, initial T) (File[T], error) {
	f := File[T]{Value: initial, Path: path}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return f, err
	}
	directory, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return f, err
	}
	path = filepath.Join(directory, filepath.Base(absolute))
	f.Path = path
	b, err := files.Read(path, 1<<20)
	if err != nil {
		return f, err
	}
	f.Digest = files.Hash(b)
	md, err := toml.Decode(string(b), &f.Value)
	if err != nil {
		return f, model.Invalid(path, err.Error())
	}
	var bounded func(reflect.Value) bool
	bounded = func(v reflect.Value) bool {
		switch v.Kind() {
		case reflect.String:
			return len(v.String()) <= 64<<10
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if !bounded(v.Field(i)) {
					return false
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				if !bounded(v.Index(i)) {
					return false
				}
			}
		}
		return true
	}
	if !bounded(reflect.ValueOf(f.Value)) {
		return f, model.Invalid(path, "a string exceeds 64 KiB")
	}
	f.Metadata = md
	if keys := md.Undecoded(); len(keys) > 0 {
		return f, model.Fail("config.unknown_field", fmt.Sprintf("%s: unknown field %s", path, keys[0]))
	}
	var head struct {
		Schema int `toml:"schema_version"`
	}
	_, _ = toml.Decode(string(b), &head)
	if head.Schema != 1 {
		return f, model.Fail("config.unsupported_schema", fmt.Sprintf("%s: schema_version must be 1", path))
	}
	return f, nil
}
func DefaultUser() User {
	var u User
	u.SchemaVersion = 1
	u.UI.OpenAction = "code"
	u.Editor.Args = []string{"--remote", "ssh-remote+{alias}", "{workspace}"}
	return u
}
func LoadUser(path string) (File[User], error) {
	f, err := Read(path, DefaultUser())
	if err != nil {
		return f, err
	}
	if f.Metadata.IsDefined("defaults", "resources", "cpus") && f.Value.Defaults.Resources.CPUs == 0 {
		return f, model.Invalid("defaults.resources.cpus", "must be positive")
	}
	base := filepath.Dir(f.Path)
	u := &f.Value
	if u.Storage.Root != "" {
		u.Storage.Root, err = ResolvePath(u.Storage.Root, base)
		if err != nil {
			return f, err
		}
	}
	if u.Runtime.Limactl != "" && strings.Contains(u.Runtime.Limactl, "/") {
		u.Runtime.Limactl, err = ResolvePath(u.Runtime.Limactl, base)
		if err != nil {
			return f, err
		}
	}
	if u.UI.OpenAction != "code" && u.UI.OpenAction != "ssh" {
		return f, model.Invalid("ui.open_action", "must be code or ssh")
	}
	if u.Editor.Executable != "" && strings.Contains(u.Editor.Executable, "/") {
		u.Editor.Executable, err = ResolvePath(u.Editor.Executable, base)
		if err != nil {
			return f, err
		}
	}
	for _, a := range u.Editor.Args {
		if len(a) > 65536 || strings.ContainsRune(a, 0) {
			return f, model.Invalid("editor.args", "invalid argument")
		}
		s := strings.ReplaceAll(strings.ReplaceAll(a, "{alias}", ""), "{workspace}", "")
		if strings.ContainsAny(s, "{}") {
			return f, model.Invalid("editor.args", "only {alias} and {workspace} placeholders are supported")
		}
	}
	err = NormalizeSettings(&u.Defaults, base, false)
	return f, err
}
func LoadProject(path string) (File[Project], error) {
	f, err := Read(path, Project{})
	if err != nil {
		return f, err
	}
	if f.Metadata.IsDefined("resources", "cpus") && f.Value.Resources.CPUs == 0 {
		return f, model.Invalid("resources.cpus", "must be positive")
	}
	p := &f.Value
	s := Settings{p.Environment, p.Resources, p.Mounts, p.Ports}
	err = NormalizeSettings(&s, filepath.Dir(f.Path), false)
	p.Environment, p.Resources, p.Mounts, p.Ports = s.Environment, s.Resources, s.Mounts, s.Ports
	if p.Identity.ProfileLabel != "" && !labelPattern.MatchString(p.Identity.ProfileLabel) {
		return f, model.Invalid("identity.profile_label", "invalid profile label")
	}
	return f, err
}

var quantityPattern = regexp.MustCompile(`^([1-9][0-9]*)(MiB|GiB|TiB)$`)
var labelPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)
var githubPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

func Quantity(s string) (int64, error) {
	m := quantityPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, model.Invalid("quantity", "expected an integer followed by MiB, GiB or TiB")
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	shift := uint(20)
	if m[2] == "GiB" {
		shift = 30
	}
	if m[2] == "TiB" {
		shift = 40
	}
	if err != nil || n > math.MaxInt64>>shift {
		return 0, model.Invalid("quantity", "overflows 64-bit bytes")
	}
	return n << shift, nil
}
func Presets(values []string) ([]string, error) {
	if values == nil {
		return nil, nil
	}
	out := []string{"base"}
	seen := map[string]bool{}
	for _, p := range values {
		if p != "base" && p != "development" && p != "docker" {
			return nil, model.Invalid("environment.presets", "unknown preset")
		}
		if seen[p] {
			return nil, model.Invalid("environment.presets", "duplicate preset")
		}
		seen[p] = true
	}
	for _, p := range []string{"development", "docker"} {
		if seen[p] {
			out = append(out, p)
		}
	}
	return out, nil
}
func NormalizeSettings(s *Settings, base string, complete bool) error {
	var err error
	if s.Environment.Template != "" && s.Environment.Template != "ubuntu-24.04" {
		s.Environment.Template, err = ResolvePath(s.Environment.Template, base)
		if err != nil {
			return err
		}
	}
	s.Environment.Presets, err = Presets(s.Environment.Presets)
	if err != nil {
		return err
	}
	if s.Resources.CPUs < 0 || s.Resources.CPUs > 1024 {
		return model.Invalid("resources.cpus", "must be between 1 and 1024")
	}
	if complete && s.Resources.CPUs == 0 {
		return model.Invalid("resources.cpus", "must be positive")
	}
	for k, v := range map[string]string{"memory": s.Resources.Memory, "disk": s.Resources.Disk} {
		if v != "" {
			if _, err = Quantity(v); err != nil {
				return model.Invalid("resources."+k, err.Error())
			}
		}
	}
	if len(s.Mounts) > 256 || len(s.Ports) > 256 {
		return model.Invalid("mounts/ports", "at most 256 entries are allowed")
	}
	for i := range s.Mounts {
		m := &s.Mounts[i]
		m.Source, err = ResolvePath(m.Source, base)
		if err != nil {
			return err
		}
		if !files.PathOK(m.Target) || m.Target == "/" || Overlaps(m.Target, "/workspace") {
			return model.Invalid("mount.target", "must be an absolute guest directory outside /workspace")
		}
		for j := 0; j < i; j++ {
			if Overlaps(m.Target, s.Mounts[j].Target) {
				return model.Invalid("mounts", "guest targets overlap")
			}
		}
	}
	seen := map[string]bool{}
	for i := range s.Ports {
		p := &s.Ports[i]
		if p.Protocol == "" {
			p.Protocol = "tcp"
		}
		if p.HostAddress == "" {
			p.HostAddress = "127.0.0.1"
		}
		if p.GuestAddress == "" {
			p.GuestAddress = "127.0.0.1"
		}
		if p.Guest < 1 || p.Guest > 65535 || p.Local < 1 || p.Local > 65535 || p.Guest == 22 {
			return model.Invalid("ports", "ports must be 1–65535; guest SSH port 22 is reserved")
		}
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			return model.Invalid("ports.protocol", "must be tcp or udp")
		}
		if p.HostAddress != "127.0.0.1" && p.HostAddress != "::1" {
			return model.Invalid("ports.host_address", "only literal loopback addresses are permitted")
		}
		if net.ParseIP(p.GuestAddress) == nil {
			return model.Invalid("ports.guest_address", "must be a literal IP address")
		}
		key := fmt.Sprintf("%s/%s/%d", p.Protocol, p.HostAddress, p.Local)
		if seen[key] {
			return model.Invalid("ports", "duplicate host endpoint")
		}
		seen[key] = true
	}
	return nil
}
func Overlaps(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	return a == b || strings.HasPrefix(a, b+string(os.PathSeparator)) || strings.HasPrefix(b, a+string(os.PathSeparator)) || a == "/" || b == "/"
}
func ValidateProfile(p Profile) error {
	if p.SchemaVersion != 1 || !model.ValidID(p.ID) {
		return model.Invalid("profile.id", "invalid schema or immutable ID")
	}
	if !labelPattern.MatchString(p.Label) {
		return model.Invalid("profile.label", "use lowercase letters, digits and hyphens, starting with a letter (1–48 characters)")
	}
	if p.Git.Name == "" || p.Git.Name != strings.TrimSpace(p.Git.Name) || len(p.Git.Name) > 256 || strings.ContainsFunc(p.Git.Name, unicode.IsControl) {
		return model.Invalid("git.name", "an explicit trimmed name without control characters is required")
	}
	a, err := mail.ParseAddress(p.Git.Email)
	if err != nil || a.Address != p.Git.Email || a.Name != "" || len(p.Git.Email) > 254 || strings.ContainsAny(p.Git.Email, " \t\r\n()<>") {
		return model.Invalid("git.email", "an explicit single email address is required")
	}
	if p.GitHub.Username != "" && !githubPattern.MatchString(p.GitHub.Username) {
		return model.Invalid("github.username", "invalid GitHub account name")
	}
	if !files.PathOK(p.Key.Path) {
		return model.Invalid("key.path", "an absolute host key path is required")
	}
	return nil
}
func LoadProfile(path string) (File[Profile], error) {
	p := Profile{}
	p.Git.SignCommits = true
	p.Git.SignTags = true
	f, err := Read(path, p)
	if err != nil {
		return f, err
	}
	p = f.Value
	p.Key.Path, err = ResolvePath(p.Key.Path, filepath.Dir(f.Path))
	if err != nil {
		return f, err
	}
	if p.Key.PublicPath == "" {
		p.Key.PublicPath = p.Key.Path + ".pub"
	} else {
		p.Key.PublicPath, err = ResolvePath(p.Key.PublicPath, filepath.Dir(f.Path))
		if err != nil {
			return f, err
		}
	}
	f.Value = p
	if filepath.Base(path) != p.ID+".toml" {
		return f, model.Invalid("profile.id", "does not match the filename")
	}
	return f, ValidateProfile(p)
}
func Encode(v any) ([]byte, error) {
	var b bytes.Buffer
	err := toml.NewEncoder(&b).Encode(v)
	return b.Bytes(), err
}

// Merge replaces lists as complete requests; nil distinguishes absence from [].
func Merge(a, b Settings) Settings {
	if b.Environment.Template != "" {
		a.Environment.Template = b.Environment.Template
	}
	if b.Environment.Presets != nil {
		a.Environment.Presets = b.Environment.Presets
	}
	if b.Resources.CPUs != 0 {
		a.Resources.CPUs = b.Resources.CPUs
	}
	if b.Resources.Memory != "" {
		a.Resources.Memory = b.Resources.Memory
	}
	if b.Resources.Disk != "" {
		a.Resources.Disk = b.Resources.Disk
	}
	if b.Mounts != nil {
		a.Mounts = b.Mounts
	}
	if b.Ports != nil {
		a.Ports = b.Ports
	}
	return a
}
func Shipped() Settings {
	return Settings{Environment: Environment{Template: "ubuntu-24.04", Presets: []string{"base"}}, Resources: Resources{CPUs: 4, Memory: "8GiB", Disk: "80GiB"}, Mounts: []Mount{}, Ports: []Port{}}
}
