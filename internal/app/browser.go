package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

// BrowserDetails observes only the selected VM. It never starts, unlocks or repairs anything.
func (s Service) BrowserDetails(ctx context.Context, id string, r Request) []string {
	db, err := store.Open(ctx, s.Paths.State, false)
	if err != nil {
		return []string{"Current state: registry unavailable; use recovery inventory."}
	}
	defer db.Close()
	v, err := db.VM(ctx, id)
	if err != nil {
		return []string{"Current state: registration unavailable."}
	}
	lines := []string{}
	if in, e := decodeEnvironment(v); e == nil {
		lines = append(lines, "Environment: "+in.Settings.Environment.Template+" · "+strings.Join(in.Settings.Environment.Presets, ", "))
	}
	n, err := db.Namespace(ctx)
	if err != nil || storage.CheckRoot(n) != nil {
		return append(lines, "Current state: storage unavailable. Reconnect the selected drive.")
	}
	u, err := s.user(r)
	if err != nil {
		return append(lines, "Current state: configuration unavailable. Run doctor for details.")
	}
	a, err := s.adapter(u.Value, n, r, "browser")
	if err != nil {
		return append(lines, "Current state: Lima unavailable.")
	}
	obs, err := a.Observe(ctx, v.VM.InstanceName)
	if err != nil {
		return append(lines, "Current state: unknown. Inspect diagnostics before changing this VM.")
	}
	lines = append(lines, "Current VM state: "+strings.ToLower(obs.Status))
	if v.VM.ProfileID != nil {
		p, e := db.Profile(ctx, *v.VM.ProfileID)
		if e != nil {
			return append(lines, "Git readiness: profile unavailable.")
		}
		readiness := "not observed"
		if p.Profile.LockState == "locked" {
			readiness = "locked; use profile unlock"
		} else if c, e := agent.Connect(ctx, p.Agent.SocketPath); e == nil {
			if c.Verify(*p.Profile.Fingerprint) == nil {
				readiness = "host key available"
			}
			c.Close()
		} else {
			readiness = "agent unavailable; opening will request activation if permitted"
		}
		if v.PendingIdentityKind != nil {
			readiness += "; guest identity pending"
		} else if t, e := connection.ReadTunnel(ctx, db, id); e == nil {
			readiness += fmt.Sprintf("; tunnel %s (recorded)", t.State)
		}
		lines = append(lines, "Git readiness: "+readiness, "GitHub account: not checked here; use profile check --online.")
	}
	return lines
}
