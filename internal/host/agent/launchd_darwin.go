//go:build darwin

package agent

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func escaped(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func serviceName(id string) string { return "io.github.easy-lima.agent." + id }
func verifySupervisor(ctx context.Context, id string, b Binding) error {
	if b.SupervisorKind != "launchd" || b.SupervisorRef == "" {
		return model.Fail("ownership.conflict", "The agent has no matching owned supervisor.")
	}
	d, err := files.Digest(b.SupervisorRef)
	if err != nil || d != b.OwnershipEvidenceDigest {
		return model.Fail("settings.edited", "The profile's launchd definition was changed.")
	}
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "/bin/launchctl", Args: []string{"print", fmt.Sprintf("gui/%d/%s", os.Getuid(), serviceName(id))}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 64 << 10})
	if err != nil || r.ExitCode != 0 {
		return model.Fail("ownership.conflict", "The profile agent is not owned by its recorded launchd service.")
	}
	return nil
}
func plan(ctx context.Context, id string, b Binding) (Binding, []byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return b, nil, err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	path := filepath.Join(dir, serviceName(id)+".plist")
	data := []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>/usr/bin/ssh-agent</string><string>-D</string><string>-a</string><string>%s</string></array><key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>ProcessType</key><string>Background</string></dict></plist>
`, escaped(serviceName(id)), escaped(b.SocketPath)))
	b.SupervisorKind = "launchd"
	b.SupervisorRef = path
	b.OwnershipEvidenceDigest = files.Hash(data)
	return b, data, nil
}
func start(ctx context.Context, id string, b Binding) (Binding, error) {
	b, data, err := plan(ctx, id, b)
	if err != nil {
		return b, err
	}
	path := b.SupervisorRef
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return b, err
	}
	old, err := files.Digest(path)
	if err != nil {
		return b, err
	}
	if old != "" && old != files.Hash(data) {
		return b, model.Fail("settings.edited", "The profile's service definition conflicts with the expected owned file.")
	}
	if err = files.Replace(path, data, old, 0600); err != nil {
		return b, err
	}
	b.SupervisorKind = "launchd"
	b.SupervisorRef = path
	b.OwnershipEvidenceDigest = files.Hash(data)
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "/bin/launchctl", Args: []string{"bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path}, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 16 << 10})
	if err != nil {
		return b, err
	}
	if r.ExitCode != 0 {
		if err = verifySupervisor(ctx, id, b); err != nil {
			return b, model.Fail("operation.failed", "launchd refused the profile service. Its definition must be on a filesystem with enforced ownership; inspect "+path+" and launchctl diagnostics before resuming.")
		}
		r, err = (process.OS{}).Run(ctx, process.Request{Executable: "/bin/launchctl", Args: []string{"kickstart", fmt.Sprintf("gui/%d/%s", os.Getuid(), serviceName(id))}, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 16 << 10})
		if err != nil || r.ExitCode != 0 {
			return b, model.Fail("operation.failed", "The owned profile agent could not be started.")
		}
	}
	return b, Wait(ctx, b.SocketPath)
}

func supervisorInactive(ctx context.Context, id string, b Binding) error {
	if b.SupervisorKind != "launchd" {
		return model.Fail("ownership.conflict", "The agent has no matching owned supervisor.")
	}
	hash, err := files.Digest(b.SupervisorRef)
	if err != nil || hash != b.OwnershipEvidenceDigest {
		return model.Fail("settings.edited", "The agent definition changed.")
	}
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "/bin/launchctl", Args: []string{"print", fmt.Sprintf("gui/%d/%s", os.Getuid(), serviceName(id))}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 64 << 10})
	if err != nil || r.Truncated {
		return model.Fail("operation.unknown", "The agent supervisor cannot be inspected.")
	}
	if r.ExitCode == 113 {
		return nil
	}
	if r.ExitCode != 0 {
		return model.Fail("operation.unknown", "The agent supervisor state is unknown.")
	}
	for _, line := range strings.Split(string(r.Stdout), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "pid = ") {
			return model.Fail("operation.unknown", "The owned agent is still starting; retry after it settles.")
		}
	}
	return nil
}
