package connection

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func GuestSocket(id string) string { return "/run/easy-lima/" + id + "/agent.sock" }

type Tunnel struct {
	ID, Control, Guest, Agent, State                      string
	ProfileID, CredentialID, IdentityID                   string
	BindingGeneration, RuntimeGeneration, TrustGeneration int64
	BootID                                                string
}

func ReadTunnel(ctx context.Context, s *store.Store, vmID string) (Tunnel, error) {
	var t Tunnel
	err := s.DB.QueryRowContext(ctx, "SELECT id,host_control_socket,guest_socket,host_agent_socket,state,profile_id,credential_revision_id,identity_revision_id,profile_binding_generation,profile_runtime_generation,trust_generation,COALESCE(vm_boot_id,'') FROM connection_bindings WHERE registration_id=? AND state<>'retired'", vmID).Scan(&t.ID, &t.Control, &t.Guest, &t.Agent, &t.State, &t.ProfileID, &t.CredentialID, &t.IdentityID, &t.BindingGeneration, &t.RuntimeGeneration, &t.TrustGeneration, &t.BootID)
	return t, err
}
func tunnelControl(ctx context.Context, s *store.Store, e Endpoint, control, action string) (process.Result, error) {
	args := tunnelArgs(s.Dir, e)
	args = append(args, "-S", control, "-O", action, e.Address)
	return (process.OS{}).Run(ctx, process.Request{Executable: "ssh", Args: args, Env: process.Environment(nil), Timeout: 10 * time.Second, Limit: 4096})
}
func EnsureTunnel(ctx context.Context, s *store.Store, opID, cache string, v store.VMRecord, p store.ProfileRecord, e Endpoint, c *Client) error {
	if err := VerifyTrustExport(ctx, s, v.VM.ID); err != nil {
		return err
	}
	if v.PendingIdentityKind != nil {
		return model.Fail("profile.binding_required", "Pending guest identity must complete before forwarding.")
	}
	if v.AppliedProfileBindingGeneration == nil || *v.AppliedProfileBindingGeneration != v.ProfileBindingGeneration || v.AppliedProfileID == nil || *v.AppliedProfileID != p.Profile.ID || v.VM.AppliedIdentityRevisionID == nil {
		return model.Fail("profile.binding_required", "The VM's applied identity does not match its selected profile.")
	}
	if p.Profile.LockState == "locked" || p.TransitionOperationID != nil {
		return model.Fail("profile.explicitly_locked", "The selected profile is locked or changing.")
	}
	if v.ActiveTrustGeneration == nil {
		return model.Fail("trust.mismatch", "Pinned VM trust is required before forwarding.")
	}
	if p.Profile.Fingerprint == nil {
		return model.Fail("record.invalid", "Profile fingerprint is missing.")
	}
	if p.Profile.AgentManagement == "owned" {
		if err := agent.VerifyOwnership(ctx, p.Profile.ID, p.Agent); err != nil {
			return err
		}
	}
	hostAgent, err := agent.Connect(ctx, p.Agent.SocketPath)
	if err != nil {
		return err
	}
	err = hostAgent.SignProbe(*p.Profile.Fingerprint)
	hostAgent.Close()
	if err != nil {
		return err
	}
	boot, err := c.Output(ctx, "cat", "/proc/sys/kernel/random/boot_id")
	if err != nil {
		return err
	}
	bootID := strings.TrimSpace(string(boot))
	if len(bootID) != 36 {
		return model.Fail("check.incomplete", "The guest boot identity is invalid.")
	}
	rawID, eID := hex.DecodeString(v.VM.ID)
	if eID != nil {
		return model.Invalid("vm id", "invalid immutable ID")
	}
	runtimeDir, err := files.RuntimeDir(cache)
	if err != nil {
		return err
	}
	control := filepath.Join(runtimeDir, "t-"+base64.RawURLEncoding.EncodeToString(rawID))
	// OpenSSH appends a dot and 16 random characters before renaming its
	// control socket. Reserve that suffix within Darwin's 104-byte sun_path.
	if len(control) >= 86 {
		return model.Invalid("control socket", "path exceeds platform socket limit")
	}
	if err = files.PrivateDir(filepath.Dir(control)); err != nil {
		return err
	}
	guard, err := files.Lock(ctx, control+".lock", true, true)
	if err != nil {
		return err
	}
	defer guard.Close()
	old, err := ReadTunnel(ctx, s, v.VM.ID)
	if err == nil {
		r, er := tunnelControl(ctx, s, e, old.Control, "check")
		if er == nil && r.ExitCode == 0 {
			if old.State != "ready" || old.ProfileID != p.Profile.ID || old.CredentialID != p.CredentialID || old.IdentityID != *v.VM.AppliedIdentityRevisionID || old.BindingGeneration != v.ProfileBindingGeneration || old.RuntimeGeneration != p.RuntimeGeneration || old.TrustGeneration != *v.ActiveTrustGeneration || old.BootID != bootID {
				if old.ProfileID != p.Profile.ID || old.CredentialID != p.CredentialID || old.BindingGeneration != v.ProfileBindingGeneration || old.TrustGeneration != *v.ActiveTrustGeneration || old.BootID != bootID {
					return model.Fail("binding.conflict", "A live identity tunnel has a different binding; reconcile its transition before replacement.")
				}
				if err = RevokeTunnel(ctx, s, opID, v, e); err != nil {
					return err
				}
			} else {
				return ProbeForwarding(ctx, c, v.VM.ID, *p.Profile.Fingerprint)
			}
		} else if old.BootID == bootID {
			return model.Fail("connection.repair_required", "The prior tunnel controller is unavailable in this guest boot. Stop the VM explicitly, then open it again; no replacement tunnel was started.")
		}
		if err = retireTunnel(ctx, s, opID, old.ID); err != nil {
			return err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	uidText, err := c.Output(ctx, "id", "-u")
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(strings.TrimSpace(string(uidText)))
	if err != nil || uid < 1 {
		return model.Fail("check.incomplete", "The guest user ID is invalid.")
	}
	id := model.NewID()
	guest := GuestSocket(v.VM.ID)
	if err = s.Change(ctx, store.Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		_, er := tx.ExecContext(ctx, `INSERT INTO connection_bindings(id,registration_id,profile_id,credential_revision_id,identity_revision_id,generation,profile_binding_generation,profile_runtime_generation,trust_generation,vm_boot_id,guest_uid,host_agent_socket,host_control_socket,guest_socket,endpoint_address,endpoint_port,state,operation_id,created_at) VALUES(?,?,?,?,?,1,?,?,?,?,?,?,?,?,?,?,'establishing',?,?)`, id, v.VM.ID, p.Profile.ID, p.CredentialID, *v.VM.AppliedIdentityRevisionID, v.ProfileBindingGeneration, p.RuntimeGeneration, *v.ActiveTrustGeneration, bootID, uid, p.Agent.SocketPath, control, guest, e.Address, e.Port, opID, model.Now())
		return er
	}); err != nil {
		return err
	}
	args := tunnelArgs(s.Dir, e)
	args = append(args, "-M", "-S", control, "-o", "ControlMaster=yes", "-o", "ControlPersist=yes", "-o", "ExitOnForwardFailure=yes", "-o", "StreamLocalBindUnlink=no", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3", "-fNT", "-R", guest+":"+p.Agent.SocketPath, e.Address)
	r, err := (process.OS{}).Run(ctx, process.Request{Executable: "ssh", Args: args, Env: process.Environment(nil), Timeout: 20 * time.Second, Limit: 4096})
	if err != nil || r.ExitCode != 0 {
		return model.Fail("operation.partial", "The VM is running, but its profile tunnel could not be established.")
	}
	if err = ProbeForwarding(ctx, c, v.VM.ID, *p.Profile.Fingerprint); err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		if revokeErr := RevokeTunnel(cleanup, s, opID, v, e); revokeErr != nil {
			return revokeErr
		}
		return err
	}
	return s.Change(ctx, store.Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		_, er := tx.ExecContext(ctx, "UPDATE connection_bindings SET state='ready',last_probe=?,row_revision=row_revision+1 WHERE id=?", store.JSON(map[string]string{"fingerprint": *p.Profile.Fingerprint, "checked_at": model.Now()}), id)
		return er
	})
}
func tunnelArgs(state string, e Endpoint) []string {
	original := SSHArgs(state, e)
	out := []string{}
	for i := 0; i < len(original); i++ {
		if original[i] == "-o" && i+1 < len(original) && (original[i+1] == "ControlMaster=no" || original[i+1] == "ControlPath=none") {
			i++
			continue
		}
		out = append(out, original[i])
	}
	return out
}

// A verified stopped VM cannot retain a guest forwarding channel. This proof
// is only accepted from the coordinator's complete native state observation.
func RetireStoppedTunnel(ctx context.Context, s *store.Store, opID, vmID string) error {
	t, err := ReadTunnel(ctx, s, vmID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return retireTunnel(ctx, s, opID, t.ID)
}
func ProbeForwarding(ctx context.Context, c *Client, id, fingerprint string) error {
	b, err := c.Output(ctx, "env", "SSH_AUTH_SOCK="+GuestSocket(id), "ssh-add", "-l", "-E", "sha256")
	if err != nil {
		return model.Fail("check.incomplete", "The stable guest profile socket is unavailable.")
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], " "+fingerprint+" ") {
		return model.Fail("identity.key_mismatch", "The guest socket does not expose exactly the selected profile key.")
	}
	if _, err = c.Output(ctx, "sh", "-c", `SSH_AUTH_SOCK="$1" ssh-add -T "$HOME/.config/easy-lima/public_key"`, "easy-lima-sign-probe", GuestSocket(id)); err != nil {
		return model.Fail("check.failed", "The forwarded profile key could not complete an SSH signing challenge.")
	}
	return nil
}
func retireTunnel(ctx context.Context, s *store.Store, opID, id string) error {
	return s.Change(ctx, store.Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE connection_bindings SET state='retired',retired_at=?,row_revision=row_revision+1 WHERE id=?", model.Now(), id)
		return err
	})
}

func RepairTunnel(ctx context.Context, s *store.Store, opID string, v store.VMRecord, e Endpoint, c *Client) error {
	t, err := ReadTunnel(ctx, s, v.VM.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	r, err := tunnelControl(ctx, s, e, t.Control, "check")
	if err == nil && r.ExitCode == 0 {
		return RevokeTunnel(ctx, s, opID, v, e)
	}
	boot, err := c.Output(ctx, "cat", "/proc/sys/kernel/random/boot_id")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(boot)) == t.BootID {
		return model.Fail("connection.repair_required", "The old tunnel cannot be proven revoked in this guest boot. Explicitly stop the VM, then retry connection repair.")
	}
	return retireTunnel(ctx, s, opID, t.ID)
}
func RevokeTunnel(ctx context.Context, s *store.Store, opID string, v store.VMRecord, e Endpoint) error {
	t, err := ReadTunnel(ctx, s, v.VM.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if t.State != "revoking" {
		if err = s.Change(ctx, store.Change{Kind: "profile", OperationID: opID}, func(tx *sql.Tx) error {
			_, e := tx.ExecContext(ctx, "UPDATE connection_bindings SET state='revoking',row_revision=row_revision+1 WHERE id=?", t.ID)
			return e
		}); err != nil {
			return err
		}
	}
	r, err := tunnelControl(ctx, s, e, t.Control, "check")
	if err == nil && r.ExitCode == 0 {
		r, err = tunnelControl(ctx, s, e, t.Control, "exit")
		if err != nil || r.ExitCode != 0 {
			return model.Fail("operation.unknown", "Profile forwarding could not be conclusively revoked.")
		}
	} else if t.State != "revoking" {
		return model.Fail("operation.unknown", fmt.Sprintf("Tunnel %s needs explicit reconciliation before identity changes.", t.ID))
	}
	trust, err := ReadTrust(ctx, s, v.VM.ID)
	if err != nil {
		return err
	}
	key, err := trust.Key()
	if err != nil {
		return err
	}
	c, err := Dial(ctx, e, key, false)
	if err != nil {
		return err
	}
	defer c.Close()
	// The persisted binding owns this exact guest socket. Only remove it after
	// a probe proves that no listener remains; never unlink an active socket.
	for attempt := 0; attempt < 20; attempt++ {
		_, err = c.Output(ctx, "python3", "-c", removeInactiveSocket, GuestSocket(v.VM.ID))
		if err == nil {
			return retireTunnel(ctx, s, opID, t.ID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return model.Fail("operation.unknown", "The revoked tunnel's guest socket could not be proven inactive. Stop the VM before recovery.")
}

const removeInactiveSocket = `import os,sys,socket,stat,errno
p=sys.argv[1]
try: st=os.lstat(p)
except FileNotFoundError: sys.exit(0)
if not stat.S_ISSOCK(st.st_mode) or st.st_uid!=os.getuid(): sys.exit(75)
s=socket.socket(socket.AF_UNIX);s.settimeout(1)
try:
 s.connect(p)
 sys.exit(76)
except OSError as e:
 if e.errno not in (errno.ECONNREFUSED,errno.ENOENT): sys.exit(77)
finally: s.close()
try:
 now=os.lstat(p)
 if (st.st_dev,st.st_ino)!=(now.st_dev,now.st_ino): sys.exit(78)
 os.unlink(p)
except FileNotFoundError: pass
`
