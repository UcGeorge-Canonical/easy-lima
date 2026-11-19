package lima

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Quiescence struct {
	Instance          string `json:"instance"`
	ObservedAt        string `json:"observed_at"`
	ConfigDigest      string `json:"config_digest"`
	ProcessVisibility string `json:"process_visibility"`
	ControlEndpoint   string `json:"control_endpoint"`
	Complete          bool   `json:"complete"`
}

func (a Adapter) ObserveStopped(ctx context.Context, name, prototypeRoot string) (Quiescence, error) {
	proof := Quiescence{Instance: name, ObservedAt: model.Now()}
	obs, err := a.Observe(ctx, name)
	if err != nil {
		return proof, err
	}
	if obs.Status != "Stopped" {
		return proof, model.Fail("operation.unknown", "The existing VM is not observably stopped.")
	}
	if err = a.Existing(name); err != nil {
		return proof, err
	}
	dir := filepath.Join(a.Root, name)
	proof.ConfigDigest, err = files.Digest(filepath.Join(dir, "lima.yaml"))
	if err != nil {
		return proof, err
	}
	// Complete bounded process visibility is required. Raw command lines are
	// inspected in memory only and never retained in operation evidence or logs.
	r, err := a.runner().Run(ctx, process.Request{Executable: "ps", Args: []string{"-axww", "-o", "pid=,comm=,args="}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 8 << 20})
	if err != nil || r.ExitCode != 0 || r.Truncated {
		return proof, model.Fail("operation.unknown", "Host process visibility is incomplete; ownership handoff is blocked.")
	}
	for _, line := range strings.Split(string(r.Stdout), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil {
			return proof, model.Fail("operation.unknown", "Host process evidence could not be parsed.")
		}
		if pid == os.Getpid() {
			continue
		}
		command := filepath.Base(f[1])
		args := strings.Join(f[2:], " ")
		if strings.Contains(command, "limactl") || strings.HasPrefix(command, "qemu-system") || strings.Contains(command, "lima-hostagent") {
			if strings.Contains(args, name) || strings.Contains(args, a.Root) || command == "limactl" {
				return proof, model.Fail("operation.unknown", "A native VM controller may still affect this namespace; wait for it to finish before handoff.")
			}
		}
		if prototypeRoot != "" && strings.Contains(args, filepath.Join(prototypeRoot, ".local-launcher")) {
			return proof, model.Fail("operation.unknown", "A prototype controller may still be active; ownership was not transferred.")
		}
	}
	for _, file := range []string{"ha.pid", "qemu.pid", "vz.pid"} {
		b, e := files.Read(filepath.Join(dir, file), 64)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return proof, e
		}
		pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
		if e != nil || pid <= 0 {
			return proof, model.Fail("operation.unknown", "A native PID marker is ambiguous.")
		}
		if e = syscall.Kill(pid, 0); e == nil || e == syscall.EPERM {
			return proof, model.Fail("operation.unknown", "A process named by native VM evidence remains alive; its identity cannot be discarded.")
		}
	}
	endpoint := filepath.Join(dir, "ha.sock")
	d := net.Dialer{Timeout: time.Second}
	conn, e := d.DialContext(ctx, "unix", endpoint)
	if e == nil {
		conn.Close()
		return proof, model.Fail("operation.unknown", "The VM host control endpoint remains active.")
	}
	if ne, ok := e.(net.Error); ok && ne.Timeout() {
		return proof, model.Fail("operation.unknown", "The VM host control endpoint did not yield conclusive evidence.")
	}
	if !errors.Is(e, syscall.ENOENT) && !errors.Is(e, syscall.ECONNREFUSED) {
		return proof, model.Fail("operation.unknown", "The VM control endpoint could not be conclusively inspected.")
	}
	proof.ControlEndpoint = "inactive"
	proof.ProcessVisibility = "complete"
	final, e := a.Observe(ctx, name)
	if e != nil || final.Status != "Stopped" {
		return proof, model.Fail("operation.unknown", "VM state changed during quiescence observation.")
	}
	hash, e := files.Digest(filepath.Join(dir, "lima.yaml"))
	if e != nil || hash != proof.ConfigDigest {
		return proof, model.Fail("config.drift", "Native configuration changed during quiescence observation.")
	}
	proof.Complete = true
	return proof, nil
}

// NoController rejects a concurrent lifecycle CLI while allowing the existing
// VM host agent to keep running during a graceful stop retry.
func (a Adapter) NoController(ctx context.Context, name string) error {
	r, err := a.runner().Run(ctx, process.Request{Executable: "ps", Args: []string{"-axww", "-o", "pid=,comm=,args="}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 8 << 20})
	if err != nil || r.ExitCode != 0 || r.Truncated {
		return model.Fail("operation.unknown", "Native controller visibility is incomplete.")
	}
	for _, line := range strings.Split(string(r.Stdout), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil {
			return model.Fail("operation.unknown", "Invalid process evidence.")
		}
		if pid == os.Getpid() {
			continue
		}
		if filepath.Base(f[1]) != "limactl" {
			continue
		}
		args := f[2:]
		hasName, mutating := false, false
		for _, arg := range args {
			if arg == name {
				hasName = true
			}
			switch arg {
			case "start", "stop", "delete", "create", "edit", "factory-reset":
				mutating = true
			}
		}
		if hasName && mutating {
			return model.Fail("operation.unknown", "A native lifecycle command still targets this VM. Wait for it before resuming.")
		}
	}
	return nil
}
