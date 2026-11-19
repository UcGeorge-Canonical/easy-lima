//go:build linux

package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func verifySupervisor(ctx context.Context, id string, b Binding) error {
	if b.SupervisorKind == "detached" {
		p, err := files.Read(b.SupervisorRef, 4096)
		if err != nil || files.Hash(p) != b.OwnershipEvidenceDigest {
			return model.Fail("ownership.conflict", "Detached agent ownership evidence is unavailable.")
		}
		p, err = files.Read(b.SupervisorRef+".process", 4096)
		if err != nil {
			return model.Fail("ownership.conflict", "Detached agent process evidence is unavailable.")
		}
		var pid int
		var startID string
		if _, err = fmt.Sscanf(string(p), "%d %s", &pid, &startID); err != nil {
			return err
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil || processStart(stat) != startID {
			return model.Fail("ownership.conflict", "Detached agent process identity changed.")
		}
		info, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
		if err != nil {
			return model.Fail("ownership.conflict", "Detached agent process is unavailable.")
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || owner.Uid != uint32(os.Getuid()) {
			return model.Fail("ownership.conflict", "Detached agent belongs to another user.")
		}
		definition, err := files.Read(b.SupervisorRef, 4096)
		if err != nil {
			return err
		}
		fields := strings.Split(strings.TrimSuffix(string(definition), "\n"), "\n")
		command, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil || len(fields) != 4 || string(command) != fields[2]+"\x00-D\x00-a\x00"+b.SocketPath+"\x00" {
			return model.Fail("ownership.conflict", "Detached agent command differs from its owned definition.")
		}
		return nil
	}
	if b.SupervisorKind != "systemd_user" {
		return model.Fail("ownership.conflict", "The profile agent has no owned supervisor.")
	}
	d, err := files.Digest(b.SupervisorRef)
	if err != nil || d != b.OwnershipEvidenceDigest {
		return model.Fail("settings.edited", "The profile agent unit was changed.")
	}
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "systemctl", Args: []string{"--user", "is-active", "easy-lima-agent-" + id + ".service"}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 4096})
	if err != nil || r.ExitCode != 0 {
		return model.Fail("ownership.conflict", "The owned agent unit is not active.")
	}
	return nil
}
func processStart(b []byte) string {
	i := strings.LastIndex(string(b), ")")
	if i < 0 {
		return ""
	}
	fields := strings.Fields(string(b[i+1:]))
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}
func plan(ctx context.Context, id string, b Binding) (Binding, []byte, error) {
	executable, err := exec.LookPath("ssh-agent")
	if err != nil {
		return b, nil, model.Fail("dependency.missing", "OpenSSH ssh-agent is required.")
	}
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "systemctl", Args: []string{"--user", "show-environment"}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 16 << 10})
	if (err == nil && r.ExitCode == 0 && b.SupervisorKind != "detached") || b.SupervisorKind == "systemd_user" {
		home, e := os.UserHomeDir()
		if e != nil {
			return b, nil, e
		}
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		quote := func(v string) string {
			return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(v) + `"`
		}
		data := []byte("[Unit]\nDescription=easy-lima profile SSH agent\n[Service]\nType=simple\nExecStart=" + quote(executable) + " -D -a " + quote(b.SocketPath) + "\nRestart=on-failure\nUMask=0077\n[Install]\nWantedBy=default.target\n")
		b.SupervisorKind = "systemd_user"
		b.SupervisorRef = filepath.Join(cfg, "systemd", "user", "easy-lima-agent-"+id+".service")
		b.OwnershipEvidenceDigest = files.Hash(data)
		return b, data, nil
	}
	data := []byte("easy-lima-agent-v1\n" + id + "\n" + executable + "\n" + b.SocketPath + "\n")
	b.SupervisorKind = "detached"
	b.SupervisorRef = filepath.Join(filepath.Dir(b.SocketPath), id+".owner")
	b.OwnershipEvidenceDigest = files.Hash(data)
	return b, data, nil
}
func start(ctx context.Context, id string, b Binding) (Binding, error) {
	planned, ownerData, planErr := plan(ctx, id, b)
	if planErr != nil {
		return b, planErr
	}
	b = planned
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "systemctl", Args: []string{"--user", "show-environment"}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 16 << 10})
	if err == nil && r.ExitCode == 0 && b.SupervisorKind == "systemd_user" {
		home, err := os.UserHomeDir()
		if err != nil {
			return b, err
		}
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		dir := filepath.Join(cfg, "systemd", "user")
		if err = os.MkdirAll(dir, 0700); err != nil {
			return b, err
		}
		name := "easy-lima-agent-" + id + ".service"
		path := filepath.Join(dir, name)
		agent, err := exec.LookPath("ssh-agent")
		if err != nil {
			return b, model.Fail("dependency.missing", "OpenSSH ssh-agent is required.")
		}
		quote := func(v string) string {
			return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(v) + `"`
		}
		data := []byte("[Unit]\nDescription=easy-lima profile SSH agent\n[Service]\nType=simple\nExecStart=" + quote(agent) + " -D -a " + quote(b.SocketPath) + "\nRestart=on-failure\nUMask=0077\n[Install]\nWantedBy=default.target\n")
		old, err := files.Digest(path)
		if err != nil {
			return b, err
		}
		if old != "" && old != files.Hash(data) {
			return b, model.Fail("settings.edited", "The profile agent unit differs from its owned definition.")
		}
		if err = files.Replace(path, data, old, 0600); err != nil {
			return b, err
		}
		for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", "--now", name}} {
			r, err = (process.OS{}).Run(ctx, process.Request{Executable: "systemctl", Args: args, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 4096})
			if err != nil || r.ExitCode != 0 {
				return b, model.Fail("operation.failed", "The owned user agent unit could not be started.")
			}
		}
		b.SupervisorKind = "systemd_user"
		b.SupervisorRef = path
		b.OwnershipEvidenceDigest = files.Hash(data)
		return b, Wait(ctx, b.SocketPath)
	}
	// Headless sessions retain an independently sessioned agent with exact /proc
	// process-start evidence. It is never killed on foreground command exit.
	p, err := exec.LookPath("ssh-agent")
	if err != nil {
		return b, model.Fail("dependency.missing", "OpenSSH ssh-agent is required.")
	}
	cmd := exec.Command(p, "-D", "-a", b.SocketPath)
	cmd.Env = process.Environment(nil)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		return b, err
	}
	// Reap a stopped agent while this foreground process still exists. The
	// separate session continues independently when the CLI exits first.
	go func() { _ = cmd.Wait() }()
	pid := cmd.Process.Pid
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return b, err
	}
	data := []byte(fmt.Sprintf("%d %s\n", pid, processStart(stat)))
	path := b.SupervisorRef + ".process"
	old, err := files.Digest(path)
	if err != nil {
		return b, err
	}
	if err = files.Replace(path, data, old, 0600); err != nil {
		return b, err
	}
	ownerOld, e := files.Digest(b.SupervisorRef)
	if e != nil {
		return b, e
	}
	if ownerOld != "" && ownerOld != b.OwnershipEvidenceDigest {
		return b, model.Fail("ownership.conflict", "Detached agent owner record changed.")
	}
	if e = files.Replace(b.SupervisorRef, ownerData, ownerOld, 0600); e != nil {
		return b, e
	}
	return b, Wait(ctx, b.SocketPath)
}

func supervisorInactive(ctx context.Context, id string, b Binding) error {
	hash, err := files.Digest(b.SupervisorRef)
	if err != nil || hash != b.OwnershipEvidenceDigest {
		return model.Fail("settings.edited", "The agent definition changed.")
	}
	if b.SupervisorKind == "detached" {
		raw, e := files.Read(b.SupervisorRef+".process", 4096)
		if e != nil {
			return model.Fail("operation.unknown", "Detached agent process evidence is unavailable.")
		}
		var pid int
		var start string
		if _, e = fmt.Sscanf(string(raw), "%d %s", &pid, &start); e != nil {
			return e
		}
		stat, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if os.IsNotExist(e) || e == nil && processStart(stat) != start {
			return nil
		}
		return model.Fail("operation.unknown", "The detached agent may still be active.")
	}
	if b.SupervisorKind != "systemd_user" {
		return model.Fail("ownership.conflict", "The agent has no matching owned supervisor.")
	}
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "systemctl", Args: []string{"--user", "show", "easy-lima-agent-" + id + ".service", "--property=ActiveState", "--property=MainPID"}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 4096})
	if err != nil || r.ExitCode != 0 || r.Truncated {
		return model.Fail("operation.unknown", "The agent supervisor cannot be inspected.")
	}
	state := string(r.Stdout)
	if strings.Contains(state, "MainPID=0\n") && (strings.Contains(state, "ActiveState=inactive\n") || strings.Contains(state, "ActiveState=failed\n")) {
		return nil
	}
	return model.Fail("operation.unknown", "The owned agent is still active or starting.")
}
