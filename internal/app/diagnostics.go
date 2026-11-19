package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/diagnostics"
	"github.com/UcGeorge-Canonical/easy-lima/internal/environment"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

func checkError(code, success string, scope model.Scope, err error) model.Check {
	if err == nil {
		return model.NewCheck(code, "passed", success, scope)
	}
	e := model.AsError(err)
	status := "failed"
	switch e.Exit {
	case 3, 5, 8, 9:
		status = "unavailable"
	case 6:
		status = "unknown"
	}
	if e.Code == "provider.unverified" {
		status = "unverified"
	}
	c := model.NewCheck(code, status, e.Message, scope)
	c.NextAction = e.NextAction
	return c
}
func (s Service) profileChecks(ctx context.Context, p store.ProfileRecord, online bool) []model.Check {
	scope := model.Scope{Kind: "profile", ID: &p.Profile.ID}
	out := []model.Check{}
	if p.Profile.Fingerprint == nil {
		return append(out, model.NewCheck("profile.agent", "unknown", "The committed public key is unavailable.", scope))
	}
	c, err := agent.Connect(ctx, p.Agent.SocketPath)
	out = append(out, checkError("profile.agent", "The selected profile agent is reachable.", scope, err))
	if err != nil {
		out = append(out, model.NewCheck("profile.capabilities", "skipped", "No agent was activated by this check.", scope), model.NewCheck("identity.signing", "skipped", "The signing challenge requires the existing agent.", scope))
		if online {
			out = append(out, model.NewCheck("provider.authentication", "unverified", "The profile agent is unavailable; provider authentication was not attempted.", scope), model.NewCheck("provider.account", "unverified", "The provider account could not be checked.", scope))
		}
		return out
	}
	defer c.Close()
	err = c.Verify(*p.Profile.Fingerprint)
	out = append(out, checkError("profile.capabilities", "The agent exposes exactly the selected profile key.", scope, err))
	if err == nil {
		out = append(out, checkError("identity.signing", "An in-memory signing challenge verified.", scope, c.SignProbe(*p.Profile.Fingerprint)))
	} else {
		out = append(out, model.NewCheck("identity.signing", "skipped", "The offered-key check must pass before signing.", scope))
	}
	if online && err == nil {
		providerErr := diagnostics.GitHub(ctx, c, p.Profile.GitHubUsername)
		authErr := providerErr
		if providerErr != nil && model.AsError(providerErr).Code == "provider.account_mismatch" {
			authErr = nil
		}
		out = append(out, checkError("provider.authentication", "GitHub authenticated the selected profile key.", scope, authErr))
		if p.Profile.GitHubUsername == nil {
			out = append(out, model.NewCheck("provider.account", "skipped", "No expected GitHub account is configured.", scope))
		} else {
			out = append(out, checkError("provider.account", "GitHub matched the profile's expected account.", scope, providerErr))
		}
	} else if online {
		out = append(out, model.NewCheck("provider.authentication", "unverified", "The offered-key check failed; provider authentication was not attempted.", scope), model.NewCheck("provider.account", "unverified", "The provider account could not be checked.", scope))
	} else {
		out = append(out, model.NewCheck("provider.authentication", "skipped", "Use --online to request provider authentication.", scope), model.NewCheck("provider.account", "skipped", "Use --online to request the provider account check.", scope))
	}
	return out
}
func (s Service) profileCheck(ctx context.Context, db *store.Store, r Request) model.Result {
	out := model.NewResult(r.Command)
	p, err := db.Profile(ctx, r.Label)
	if err != nil {
		out.SetError(err)
		return out
	}
	out.Checks = s.profileChecks(ctx, p, r.Online)
	out.Diagnostic(model.Scope{Kind: "profile", ID: &p.Profile.ID})
	return out
}
func (s Service) doctorUnavailable(ctx context.Context, r Request, cause error) model.Result {
	out := model.NewResult(r.Command)
	out.Checks = append(out.Checks, checkError("registry.readability", "The registry is readable.", model.Global(), cause))
	inventory := store.ReadInventory(s.Paths.State, r.Limit)
	status := "unavailable"
	message := "No complete recovery inventory is available."
	if len(inventory.Items) > 0 {
		status = "unknown"
		message = "Diagnostic inventory is readable; it cannot establish current ownership or authorize repairs."
	}
	out.Checks = append(out.Checks, model.NewCheck("inventory.freshness", status, message, model.Global()))
	out.Diagnostic(model.Global())
	out.Data.(map[string]any)["recovery_inventory"] = inventory
	return out
}
func (s Service) doctor(ctx context.Context, db *store.Store, r Request) model.Result {
	out := model.NewResult(r.Command)
	scope := model.Global()
	out.Checks = append(out.Checks, checkError("registry.publication", "The authoritative critical projection and local witness agree.", scope, db.VerifyHead(ctx)))
	v, err := s.resolveVM(ctx, db, r)
	if err != nil {
		if isNotFound(err) {
			out.SetError(err)
			return out
		}
		out.Checks = append(out.Checks, checkError("registry.readability", "The registration is readable.", scope, err))
		out.Diagnostic(scope)
		return out
	}
	scope = model.VMScope(v.VM.ID, v.VM.SourcePath)

	_, pathErr := os.Stat(v.VM.SourcePath)
	if pathErr != nil {
		pathErr = model.Fail("check.incomplete", "The recorded source folder is unavailable; use explicit rebind after a move.")
	}
	out.Checks = append(out.Checks, checkError("project.source", "The source folder is available.", scope, pathErr))
	n, err := db.Namespace(ctx)
	if err == nil {
		err = storage.CheckRoot(n)
	}
	out.Checks = append(out.Checks, checkError("storage.presence", "The selected volume and root match their recorded identities.", scope, err))
	rootOK := err == nil
	if rootOK {
		out.Checks = append(out.Checks, checkError("ownership.consistency", "The registration and owner markers agree.", scope, s.verifyOwnership(ctx, db, v)))
	} else {
		out.Checks = append(out.Checks, model.NewCheck("ownership.consistency", "unavailable", "Storage is unavailable; ownership disagreement was not established.", scope))
	}
	in, envErr := decodeEnvironment(v)
	if !rootOK {
		envErr = model.Fail("storage.unavailable", "Environment inputs cannot be verified while storage is unavailable.")
	}
	if envErr == nil && rootOK {
		envErr = s.verifyNative(v, in)
	}
	out.Checks = append(out.Checks, checkError("environment.stages", "The frozen environment inputs match the instance configuration.", scope, envErr))
	if rootOK && in.RevisionID != "" {
		out.Checks = append(out.Checks, checkError("environment.namespace", "Namespace default and override inputs match the frozen environment.", scope, lima.CheckGlobals(n.Root, in.Globals, false)))
	} else {
		out.Checks = append(out.Checks, model.NewCheck("environment.namespace", "unavailable", "Namespace inputs cannot be compared without the recorded environment and storage.", scope))
	}
	out.Checks = append(out.Checks, checkError("vm.alias", "The managed SSH alias is intact.", scope, connection.VerifyAlias(ctx, db, s.Paths.Home, v)), checkError("vm.trust_export", "The exported pinned host key is intact.", scope, connection.VerifyTrustExport(ctx, db, v.VM.ID)))
	if v.VM.ProfileID != nil {
		p, e := db.Profile(ctx, *v.VM.ProfileID)
		if e != nil {
			out.Checks = append(out.Checks, checkError("profile.record", "The profile is readable.", scope, e))
		} else {
			out.Checks = append(out.Checks, s.profileChecks(ctx, p, r.Online)...)
		}
	}
	u, err := s.user(r)
	out.Checks = append(out.Checks, checkError("config.readability", "User configuration is readable.", scope, err))
	running := false
	if err == nil && rootOK {
		a, e := s.adapter(u.Value, n, r, "diagnostic")
		err = e
		out.Checks = append(out.Checks, checkError("lima.executable", "The selected Lima executable is available.", scope, err))
		if err == nil {
			_, err = a.Probe(ctx)
			out.Checks = append(out.Checks, checkError("lima.compatibility", "The installed Lima CLI matches this adapter contract.", scope, err))
			if err == nil {
				obs, e := a.Observe(ctx, v.VM.InstanceName)
				err = e
				running = e == nil && obs.Status == "Running"
				out.Checks = append(out.Checks, checkError("vm.state", "Lima reports the VM as "+strings.ToLower(obs.Status)+".", scope, e))
			}
		}
	}
	if !running {
		out.Checks = append(out.Checks, model.NewCheck("vm.ssh", "unavailable", "The VM is not observably running; diagnostics never start it.", scope))
		out.Checks = append(out.Checks, unavailableGuestChecks(v, in, scope)...)
		out.Diagnostic(scope)
		return out
	}
	c, err := s.pinned(ctx, db, v)
	out.Checks = append(out.Checks, checkError("vm.host_key", "The VM completed a handshake with its pinned host key.", scope, err))
	if err != nil {
		out.Checks = append(out.Checks, model.NewCheck("vm.ssh", "unavailable", "Pinned SSH could not be established.", scope))
		out.Checks = append(out.Checks, unavailableGuestChecks(v, in, scope)...)
		out.Diagnostic(scope)
		return out
	}
	defer c.Close()
	out.Checks = append(out.Checks, model.NewCheck("vm.ssh", "passed", "The pinned SSH connection is available.", scope))
	out.Checks = append(out.Checks, s.portChecks(ctx, in.Settings.Ports, scope)...)
	if v.PendingIdentityKind != nil {
		out.Checks = append(out.Checks, model.NewCheck("identity.git_settings", "unknown", "An authorized identity change is pending; use an explicit start/open to finish it.", scope))
	} else if v.AppliedProfileID != nil {
		var raw string
		e := db.DB.QueryRowContext(ctx, "SELECT guest_identity FROM profile_revisions WHERE id=?", *v.VM.AppliedIdentityRevisionID).Scan(&raw)
		if e == nil {
			git, de := store.Decode[model.GitIdentity](raw)
			e = de
			if e == nil {
				e = identity.VerifyGit(ctx, c, git)
			}
		}
		out.Checks = append(out.Checks, checkError("identity.git_settings", "The effective Git settings match the applied identity revision.", scope, e))
		p, e := db.Profile(ctx, *v.AppliedProfileID)
		if e == nil {
			e = connection.ProbeForwarding(ctx, c, v.VM.ID, *p.Profile.Fingerprint)
		}
		out.Checks = append(out.Checks, checkError("identity.forwarding", "The guest socket offers the exact profile key.", scope, e))
	}
	_, err = c.Output(ctx, "systemctl", "is-system-running", "--wait")
	out.Checks = append(out.Checks, checkError("guest.systemd", "Guest systemd is running.", scope, err))
	for _, preset := range in.Settings.Environment.Presets {
		switch preset {
		case "development":
			for _, name := range []string{"snapd", "snap.lxd.daemon"} {
				_, e := c.Output(ctx, "systemctl", "is-active", name)
				out.Checks = append(out.Checks, checkError("guest."+name, "The service is active.", scope, e))
			}
		case "docker":
			_, e := c.Output(ctx, "systemctl", "is-active", "docker")
			out.Checks = append(out.Checks, checkError("guest.docker", "Docker is active.", scope, e))
		}
	}
	if v.AppliedProfileID != nil && v.VM.AppliedIdentityRevisionID != nil {
		var raw string
		e := db.DB.QueryRowContext(ctx, "SELECT guest_identity FROM profile_revisions WHERE id=?", *v.VM.AppliedIdentityRevisionID).Scan(&raw)
		if e == nil {
			git, de := store.Decode[model.GitIdentity](raw)
			if de == nil {
				coverage, ce := identity.CheckRepositories(ctx, c, git, r.Repositories, len(r.Repositories) == 0)
				if ce != nil {
					out.Checks = append(out.Checks, checkError("repository.coverage", "", scope, ce))
				} else {
					for _, repo := range coverage.Repositories {
						status, message := "passed", "Effective identity checked: "+repo.Path
						if !repo.Repository {
							message = "Valid workspace folder (not a Git repository); global identity checked: " + repo.Path
						}
						if repo.Unavailable {
							status = "unavailable"
							message = "Cannot inspect requested directory: " + repo.Path
						}
						if len(repo.Conflicts) > 0 {
							status = "failed"
							conflict := repo.Conflicts[0]
							message = fmt.Sprintf("%s: %s conflicts (source: %s); user overrides preserved.", repo.Path, conflict.Field, conflict.Origin)
						}
						out.Checks = append(out.Checks, model.NewCheck("repository.identity", status, message, scope))
					}
					status := "passed"
					message := fmt.Sprintf("Examined %d directories/repositories (maximum 32 repos and 256 directories).", len(coverage.Repositories))
					if coverage.Truncated {
						status = "unknown"
						message += " Additional paths were not examined; pass --repository /workspace/path explicitly."
					}
					out.Checks = append(out.Checks, model.NewCheck("repository.coverage", status, message, scope))
				}
			}
		}
	} else {
		out.Checks = append(out.Checks, model.NewCheck("repository.coverage", "skipped", "No applied Git profile; ordinary non-repository folders are valid workspaces.", scope))
	}
	out.Diagnostic(scope)
	return out
}

func unavailableGuestChecks(v store.VMRecord, in environment.Input, scope model.Scope) []model.Check {
	codes := []string{"ports.readiness", "guest.systemd"}
	if v.VM.ProfileID != nil || v.PendingIdentityKind != nil {
		codes = append(codes, "identity.git_settings", "identity.forwarding", "identity.signing", "repository.coverage")
	}
	for _, preset := range in.Settings.Environment.Presets {
		if preset == "development" {
			codes = append(codes, "guest.snapd", "guest.snap.lxd.daemon")
		}
		if preset == "docker" {
			codes = append(codes, "guest.docker")
		}
	}
	out := make([]model.Check, 0, len(codes))
	for _, code := range codes {
		out = append(out, model.NewCheck(code, "unavailable", "Not observed because the VM's pinned SSH connection is unavailable.", scope))
	}
	return out
}
func (s Service) storageInspect(ctx context.Context, db *store.Store) (any, error) {
	n, err := db.Namespace(ctx)
	if err != nil {
		return nil, err
	}
	available := storage.CheckRoot(n) == nil
	paths := []map[string]any{}
	runningCPUs := 0
	var runningMemory int64
	groups := []map[string]any{}
	rows, err := db.DB.QueryContext(ctx, "SELECT id,instance_name FROM registrations WHERE retired_at IS NULL ORDER BY id")
	if err != nil {
		return nil, err
	}
	ids := [][2]string{}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, [2]string{id, name})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		v, e := db.VM(ctx, id[0])
		path := filepath.Join(n.Root, id[1])
		item := map[string]any{"vm_id": id[0], "path": path, "allocated_bytes": nil, "disk_max_bytes": nil}
		if e == nil {
			item["disk_max_bytes"] = v.VM.Resources.DiskMaxBytes
			if v.VM.State.Recorded == "running" {
				if v.VM.Resources.CPUs != nil {
					runningCPUs += *v.VM.Resources.CPUs
				}
				if v.VM.Resources.MemoryBytes != nil {
					runningMemory += *v.VM.Resources.MemoryBytes
				}
			}
		}
		if available {
			if size, e := files.Allocated(path); e == nil {
				item["allocated_bytes"] = size
			}
		}
		paths = append(paths, item)
	}
	seen := map[string]bool{}
	for _, path := range []string{n.Root, s.Paths.State} {
		if path == n.Root && !available {
			continue
		}
		v, e := files.InspectVolume(path)
		if e != nil || seen[v.CapacityGroup] {
			continue
		}
		seen[v.CapacityGroup] = true
		var pending int64
		e = db.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(remaining_bytes),0) FROM capacity_claims WHERE capacity_group_id=? AND state='held'", v.CapacityGroup).Scan(&pending)
		if e != nil {
			return nil, e
		}
		groups = append(groups, map[string]any{"id": v.CapacityGroup, "available_bytes": v.AvailableBytes, "pending_bytes": pending, "quota_known": v.QuotaKnown})
	}
	var cacheBytes *int64
	cachePaths := []string{filepath.Join(n.Root, "_easy-lima", "assets"), filepath.Join(n.Root, "_easy-lima", "cache")}
	if available {
		var total int64
		complete := true
		for _, path := range cachePaths {
			size, e := files.Allocated(path)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				complete = false
				break
			}
			total += size
		}
		if complete {
			cacheBytes = &total
		}
	}
	return map[string]any{"recorded_running_cpus": runningCPUs, "recorded_running_memory_bytes": runningMemory, "cache_allocated_bytes": cacheBytes, "measurement_time": model.Now(), "root": n.Root, "namespace_id": n.ID, "available": available, "vm_paths": paths, "capacity_groups": groups, "cache_paths": cachePaths, "temporary_paths": []string{filepath.Join(n.Root, "_easy-lima", "tmp")}, "future_growth_reserved": false}, nil
}
func (s Service) explain(ctx context.Context, db *store.Store, r Request) (any, error) {
	p, err := s.selected(r, true)
	if err != nil {
		return nil, err
	}
	u, err := s.user(r)
	if err != nil {
		return nil, err
	}
	settings, _, fields, err := s.creationSettings(ctx, r, u.Value, p)
	if err != nil {
		return nil, err
	}
	target := model.Scope{Kind: "record", Path: &p}
	diff := []model.Field{}
	v, e := db.ResolveVM(ctx, "", p)
	if e == nil {
		target = model.VMScope(v.VM.ID, p)
		in, e := decodeEnvironment(v)
		if e != nil {
			return nil, e
		}
		diff = environment.Diff(in.Settings, settings)
	} else if !isNotFound(e) {
		return nil, e
	}
	return map[string]any{"target": target, "fields": fields, "differences": diff, "complete": true}, nil
}

type logEntry struct {
	Time        string  `json:"time"`
	OperationID string  `json:"operation_id"`
	Stage       *string `json:"stage"`
	Code        string  `json:"code"`
	Message     string  `json:"message"`
}

func (s Service) logs(ctx context.Context, db *store.Store, r Request) (any, error) {
	v, err := s.resolveVM(ctx, db, r)
	if err != nil {
		return nil, err
	}
	after, gen, err := db.Page(ctx, "logs", v.VM.ID, r.Cursor)
	if err != nil {
		return nil, err
	}
	entries := []logEntry{}
	var next *string
	read := func() error {
		rows, e := db.DB.QueryContext(ctx, `SELECT e.sequence,e.created_at,e.operation_id,COALESCE(json_extract(e.payload,'$.result_code'),e.kind),json_extract(e.payload,'$.stage'),COALESCE(json_extract(e.payload,'$.effect_state'),'') FROM namespace_events e JOIN operations o ON o.id=e.operation_id WHERE o.scope_id=? AND e.sequence>CAST(? AS INTEGER) ORDER BY e.sequence LIMIT ?`, v.VM.ID, after, r.Limit+1)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var seq int64
			var entry logEntry
			var effect string
			if e = rows.Scan(&seq, &entry.Time, &entry.OperationID, &entry.Code, &entry.Stage, &effect); e != nil {
				return e
			}
			if len(entries) == r.Limit {
				next = store.LogCursor(gen, v.VM.ID, after)
				break
			}
			entry.Message = "Recorded " + entry.Code + " state transition."
			if entry.Stage != nil {
				entry.Message = *entry.Stage + ": " + effect + " (" + entry.Code + ")"
			}
			entries = append(entries, entry)
			after = fmt.Sprint(seq)
		}
		return rows.Err()
	}
	if err = read(); err != nil {
		return nil, err
	}
	if r.Follow {
		for _, e := range entries {
			fmt.Fprintf(s.Out, "%s %s %s\n", e.Time, e.OperationID, e.Message)
		}
		for {
			select {
			case <-ctx.Done():
				return nil, nil
			case <-time.After(time.Second):
				fresh, e := store.Open(ctx, s.Paths.State, false)
				if e != nil {
					return nil, e
				}
				old := db
				db = fresh
				entries = nil
				err = read()
				db = old
				fresh.Close()
				if err != nil {
					return nil, err
				}
				for _, e := range entries {
					fmt.Fprintf(s.Out, "%s %s %s\n", e.Time, e.OperationID, e.Message)
				}
			}
		}
	}
	captures := []string{}
	ops, er := db.DB.QueryContext(ctx, "SELECT id FROM operations WHERE scope_id=? ORDER BY created_at DESC LIMIT 1000", v.VM.ID)
	if er != nil {
		return nil, er
	}
	defer ops.Close()
	for ops.Next() {
		var id string
		if er = ops.Scan(&id); er != nil {
			return nil, er
		}
		p := filepath.Join(db.Dir, "logs", id+".raw.log")
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
			captures = append(captures, p)
		}
	}
	return map[string]any{"entries": entries, "next_cursor": next, "raw_capture_paths": captures, "runtime_log_paths": []string{filepath.Join(v.VM.Storage.InstanceDirectory, "ha.stderr.log"), filepath.Join(v.VM.Storage.InstanceDirectory, "serialv.log")}}, nil
}
