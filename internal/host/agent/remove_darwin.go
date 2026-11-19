//go:build darwin

package agent

import (
	"context"
	"fmt"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"os"
	"time"
)

func Remove(ctx context.Context, id string, b Binding) error {
	if b.SupervisorKind != "launchd" {
		return model.Fail("profile.external_managed", "Only an owned launchd agent can be removed.")
	}
	hash, err := files.Digest(b.SupervisorRef)
	if err != nil {
		return err
	}
	if hash != "" && hash != b.OwnershipEvidenceDigest {
		return model.Fail("settings.edited", "The owned agent definition changed.")
	}
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), serviceName(id))
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "/bin/launchctl", Args: []string{"print", service}, Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 64 << 10})
	if err != nil {
		return err
	}
	if r.Truncated || r.ExitCode != 0 && r.ExitCode != 113 {
		return model.Fail("operation.unknown", "The owned agent supervisor could not be inspected.")
	}
	if r.ExitCode == 0 {
		if hash == "" {
			return model.Fail("ownership.conflict", "The live service has no retained owned definition.")
		}
		r, err = (process.OS{}).Run(ctx, process.Request{Executable: "/bin/launchctl", Args: []string{"bootout", service}, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 4096})
		if err != nil || r.ExitCode != 0 {
			return model.Fail("operation.unknown", "The owned agent service could not be conclusively stopped.")
		}
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
