// Package cli implements the versioned command grammar without invoking effects
// during parsing. In particular, secret descriptors are not read here.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/app"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Spec struct {
	Usage    string
	Flags    string
	Path     bool
	Label    bool
	ID       bool
	ReadOnly bool
	Raw      bool
}

var Specs = map[string]Spec{
	"init":  {Usage: "init --storage-root PATH", Flags: "storage-root open-action editor"},
	"up":    {Usage: "up [PATH]", Flags: "vm profile no-profile template presets cpus memory disk mounts-from ports-from", Path: true},
	"start": {Usage: "start [PATH]", Flags: "vm", Path: true}, "stop": {Usage: "stop [PATH]", Flags: "vm", Path: true}, "restart": {Usage: "restart [PATH]", Flags: "vm", Path: true}, "delete": {Usage: "delete [PATH] --confirm-vm ID", Flags: "vm confirm-vm", Path: true},
	"code": {Usage: "code [PATH]", Flags: "vm no-start", Path: true, Raw: true}, "ssh": {Usage: "ssh [PATH]", Flags: "vm no-start", Path: true, Raw: true}, "exec": {Usage: "exec [PATH] [--start] -- COMMAND ARG…", Flags: "vm start", Path: true, Raw: true}, "forward": {Usage: "forward [PATH] GUEST_PORT --local HOST_PORT", Flags: "vm local bind", Path: true, Raw: true},
	"list": {Usage: "list [--profile LABEL]", Flags: "profile limit cursor", ReadOnly: true}, "inspect": {Usage: "inspect [PATH]", Flags: "vm", Path: true, ReadOnly: true}, "doctor": {Usage: "doctor [PATH] [--online]", Flags: "vm online repository", Path: true, ReadOnly: true}, "logs": {Usage: "logs [PATH]", Flags: "vm follow limit cursor", Path: true, ReadOnly: true},
	"environment.update": {Usage: "environment update [PATH] [--apply]", Flags: "vm apply template presets cpus memory disk mounts-from ports-from refresh-tools from-project accept-native-config", Path: true}, "rebind": {Usage: "rebind --vm ID --to PATH", Flags: "vm to"}, "adopt": {Usage: "adopt --from-prototype PATH --project PATH", Flags: "from-prototype project"}, "repair": {Usage: "repair [PATH] [--kind KIND --apply]", Flags: "vm kind apply host-key-file", Path: true},
	"profile.list": {Usage: "profile list", Flags: "limit cursor", ReadOnly: true}, "profile.show": {Usage: "profile show LABEL", Label: true, ReadOnly: true}, "profile.check": {Usage: "profile check LABEL", Flags: "online", Label: true, ReadOnly: true},
	"profile.create": {Usage: "profile create LABEL --git-name NAME --git-email EMAIL", Flags: "git-name git-email github-user sign-commits sign-tags remember passphrase-fd", Label: true}, "profile.import": {Usage: "profile import LABEL --key PATH --git-name NAME --git-email EMAIL", Flags: "key public-key git-name git-email github-user sign-commits sign-tags remember passphrase-fd", Label: true}, "profile.edit": {Usage: "profile edit LABEL", Flags: "key public-key git-name git-email github-user sign-commits sign-tags remember label", Label: true}, "profile.apply": {Usage: "profile apply LABEL [--vm ID]", Flags: "vm passphrase-fd", Label: true}, "profile.assign": {Usage: "profile assign LABEL --vm ID", Flags: "vm passphrase-fd", Label: true}, "profile.unassign": {Usage: "profile unassign --vm ID", Flags: "vm"}, "profile.migrate-agent": {Usage: "profile migrate-agent LABEL", Flags: "passphrase-fd", Label: true}, "profile.unlock": {Usage: "profile unlock LABEL", Flags: "passphrase-fd", Label: true}, "profile.lock": {Usage: "profile lock LABEL", Label: true}, "profile.forget": {Usage: "profile forget LABEL", Label: true}, "profile.remove": {Usage: "profile remove LABEL", Label: true},
	"operations.list": {Usage: "operations list", Flags: "limit cursor", ReadOnly: true}, "operations.inspect": {Usage: "operations inspect ID", ID: true, ReadOnly: true}, "operations.resume": {Usage: "operations resume ID", Flags: "passphrase-fd", ID: true},
	"state.backup": {Usage: "state backup [--to PATH]", Flags: "to"}, "state.restore": {Usage: "state restore BACKUP_PATH [--apply]", Flags: "apply", Path: true}, "state.migrate": {Usage: "state migrate [--apply]", Flags: "apply"}, "state.checkpoint": {Usage: "state checkpoint [--mode passive|full|restart|truncate]", Flags: "mode timeout"}, "state.inventory.refresh": {Usage: "state inventory refresh [--vm ID]", Flags: "vm"}, "config.explain": {Usage: "config explain [PATH]", Path: true, ReadOnly: true}, "storage.inspect": {Usage: "storage inspect", ReadOnly: true}, "version": {Usage: "version", ReadOnly: true},
}
var booleans = words("json dry-run yes capture-output apply no-start start online follow from-project refresh-tools accept-native-config no-profile remember sign-commits sign-tags")
var common = words("config project-config limactl json dry-run yes capture-output")

func words(s string) map[string]bool {
	v := map[string]bool{}
	for _, k := range strings.Fields(s) {
		v[k] = true
	}
	return v
}

type Parsed struct {
	Request app.Request
	Help    bool
	Browse  bool
}

func Parse(args []string, cwd string) (p Parsed, err error) {
	p = Parsed{Request: app.Request{Limit: 100, PassphraseFD: -1, Bind: "127.0.0.1", CheckpointMode: "passive", Timeout: "5s"}}
	r := &p.Request
	defer func() {
		if err != nil && r.Command == "" {
			r.Command = identifyCommand(args)
		}
	}()
	flags := map[string][]string{}
	positionals := []string{}
	known := words("limit cursor local bind vm profile label to project from-prototype storage-root open-action editor template presets cpus memory disk mounts-from ports-from repository git-name git-email github-user key public-key passphrase-fd kind host-key-file mode timeout")
	known["confirm-vm"] = true
	for k := range common {
		known[k] = true
	}
	for k := range booleans {
		known[k] = true
	}
	separator := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			separator = true
			if len(positionals) > 0 && positionals[0] == "exec" {
				r.GuestArgs = append([]string{}, args[i+1:]...)
			} else {
				positionals = append(positionals, args[i+1:]...)
			}
			break
		}
		if arg == "--help" || arg == "-h" {
			p.Help = true
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			return p, model.Fail("input.invalid", "Only documented long flags are accepted.")
		}
		name, value, equals := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !known[name] {
			return p, model.Fail("input.invalid", "Unknown flag --"+name+".")
		}
		if !equals {
			if booleans[name] {
				value = "true"
				if i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
					i++
					value = args[i]
				}
			} else {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return p, model.Fail("input.invalid", "--"+name+" requires a value.")
				}
				i++
				value = args[i]
			}
		}
		flags[name] = append(flags[name], value)
	}
	if len(positionals) == 0 {
		positionals = []string{"__browse"}
	}
	command := positionals[0]
	positionals = positionals[1:]
	switch command {
	case "profile", "operations", "environment", "state", "config", "storage":
		if len(positionals) == 0 {
			r.Command = command
			if p.Help {
				return p, nil
			}
			return p, model.Fail("input.invalid", "Select an action for "+command+".")
		}
		command += "." + positionals[0]
		positionals = positionals[1:]
		if command == "state.inventory" {
			if len(positionals) == 0 {
				return p, model.Fail("input.invalid", "Use state inventory refresh.")
			}
			command += "." + positionals[0]
			positionals = positionals[1:]
		}
	}
	r.Command = command
	spec, ok := Specs[command]
	if command == "__browse" {
		spec = Spec{Usage: "[PATH]", Flags: "config project-config limactl", Path: true}
		ok = true
		p.Browse = !p.Help
	}
	if !ok {
		if len(positionals) == 0 {
			path := command
			if !filepath.IsAbs(path) {
				path = filepath.Join(cwd, path)
			}
			if info, e := os.Stat(path); e == nil && info.IsDir() {
				p.Browse = true
				r.Path = path
				r.Command = "__browse"
				command = "__browse"
				spec = Spec{Usage: "[PATH]", Flags: "config project-config limactl", Path: true}
				ok = true
			}
		}
		if !ok {
			r.Command = "unknown"
			return p, model.Fail("input.invalid", "Unknown command. Run easy-lima --help.")
		}
	}
	if p.Help {
		return p, nil
	}
	allowed := words(spec.Flags)
	if words("up start restart code ssh exec forward environment.update rebind repair adopt")[command] {
		allowed["passphrase-fd"] = true
	}
	if !p.Browse {
		for k := range common {
			allowed[k] = true
		}
	}
	values := map[string]string{}
	for name, list := range flags {
		if !allowed[name] {
			return p, model.Fail("input.invalid", "--"+name+" is not accepted by "+strings.ReplaceAll(command, ".", " ")+".")
		}
		repeat := name == "repository" || name == "vm" && (command == "profile.assign" || command == "profile.unassign" || command == "profile.apply")
		if len(list) > 1 && !repeat {
			return p, model.Fail("input.invalid", "Scalar flag --"+name+" cannot be repeated.")
		}
		values[name] = list[len(list)-1]
	}
	boolean := func(name string) (bool, error) {
		if _, ok := flags[name]; !ok {
			return false, nil
		}
		v, e := strconv.ParseBool(values[name])
		if e != nil || (values[name] != "true" && values[name] != "false") {
			return false, model.Fail("input.invalid", "--"+name+" requires true or false.")
		}
		return v, nil
	}
	for _, pair := range []struct {
		name string
		dst  *bool
	}{{"json", &r.JSON}, {"dry-run", &r.DryRun}, {"yes", &r.Yes}, {"capture-output", &r.CaptureOutput}, {"apply", &r.Apply}, {"no-start", &r.NoStart}, {"start", &r.Start}, {"online", &r.Online}, {"follow", &r.Follow}, {"from-project", &r.FromProject}, {"refresh-tools", &r.RefreshTools}, {"accept-native-config", &r.AcceptNative}, {"no-profile", &r.NoProfile}} {
		v, e := boolean(pair.name)
		if e != nil {
			return p, e
		}
		*pair.dst = v
	}
	for _, pair := range []struct {
		name string
		dst  **bool
	}{{"sign-commits", &r.SignCommits}, {"sign-tags", &r.SignTags}, {"remember", &r.Remember}} {
		if _, ok := flags[pair.name]; ok {
			v, e := boolean(pair.name)
			if e != nil {
				return p, e
			}
			*pair.dst = &v
		}
	}
	for _, pair := range []struct {
		name string
		dst  *string
	}{{"config", &r.UserConfig}, {"project-config", &r.ProjectConfig}, {"limactl", &r.Limactl}, {"storage-root", &r.StorageRoot}, {"open-action", &r.OpenAction}, {"editor", &r.Editor}, {"profile", &r.Profile}, {"to", &r.To}, {"from-prototype", &r.FromPrototype}, {"project", &r.Path}, {"git-name", &r.GitName}, {"git-email", &r.GitEmail}, {"github-user", &r.GitHubUser}, {"key", &r.Key}, {"public-key", &r.PublicKey}, {"label", &r.NewLabel}, {"cursor", &r.Cursor}, {"kind", &r.RepairKind}, {"host-key-file", &r.HostKeyFile}, {"mode", &r.CheckpointMode}, {"timeout", &r.Timeout}, {"bind", &r.Bind}, {"template", &r.Settings.Environment.Template}, {"memory", &r.Settings.Resources.Memory}, {"disk", &r.Settings.Resources.Disk}, {"confirm-vm", &r.ConfirmVM}} {
		if value, ok := values[pair.name]; ok {
			if value == "" && !(pair.name == "github-user" && r.Command == "profile.edit") {
				return p, model.Fail("input.invalid", "--"+pair.name+" cannot be empty.")
			}
			*pair.dst = value
		}
	}
	_, r.GitHubUserSet = values["github-user"]
	for _, pair := range []struct {
		name string
		dst  *int
	}{{"cpus", &r.Settings.Resources.CPUs}, {"local", &r.LocalPort}, {"limit", &r.Limit}, {"passphrase-fd", &r.PassphraseFD}} {
		if value, ok := values[pair.name]; ok {
			v, e := strconv.Atoi(value)
			if e != nil {
				return p, model.Fail("input.invalid", "--"+pair.name+" requires an integer.")
			}
			*pair.dst = v
		}
	}
	if command == "forward" {
		if len(positionals) < 1 {
			return p, model.Fail("input.invalid", "forward requires a guest port.")
		}
		v, e := strconv.Atoi(positionals[len(positionals)-1])
		if e != nil {
			return p, model.Fail("input.invalid", "The guest port must be an integer.")
		}
		r.GuestPort = v
		positionals = positionals[:len(positionals)-1]
	}
	if spec.Label || spec.ID {
		if len(positionals) != 1 {
			return p, model.Fail("input.invalid", "Expected "+spec.Usage+".")
		}
		if spec.Label {
			r.Label = positionals[0]
		} else {
			r.ID = positionals[0]
			if !model.ValidID(r.ID) {
				return p, model.Fail("input.invalid", "Expected a full immutable operation ID.")
			}
		}
	} else if spec.Path {
		if len(positionals) > 1 {
			return p, model.Fail("input.invalid", "Only one project path is accepted.")
		}
		if len(positionals) == 1 {
			r.Path = positionals[0]
		}
	} else if len(positionals) != 0 {
		return p, model.Fail("input.invalid", "Unexpected positional argument.")
	}
	if ids := flags["vm"]; len(ids) > 0 {
		for _, id := range ids {
			if !model.ValidID(id) {
				return p, model.Fail("input.invalid", "--vm requires a full immutable VM ID.")
			}
		}
		if strings.HasPrefix(command, "profile.") {
			r.VMIDs = ids
		} else {
			r.VMID = ids[0]
		}
		if r.Path != "" {
			return p, model.Fail("input.invalid", "A project path and --vm are mutually exclusive.")
		}
	}
	r.Repositories = flags["repository"]
	if r.JSON && spec.Raw {
		return p, model.Fail("input.invalid", "Raw terminal commands do not accept --json.")
	}
	if r.JSON && r.Follow {
		return p, model.Fail("input.invalid", "--follow and --json are mutually exclusive.")
	}
	if r.DryRun && spec.ReadOnly {
		return p, model.Fail("input.invalid", "This command is already read-only.")
	}
	if r.CaptureOutput && (spec.ReadOnly || r.DryRun) {
		return p, model.Fail("input.invalid", "Raw capture is opt-in for an executed mutation, not diagnostics or a dry run.")
	}
	if r.Profile != "" && r.NoProfile {
		return p, model.Fail("input.invalid", "Choose --profile or --no-profile.")
	}
	if _, present := flags["passphrase-fd"]; present && r.PassphraseFD < 3 {
		return p, model.Fail("secret.input_invalid", "--passphrase-fd must be an inherited descriptor of at least 3.")
	}
	if r.Limit < 1 || r.Limit > 1000 {
		return p, model.Fail("input.invalid", "--limit must be between 1 and 1000.")
	}
	if command == "exec" {
		if !separator || len(r.GuestArgs) == 0 {
			return p, model.Fail("input.invalid", "exec requires -- followed by a guest command.")
		}
	}
	if command == "forward" && (r.LocalPort < 1 || r.LocalPort > 65535 || r.GuestPort < 1 || r.GuestPort > 65535 || (r.Bind != "127.0.0.1" && r.Bind != "::1")) {
		return p, model.Fail("input.invalid", "Forwarding requires exact ports 1–65535 and a loopback bind address.")
	}
	if _, ok := values["cpus"]; ok && r.Settings.Resources.CPUs <= 0 {
		return p, model.Fail("input.invalid", "--cpus must be positive.")
	}
	if command == "state.checkpoint" {
		valid := words("passive full restart truncate")
		duration, e := time.ParseDuration(r.Timeout)
		if !valid[r.CheckpointMode] || e != nil || duration <= 0 || duration > time.Minute {
			return p, model.Fail("input.invalid", "Invalid checkpoint mode or timeout.")
		}
	}
	if value, ok := values["presets"]; ok {
		r.Settings.Environment.Presets = []string{}
		if value != "" {
			r.Settings.Environment.Presets = strings.Split(value, ",")
		}
	}
	for _, key := range []string{"template", "presets", "cpus", "memory", "disk", "mounts-from", "ports-from"} {
		if _, ok := flags[key]; ok {
			r.CreationFlags = true
			if r.FromProject {
				return p, model.Fail("input.invalid", "--from-project cannot be combined with per-field environment overrides.")
			}
		}
	}
	for _, kind := range []string{"mounts", "ports"} {
		value, ok := values[kind+"-from"]
		if !ok {
			continue
		}
		path, e := config.ResolvePath(value, cwd)
		if e != nil {
			return p, e
		}
		if kind == "mounts" {
			f, e := config.Read(path, struct {
				SchemaVersion int            `toml:"schema_version"`
				Mounts        []config.Mount `toml:"mounts"`
			}{SchemaVersion: 1})
			if e != nil {
				return p, e
			}
			if f.Value.Mounts == nil {
				return p, model.Fail("config.invalid", "A mounts file must contain mounts, including [] to clear it.")
			}
			r.Settings.Mounts = f.Value.Mounts
			for i := range r.Settings.Mounts {
				r.Settings.Mounts[i].Source, e = config.ResolvePath(r.Settings.Mounts[i].Source, filepath.Dir(f.Path))
				if e != nil {
					return p, e
				}
			}
		} else {
			f, e := config.Read(path, struct {
				SchemaVersion int           `toml:"schema_version"`
				Ports         []config.Port `toml:"ports"`
			}{SchemaVersion: 1})
			if e != nil {
				return p, e
			}
			if f.Value.Ports == nil {
				return p, model.Fail("config.invalid", "A ports file must contain ports, including [] to clear it.")
			}
			r.Settings.Ports = f.Value.Ports
		}
	}
	if r.CreationFlags {
		if e := config.NormalizeSettings(&r.Settings, cwd, false); e != nil {
			return p, e
		}
	}
	if command == "adopt" && (r.FromPrototype == "" || r.Path == "") {
		return p, model.Fail("input.invalid", "Adopt requires --from-prototype and --project.")
	}
	if command == "state.restore" && r.Path == "" {
		return p, model.Fail("input.invalid", "Restore requires a backup path.")
	}
	if command == "rebind" && (r.VMID == "" || r.To == "") {
		return p, model.Fail("input.invalid", "Rebind requires --vm and --to.")
	}
	if command == "repair" && r.Apply && r.RepairKind == "" {
		return p, model.Fail("input.invalid", "Choose --kind before applying a repair.")
	}
	return p, nil
}

func Help(command string) string {
	if spec, ok := Specs[command]; ok {
		return fmt.Sprintf("Usage: easy-lima %s\n\nOptions: %s\nCommon: --config FILE --project-config FILE --limactl FILE --json --dry-run --yes --capture-output\n", spec.Usage, "--"+strings.Join(strings.Fields(spec.Flags), ", --"))
	}
	if words("profile operations environment state config storage")[command] {
		lines := []string{}
		for name, spec := range Specs {
			if strings.HasPrefix(name, command+".") {
				lines = append(lines, "  easy-lima "+spec.Usage)
			}
		}
		sort.Strings(lines)
		return "Usage:\n" + strings.Join(lines, "\n") + "\n\nUse an action followed by --help for its flags.\n"
	}
	return `easy-lima — Ubuntu workspaces powered by Lima

Usage: easy-lima                         Browse projects from the current folder
       easy-lima COMMAND [options]

  init          Choose the VM storage root explicitly
  up            Create or start a project workspace
  start/stop/restart/delete   Manage the same VM and disk
  code/ssh/exec  Open VS Code, a shell, or run a command
  list/inspect/doctor/logs    Inspect independent VM and identity states
  forward       Run a foreground localhost port forward
  environment update         Preview and explicitly apply environment changes
  rebind        Reattach a moved project without replacing its VM
  adopt         Transfer exclusive ownership from the supported prototype
  profile       Create, import, reuse, apply, assign, unlock, or lock identities
  operations    Inspect and resume recorded attempts
  repair        Preview a specific repair
  state         Backup, restore, checkpoint, or refresh diagnostic inventory
  config explain / storage inspect / version

Run easy-lima COMMAND --help for exact inputs. --json is noninteractive.
`
}

func identifyCommand(args []string) string {
	for i, a := range args {
		if _, ok := Specs[a]; ok {
			return a
		}
		if words("profile operations environment state config storage")[a] && i+1 < len(args) {
			name := a + "." + args[i+1]
			if name == "state.inventory" && i+2 < len(args) {
				name += "." + args[i+2]
			}
			if _, ok := Specs[name]; ok {
				return name
			}
		}
	}
	return "unknown"
}
