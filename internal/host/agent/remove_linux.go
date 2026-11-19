//go:build linux

package agent

import (
	"context"
	"fmt"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"os"
	"syscall"
	"time"
)

func Remove(ctx context.Context, id string, b Binding) error {
	if b.SupervisorKind == "external" {
		return model.Fail("profile.external_managed", "An external agent cannot be removed.")
	}
	hash, err := files.Digest(b.SupervisorRef)
	if err != nil {
		return err
	}
	if hash != "" && hash != b.OwnershipEvidenceDigest {
		return model.Fail("settings.edited", "The owned agent definition changed.")
	}
	if b.SupervisorKind == "systemd_user" {
		r, e := (process.OS{}).Run(ctx, process.Request{Executable: "systemctl", Args: []string{"--user", "disable", "--now", "easy-lima-agent-" + id + ".service"}, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 4096})
		if e != nil || r.ExitCode != 0 {
			return model.Fail("operation.unknown", "The owned user agent service could not be stopped.")
		}
	} else if b.SupervisorKind == "detached" {
		body, e := files.Read(b.SupervisorRef+".process", 4096)
		if e != nil {
			return e
		}
		var pid int
		var startID string
		if _, e = fmt.Sscanf(string(body), "%d %s", &pid, &startID); e != nil {
			return e
		}
		stat, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if e == nil && processStart(stat) == startID {
			if e = verifySupervisor(ctx, id, b); e != nil {
				return e
			}
			if e = syscall.Kill(pid, syscall.SIGTERM); e != nil {
				return e
			}
			for attempts := 0; attempts < 40; attempts++ {
				stat, e = os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
				if os.IsNotExist(e) || e == nil && processStart(stat) != startID {
					break
				}
				if attempts == 39 {
					return model.Fail("operation.unknown", "The detached agent did not stop.")
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(50 * time.Millisecond):
				}
			}
		}
	} else {
		return model.Fail("ownership.conflict", "Unknown agent supervisor.")
	}
	if err = waitSocketInactive(ctx, b.SocketPath); err != nil {
		return err
	}
	if hash != "" {
		if err = os.Remove(b.SupervisorRef); err != nil {
			return err
		}
	}
	return nil
}
