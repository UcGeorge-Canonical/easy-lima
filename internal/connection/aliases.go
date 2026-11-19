package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func sshQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(s) + `"`
}
func SSHArgs(state string, e Endpoint) []string {
	return []string{"-F", "/dev/null", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "NoHostAuthenticationForLocalhost=no", "-o", "GlobalKnownHostsFile=/dev/null", "-o", "UserKnownHostsFile=" + sshQuote(TrustPath(state, e.VMID)), "-o", "HostKeyAlias=" + TrustToken(e.VMID), "-o", "VerifyHostKeyDNS=no", "-o", "UpdateHostKeys=no", "-o", "IdentityAgent=none", "-o", "IdentitiesOnly=yes", "-o", "AddKeysToAgent=no", "-o", "ForwardAgent=no", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no", "-o", "GSSAPIAuthentication=no", "-o", "HostbasedAuthentication=no", "-o", "ConnectTimeout=10", "-o", "ControlMaster=no", "-o", "ControlPath=none", "-i", e.IdentityFile, "-p", fmt.Sprint(e.Port), "-l", e.User}
}
func AliasText(binary, state, alias string, e Endpoint) []byte {
	proxy := strings.ReplaceAll(process.Quote(binary), "%", "%%") + " __ssh-proxy " + e.VMID
	return []byte(fmt.Sprintf("# Managed by easy-lima; use explicit alias repair after moving the binary.\nHost %s\n  HostName %s\n  User %s\n  Port %d\n  IdentityFile %s\n  IdentityAgent none\n  IdentitiesOnly yes\n  AddKeysToAgent no\n  ForwardAgent no\n  PasswordAuthentication no\n  KbdInteractiveAuthentication no\n  GSSAPIAuthentication no\n  HostbasedAuthentication no\n  StrictHostKeyChecking yes\n  NoHostAuthenticationForLocalhost no\n  VerifyHostKeyDNS no\n  UpdateHostKeys no\n  GlobalKnownHostsFile /dev/null\n  UserKnownHostsFile %s\n  HostKeyAlias %s\n  ControlMaster no\n  ControlPath none\n  ProxyCommand %s\n", alias, e.Address, e.User, e.Port, sshQuote(e.IdentityFile), sshQuote(TrustPath(state, e.VMID)), TrustToken(e.VMID), proxy))
}

const includeBegin = "# BEGIN easy-lima managed include\n"
const includeEnd = "# END easy-lima managed include\n"

func configTarget(home string) (string, error) {
	p := filepath.Join(home, ".ssh", "config")
	if i, err := os.Lstat(p); err == nil && i.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", model.Fail("settings.edited", "The user's SSH config symlink cannot be resolved.")
		}
		return target, nil
	}
	return p, nil
}
func checkStaticConfig(b []byte, alias string) error {
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if strings.EqualFold(fields[0], "Match") {
			return model.Fail("settings.edited", "Dynamic SSH Match rules require review before a managed alias can be used.")
		}
		if strings.EqualFold(fields[0], "Host") {
			for _, h := range fields[1:] {
				if h == alias {
					return model.Fail("settings.edited", "The user's SSH configuration already defines this alias.")
				}
			}
		}
	}
	return nil
}
func PublishAlias(ctx context.Context, s *store.Store, opID, home, binary string, v store.VMRecord, e Endpoint) error {
	if !files.PathOK(binary) {
		return model.Invalid("binary path", "the installed executable path is not representable")
	}
	dir := filepath.Join(s.Dir, "ssh", "hosts")
	if err := files.PrivateDir(dir); err != nil {
		return err
	}
	p := filepath.Join(dir, v.VM.ID+".conf")
	expected, err := s.Setting(ctx, v.VM.ID, p, "alias")
	if err != nil {
		return err
	}
	b := AliasText(binary, s.Dir, v.VM.SSHAlias, e)
	current, err := files.Digest(p)
	if err != nil {
		return err
	}
	if current != expected && current != files.Hash(b) {
		return model.Fail("settings.edited", "The managed SSH alias was edited.")
	}
	if current != files.Hash(b) {
		if err = files.Replace(p, b, expected, 0600); err != nil {
			return err
		}
	}
	if err = s.OwnSetting(ctx, opID, v.VM.ID, "vm", p, "alias", files.Hash(b)); err != nil {
		return err
	}
	sshdir := filepath.Join(home, ".ssh")
	if err = files.PrivateDir(sshdir); err != nil {
		return err
	}
	target, err := configTarget(home)
	if err != nil {
		return err
	}
	raw, err := files.Read(target, 1<<20)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	old := ""
	if err == nil {
		old = files.Hash(raw)
	}
	if err = checkStaticConfig(raw, v.VM.SSHAlias); err != nil {
		return err
	}
	include := includeBegin + "Include " + sshQuote(filepath.Join(dir, "*.conf")) + "\nHost *\n" + includeEnd
	text := string(raw)
	if strings.Contains(text, includeBegin) {
		start := strings.Index(text, includeBegin)
		end := strings.Index(text, includeEnd)
		if start != 0 || end < start {
			return model.Fail("settings.edited", "The managed SSH include was moved or edited.")
		}
		block := text[start : end+len(includeEnd)]
		if block != include {
			return model.Fail("settings.edited", "The managed SSH include differs from its recorded destination.")
		}
		return nil
	}
	if strings.Contains(text, includeEnd) {
		return model.Fail("settings.edited", "The managed SSH include is incomplete.")
	}
	previous, err := s.Setting(ctx, "global", target, "include")
	if err != nil {
		return err
	}
	if previous != "" {
		op, e := s.Operation(ctx, opID)
		if e != nil {
			return e
		}
		var repair struct {
			Kind string `json:"kind"`
		}
		if op.Operation.Kind != "repair" || json.Unmarshal(op.Inputs, &repair) != nil || repair.Kind != "alias" {
			return model.Fail("settings.edited", "The managed SSH include was removed. Use explicit alias repair to restore it.")
		}
	}
	backup := filepath.Join(s.Dir, "backups", "ssh-config-"+opID)
	if old != "" {
		if d, _ := files.Digest(backup); d == "" {
			if err = files.WriteNew(backup, raw); err != nil {
				return err
			}
		}
	}
	if err = files.Replace(target, []byte(include+text), old, 0600); err != nil {
		return err
	}
	return s.OwnSetting(ctx, opID, "global", "global", target, "include", files.Hash([]byte(include)))
}
func VerifyAlias(ctx context.Context, s *store.Store, home string, v store.VMRecord) error {
	p := filepath.Join(s.Dir, "ssh", "hosts", v.VM.ID+".conf")
	expected, err := s.Setting(ctx, v.VM.ID, p, "alias")
	if err != nil {
		return err
	}
	got, err := files.Digest(p)
	if err != nil || got == "" || got != expected {
		return model.Fail("settings.edited", "The SSH alias is missing or edited. Use explicit alias repair.")
	}
	target, err := configTarget(home)
	if err != nil {
		return err
	}
	b, err := files.Read(target, 1<<20)
	if err != nil {
		return model.Fail("settings.edited", "The user's SSH configuration is unavailable.")
	}
	include := includeBegin + "Include " + sshQuote(filepath.Join(s.Dir, "ssh", "hosts", "*.conf")) + "\nHost *\n" + includeEnd
	if !strings.HasPrefix(string(b), include) {
		return model.Fail("settings.edited", "The managed SSH include is missing or not first.")
	}
	return checkStaticConfig(b, v.VM.SSHAlias)
}

// RemoveAlias removes only the unchanged file owned for this VM. The shared include remains.
func RemoveAlias(ctx context.Context, s *store.Store, id string) error {
	path := filepath.Join(s.Dir, "ssh", "hosts", id+".conf")
	expected, err := s.Setting(ctx, id, path, "alias")
	if err != nil {
		return err
	}
	return files.RemoveExpected(path, expected)
}
