package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/assets"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

func (s Service) pinned(ctx context.Context, db *store.Store, v store.VMRecord) (*connection.Client, error) {
	e, err := connection.ReadEndpoint(v.VM.Storage.Root, v.VM.InstanceName, v.VM.ID)
	if err != nil {
		return nil, err
	}
	t, err := connection.ReadTrust(ctx, db, v.VM.ID)
	if err != nil {
		return nil, err
	}
	key, err := t.Key()
	if err != nil {
		return nil, err
	}
	return connection.Dial(ctx, e, key, false)
}
func (s Service) completeGuestIdentity(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord, c *connection.Client, r Request) error {
	if v.PendingIdentityKind == nil {
		return nil
	}
	if v.IdentityPhase == nil || *v.IdentityPhase != "committed" {
		return model.Fail("operation.busy", "The profile membership transition must be reconciled before guest identity changes.")
	}
	if *v.PendingIdentityKind == "none" {
		if err := identity.Apply(ctx, c, identity.Bundle{Action: "remove", Files: map[string]string{}}); err != nil {
			return err
		}
	} else {
		if v.PendingProfileID == nil || v.VM.PendingIdentityRevisionID == nil {
			return model.Fail("record.invalid", "The pending identity target is incomplete.")
		}
		var public, raw string
		err := db.DB.QueryRowContext(ctx, "SELECT c.public_key,i.guest_identity FROM profile_revisions i JOIN profile_credential_revisions c ON c.id=i.credential_revision_id WHERE i.profile_id=? AND i.id=?", *v.PendingProfileID, *v.VM.PendingIdentityRevisionID).Scan(&public, &raw)
		if err != nil {
			return err
		}
		git, err := store.Decode[model.GitIdentity](raw)
		if err != nil {
			return err
		}
		home, err := c.Output(ctx, "sh", "-c", `printf '%s' "$HOME"`)
		if err != nil {
			return err
		}
		bundle, err := identity.Build(v.VM.ID, string(home), *v.VM.PendingIdentityRevisionID, public, git)
		if err != nil {
			return err
		}
		legacy, e := db.Setting(ctx, v.VM.ID, "guest:canonical-local", "legacy-identity")
		if e != nil {
			return e
		}
		if legacy != "" && legacy != "retired" {
			p, e := db.Profile(ctx, *v.PendingProfileID)
			if e != nil {
				return e
			}
			account := ""
			if p.Profile.GitHubUsername != nil {
				account = *p.Profile.GitHubUsername
			}
			if e = identity.RetireLegacy(ctx, c, git, public, account); e != nil {
				return e
			}
			if e = db.OwnSetting(ctx, op.Operation.ID, v.VM.ID, "vm", "guest:canonical-local", "legacy-identity", "retired"); e != nil {
				return e
			}
		}
		if err = identity.Apply(ctx, c, bundle); err != nil {
			return err
		}
		if err = identity.VerifyGit(ctx, c, git); err != nil {
			return err
		}
		if legacy != "" {
			if e = db.Change(ctx, store.Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
				_, e := tx.ExecContext(ctx, "DELETE FROM owned_settings WHERE owner_id=? AND field='legacy-identity'", v.VM.ID)
				return e
			}); e != nil {
				return e
			}
		}

	}
	if v.PendingProfileID != nil {
		if err := db.HoldProfile(ctx, op, *v.PendingProfileID); err != nil {
			return err
		}
		p, err := db.Profile(ctx, *v.PendingProfileID)
		if err != nil {
			return err
		}
		if err = s.ensureProfileForConnection(ctx, db, op, p, r); err != nil {
			return err
		}
		p, err = db.Profile(ctx, p.Profile.ID)
		if err != nil {
			return err
		}
		// The pending tuple remains the public gate until this exact prospective binding is ready.
		prospective := v
		prospective.PendingIdentityKind = nil
		prospective.AppliedProfileID = v.PendingProfileID
		prospective.VM.AppliedIdentityRevisionID = v.VM.PendingIdentityRevisionID
		prospective.AppliedProfileBindingGeneration = &prospective.ProfileBindingGeneration
		if err = connection.EnsureTunnel(ctx, db, op.Operation.ID, s.Paths.Cache, prospective, p, c.Endpoint, c); err != nil {
			return err
		}
	}
	if err := db.CompleteIdentity(ctx, *v.IdentityOperationID, v.VM.ID); err != nil {
		return err
	}
	continuation, err := db.Operation(ctx, *v.IdentityOperationID)
	if err != nil {
		return err
	}
	if continuation.Operation.Kind == "identity.complete" {
		return db.Finish(ctx, continuation, "completed")
	}
	return nil
}
func (s Service) prepareConnection(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord, in environment.Input, r Request, identityRequired bool) error {
	if v.VM.ProfileID != nil {
		if err := db.HoldProfile(ctx, op, *v.VM.ProfileID); err != nil {
			return err
		}
	}
	if v.ActiveTrustGeneration == nil {
		var pending *string
		if e := db.DB.QueryRowContext(ctx, "SELECT pending_trust_operation_id FROM registrations WHERE id=?", v.VM.ID).Scan(&pending); e != nil {
			return e
		}
		if pending != nil {
			consent, e := db.Operation(ctx, *pending)
			if e != nil {
				return e
			}
			if consent.Operation.Kind != "adopt" {
				return model.Fail("trust.mismatch", "Host key enrollment requires the original explicit adoption intent.")
			}
			if e = s.verifyOwnership(ctx, db, v); e != nil {
				return e
			}
			endpoint, e := connection.ReadEndpoint(v.VM.Storage.Root, v.VM.InstanceName, v.VM.ID)
			if e != nil {
				return e
			}
			if _, e = connection.Enroll(ctx, db, consent, v, endpoint); e != nil {
				return e
			}
			v, e = db.VM(ctx, v.VM.ID)
			if e != nil {
				return e
			}
		}
	}
	if err := connection.VerifyTrustExport(ctx, db, v.VM.ID); err != nil {
		return err
	}
	c, err := s.pinned(ctx, db, v)
	if err != nil {
		return err
	}
	defer c.Close()
	workspaceDigest := files.Hash([]byte(v.VM.SourcePath + fmt.Sprint(v.BindingGeneration)))
	verifiedWorkspace, err := db.Setting(ctx, v.VM.ID, "guest:workspace", "binding")
	if err != nil {
		return err
	}
	if verifiedWorkspace != workspaceDigest {
		if err = verifyWorkspace(ctx, c, v.VM.SourcePath, op); err != nil {
			return err
		}
		if err = db.OwnSetting(ctx, op.Operation.ID, v.VM.ID, "vm", "guest:workspace", "binding", workspaceDigest); err != nil {
			return err
		}
	}
	transport, err := db.Setting(ctx, v.VM.ID, "guest:transport", "transport")
	if err != nil {
		return err
	}
	if transport == "" {
		script, e := assets.Read("guest/transport.py")
		if e != nil {
			return e
		}
		steps, e := db.Steps(ctx, op.Operation.ID)
		if e != nil {
			return e
		}
		for _, step := range steps {
			if step.Stage != "connection.transport" {
				continue
			}
			var evidence struct {
				Digest string `json:"helper_sha256"`
			}
			if json.Unmarshal(step.Evidence, &evidence) != nil || len(evidence.Digest) != 64 {
				return model.Fail("record.invalid", "Transport recovery evidence is incomplete.")
			}
			if evidence.Digest != files.Hash(script) {
				script, e = files.Read(filepath.Join(db.Dir, "artifacts", evidence.Digest+".py"), 256<<10)
				if e != nil || files.Hash(script) != evidence.Digest {
					return model.Fail("config.drift", "The previously admitted transport helper is unavailable; a new binary cannot replace it during recovery.")
				}
			}
		}
		if e = environment.MaterializeSmall(map[string][]byte{filepath.Join(db.Dir, "artifacts", files.Hash(script)+".py"): script}); e != nil {
			return e
		}
		if e = db.RecordStep(ctx, op, "connection.transport", "may_have_dispatched", "unknown", "transport.preparing", map[string]string{"helper_sha256": files.Hash(script)}); e != nil {
			return e
		}
		var out, stderr bytes.Buffer
		code, e := c.Run(ctx, []string{"sudo", "python3", "-c", string(script), v.VM.ID, c.Endpoint.User}, nil, &out, &stderr)
		if e != nil {
			return e
		}
		if code != 0 {
			return model.Fail("settings.edited", "The guest's transport files conflict with the admitted settings.")
		}
		if e = db.OwnSetting(ctx, op.Operation.ID, v.VM.ID, "vm", "guest:transport", "transport", files.Hash(script)); e != nil {
			return e
		}
		if e = db.RecordStep(ctx, op, "connection.transport", "exited", "satisfied", "transport.ready", map[string]string{"helper_sha256": files.Hash(script)}); e != nil {
			return e
		}
	}
	if !identityRequired {
		// VM-only starts may reuse a remembered or already loaded key, but do
		// not prompt for one or silently unlock an explicitly locked profile.
		r.Interactive = false
	}
	identityErr := func() error {
		if v.VM.ProfileID != nil {
			p, e := db.Profile(ctx, *v.VM.ProfileID)
			if e != nil {
				return e
			}
			if e = s.ensureProfileForConnection(ctx, db, op, p, r); e != nil {
				return e
			}
		}
		if v.PendingIdentityKind != nil {
			if err = s.completeGuestIdentity(ctx, db, op, v, c, r); err != nil {
				return err
			}
			v, err = db.VM(ctx, v.VM.ID)
			if err != nil {
				return err
			}
		}
		if v.AppliedProfileID != nil && v.VM.AppliedIdentityRevisionID != nil {
			var raw string
			if err = db.DB.QueryRowContext(ctx, "SELECT guest_identity FROM profile_revisions WHERE id=?", *v.VM.AppliedIdentityRevisionID).Scan(&raw); err != nil {
				return err
			}
			git, e := store.Decode[model.GitIdentity](raw)
			if e != nil {
				return e
			}
			if e = identity.VerifyGit(ctx, c, git); e != nil {
				return e
			}
		}
		if v.VM.ProfileID != nil {
			p, err := db.Profile(ctx, *v.VM.ProfileID)
			if err != nil {
				return err
			}
			if p.Profile.LockState != "locked" {
				if err = connection.EnsureTunnel(ctx, db, op.Operation.ID, s.Paths.Cache, v, p, c.Endpoint, c); err != nil {
					return err
				}
			} else {
				return model.Fail("profile.explicitly_locked", "The VM is running; its selected profile remains locked.")
			}
		}
		return nil
	}()
	if identityErr != nil {
		code := model.AsError(identityErr).Code
		if identityRequired || (!knownConnectionConflict(identityErr) && code != "check.incomplete" && code != "check.failed" && !strings.HasPrefix(code, "credential.")) {
			return identityErr
		}
		if err = db.RecordStep(ctx, op, "connection.identity", "none", "not_applied", code, nil); err != nil {
			return err
		}
	} else {
		if err = db.RecordStep(ctx, op, "connection.identity", "exited", "satisfied", "identity.ready", nil); err != nil {
			return err
		}
	}
	return connection.PublishAlias(ctx, db, op.Operation.ID, s.Paths.Home, s.Binary, v, c.Endpoint)
}
func (s Service) connect(ctx context.Context, db *store.Store, r Request) (data any, exit int, resultErr error) {
	v, err := s.resolveVM(ctx, db, r)
	if err != nil {
		return nil, 255, err
	}
	u, err := s.user(r)
	if err != nil {
		return nil, 255, err
	}
	if r.Command == "code" {
		editor := u.Value.Editor.Executable
		if editor == "" {
			editor = "code"
		}
		if _, e := exec.LookPath(editor); e != nil {
			return nil, 3, model.Fail("editor.unavailable", "Install the VS Code command-line launcher and Remote - SSH extension, or use easy-lima ssh.")
		}
	}
	n, err := s.validateRoot(ctx, db, u.Value)
	if err != nil {
		return nil, 255, err
	}
	a, err := s.adapter(u.Value, n, r, "connect")
	if err != nil {
		return nil, 255, err
	}
	if _, err = a.Probe(ctx); err != nil {
		return nil, 255, err
	}
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return nil, 255, err
	}
	in, err := decodeEnvironment(v)
	if err != nil {
		return nil, 255, err
	}
	if err = s.verifyNative(v, in); err != nil {
		return nil, 255, err
	}
	obs, err := a.Observe(ctx, v.VM.InstanceName)
	if err != nil {
		return nil, 255, err
	}
	if obs.Status != "Running" && (r.NoStart || r.Command == "forward" || (r.Command == "exec" && !r.Start)) {
		return nil, 255, model.Fail("check.incomplete", "The VM is not observably running; this command does not permit startup.")
	}
	if r.DryRun {
		plan := model.NewPlan(r.Command, model.VMScope(v.VM.ID, v.VM.SourcePath))
		if obs.Status != "Running" {
			plan.Add("vm_start", "Start the existing VM when this command permits it.")
		}
		plan.Add("connection", "Prepare the pinned transport and selected profile connection.")
		return model.Mutation(plan), 0, nil
	}
	if obs.Status != "Running" {
		start := r
		start.Command = "start"
		start.CreationFlags = false
		start.Settings = config.Settings{}
		start.Yes = true
		if _, err = s.lifecycle(ctx, db, start); err != nil {
			return nil, 255, err
		}
		v, err = db.VM(ctx, v.VM.ID)
		if err != nil {
			return nil, 255, err
		}
	}
	if v.VM.ProfileID != nil {
		p, err := db.Profile(ctx, *v.VM.ProfileID)
		if err != nil {
			return nil, 255, err
		}
		if err = s.activateProfile(ctx, db, p, r, false); err != nil {
			return nil, 255, err
		}
	}
	plan := model.NewPlan(r.Command, model.VMScope(v.VM.ID, v.VM.SourcePath))
	plan.Add("connection", "Prepare the pinned VM connection and selected identity.")
	op, err := db.Prepare(ctx, plan, LifecycleInput{VMID: v.VM.ID, Environment: in}, []store.Claim{{Kind: "vm", Key: v.VM.ID, Mode: "shared"}}, nil)
	if err != nil {
		return nil, 255, err
	}
	defer func() {
		if resultErr != nil && data == nil {
			m := model.Mutation(plan)
			data = s.operationResult(context.WithoutCancel(ctx), db, m, op)
		}
	}()
	raw, err := capture(db, op, r)
	if err != nil {
		return nil, 255, err
	}
	if raw != nil {
		defer raw.Close()
		s.Out = io.MultiWriter(s.Out, raw)
		s.Err = io.MultiWriter(s.Err, raw)
	}
	guard, err := operationGuard(ctx, db, op.Operation.ID)
	if err != nil {
		return nil, 255, err
	}
	defer guard.Close()
	if err = s.prepareConnection(ctx, db, op, v, in, r, true); err != nil {
		if knownConnectionConflict(err) {
			_ = db.Finish(context.WithoutCancel(ctx), op, "failed_known")
		}
		return nil, 255, err
	}
	if err = db.Finish(ctx, op, "completed"); err != nil {
		return nil, 255, err
	}
	if r.Command == "code" {
		if err = connection.VerifyAlias(ctx, db, s.Paths.Home, v); err != nil {
			return nil, 255, err
		}
		editor := u.Value.Editor.Executable
		if editor == "" {
			editor = "code"
		}
		p, err := exec.LookPath(editor)
		if err != nil {
			return nil, 3, model.Fail("editor.unavailable", "Install the VS Code command-line launcher and Remote - SSH extension, or use easy-lima ssh.")
		}
		args := append([]string{}, u.Value.Editor.Args...)
		for i := range args {
			args[i] = strings.ReplaceAll(strings.ReplaceAll(args[i], "{alias}", v.VM.SSHAlias), "{workspace}", "/workspace")
		}
		result, err := s.Runner.Run(ctx, process.Request{Executable: p, Args: args, Env: process.Environment(nil), Timeout: 20 * time.Second, Limit: 64 << 10, Stdout: s.Out, Stderr: s.Err})
		if err != nil || result.ExitCode != 0 {
			return nil, 7, model.Fail("operation.failed", "VS Code did not accept the remote-window launch.")
		}
		return map[string]any{"launched": true, "connection_observed": false, "ssh_alias": v.VM.SSHAlias}, 0, nil
	}
	if r.Command == "ssh" {
		if err = connection.VerifyAlias(ctx, db, s.Paths.Home, v); err != nil {
			return nil, 255, err
		}
		result, err := s.Runner.Run(ctx, process.Request{Executable: "ssh", Args: []string{"-t", v.VM.SSHAlias, "cd /workspace && exec \"$SHELL\" -l"}, Env: process.Environment(nil), Stdin: s.In, Stdout: s.Out, Stderr: s.Err, Limit: 4096})
		return nil, result.ExitCode, err
	}
	c, err := s.pinned(ctx, db, v)
	if err != nil {
		return nil, 255, err
	}
	defer c.Close()
	if r.Command == "forward" {
		listener, err := net.Listen("tcp", net.JoinHostPort(r.Bind, fmt.Sprint(r.LocalPort)))
		if err != nil {
			return nil, 9, model.Fail("port.unavailable", "The requested localhost port could not be bound.")
		}
		defer listener.Close()
		go func() { <-ctx.Done(); listener.Close() }()
		slots := make(chan struct{}, 128)
		for {
			local, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return nil, 0, nil
				}
				return nil, 7, err
			}
			select {
			case slots <- struct{}{}:
			default:
				local.Close()
				continue
			}
			go func() {
				defer func() { <-slots }()
				defer local.Close()
				remote, err := c.SSH.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(r.GuestPort)))
				if err != nil {
					return
				}
				defer remote.Close()
				done := make(chan struct{}, 1)
				go func() {
					_, _ = io.Copy(remote, local)
					if tcp, ok := remote.(interface{ CloseWrite() error }); ok {
						_ = tcp.CloseWrite()
					}
					done <- struct{}{}
				}()
				_, _ = io.Copy(local, remote)
				local.Close()
				<-done
			}()
		}
	}
	exit, err = c.ExecWorkspace(ctx, r.GuestArgs, s.In, s.Out, s.Err)
	return nil, exit, err
}
func (s Service) revokeMembers(ctx context.Context, db *store.Store, op store.OperationRecord, ids []string) error {
	for _, id := range ids {
		v, err := db.VM(ctx, id)
		if err != nil {
			return err
		}
		_, err = connection.ReadTunnel(ctx, db, id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		u, er := s.user(Request{})
		if er != nil {
			return er
		}
		n, er := db.Namespace(ctx)
		if er != nil {
			return er
		}
		a, er := s.adapter(u.Value, n, Request{}, op.Operation.ID)
		if er != nil {
			return er
		}
		obs, er := a.Observe(ctx, v.VM.InstanceName)
		if er != nil {
			return er
		}
		if obs.Status == "Stopped" {
			if er = connection.RetireStoppedTunnel(ctx, db, op.Operation.ID, id); er != nil {
				return er
			}
			continue
		}
		e, err := connection.ReadEndpoint(v.VM.Storage.Root, v.VM.InstanceName, id)
		if err != nil {
			return err
		}
		if err = connection.RevokeTunnel(ctx, db, op.Operation.ID, v, e); err != nil {
			return err
		}
	}
	return nil
}

func connectionRetireStopped(ctx context.Context, db *store.Store, op store.OperationRecord, v store.VMRecord) error {
	return connection.RetireStoppedTunnel(ctx, db, op.Operation.ID, v.VM.ID)
}
func (s Service) Proxy(ctx context.Context, id string) error {
	if !model.ValidID(id) {
		return model.Fail("input.invalid", "Invalid immutable VM ID.")
	}
	db, err := store.Open(ctx, s.Paths.State, true)
	if err != nil {
		return err
	}
	defer db.Close()
	v, err := db.VM(ctx, id)
	if err != nil {
		return err
	}
	if err = s.verifyOwnership(ctx, db, v); err != nil {
		return err
	}
	if v.PendingIdentityKind != nil {
		return model.Fail("profile.binding_required", "Run easy-lima up to complete pending guest identity before using the alias.")
	}
	n, err := db.Namespace(ctx)
	if err != nil {
		return err
	}
	if err = storage.CheckRoot(n); err != nil {
		return err
	}
	if err = connection.VerifyTrustExport(ctx, db, id); err != nil {
		return err
	}
	c, err := s.pinned(ctx, db, v)
	if err != nil {
		return err
	}
	defer c.Close()
	if v.VM.ProfileID != nil {
		p, err := db.Profile(ctx, *v.VM.ProfileID)
		if err != nil {
			return err
		}
		if err = s.activateProfile(ctx, db, p, Request{PassphraseFD: -1}, false); err != nil {
			return err
		}
		p, err = db.Profile(ctx, p.Profile.ID)
		if err != nil {
			return err
		}
		// Direct SSH/VS Code connections may renew the selected profile's
		// transport, but never start a VM or run environment provisioning.
		t, e := connection.ReadTunnel(ctx, db, id)
		ready := e == nil && t.State == "ready" && t.RuntimeGeneration == p.RuntimeGeneration
		if ready {
			ready = connection.ProbeForwarding(ctx, c, id, *p.Profile.Fingerprint) == nil
		}
		if !ready {
			plan := model.NewPlan("ssh", model.VMScope(v.VM.ID, v.VM.SourcePath))
			plan.Add("connection", "Renew the existing selected profile transport.")
			env, e := decodeEnvironment(v)
			if e != nil {
				return e
			}
			op, e := db.Prepare(ctx, plan, LifecycleInput{VMID: v.VM.ID, Environment: env}, []store.Claim{{Kind: "vm", Key: id, Mode: "exclusive"}, {Kind: "profile", Key: p.Profile.ID, Mode: "shared"}}, nil)
			if e != nil {
				return e
			}
			guard, e := operationGuard(ctx, db, op.Operation.ID)
			if e != nil {
				return e
			}
			defer guard.Close()
			if e = connection.EnsureTunnel(ctx, db, op.Operation.ID, s.Paths.Cache, v, p, c.Endpoint, c); e != nil {
				if knownConnectionConflict(e) {
					_ = db.Finish(context.WithoutCancel(ctx), op, "failed_known")
				}
				return e
			}
			if e = db.Finish(ctx, op, "completed"); e != nil {
				return e
			}
		}
	}
	endpoint := c.Endpoint
	conn, err := net.DialTimeout("tcp", endpoint.AddressPort(), 10*time.Second)
	if err != nil {
		return model.Fail("check.incomplete", "The registered VM endpoint is unavailable.")
	}
	defer conn.Close()
	done := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(conn, s.In)
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	_, err = io.Copy(s.Out, conn)
	conn.Close()
	return err
}
