package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func (s Service) maintenance(ctx context.Context, db *store.Store, r Request) (any, error) {
	reg, err := db.Registry(ctx)
	if err != nil {
		return nil, err
	}
	plan := model.NewPlan(r.Command, model.Global())
	switch r.Command {
	case "state.backup":
		destination := r.To
		if destination == "" {
			destination = filepath.Join(db.Dir, "backups", model.NewID())
		} else {
			destination, err = config.ResolvePath(destination, s.CWD)
			if err != nil {
				return nil, err
			}
		}
		plan.Add("file_publication", "Create a verified standalone metadata backup with its referenced public artifacts; no VM disks or private keys are copied.")
		plan.Fields = append(plan.Fields, model.Field{Path: "backup.destination", Value: destination, Origin: "flag"})
		data := map[string]any{"plan": plan, "backup_id": nil, "path": destination, "recovery_generation": reg.Generation, "verified": false}
		if r.DryRun {
			return data, nil
		}
		if err = s.approve(r, plan); err != nil {
			return data, err
		}
		m, err := db.Backup(ctx, destination)
		if err == nil {
			data["backup_id"] = m.BackupID
			data["verified"] = true
		}
		return data, err
	case "state.checkpoint":
		mode := r.CheckpointMode
		if mode == "" {
			mode = "passive"
		}
		duration := 5 * time.Second
		if r.Timeout != "" {
			duration, err = time.ParseDuration(r.Timeout)
			if err != nil || duration <= 0 || duration > time.Minute {
				return nil, model.Fail("input.invalid", "Checkpoint timeout must be positive and at most 60s.")
			}
		}
		plan.Add("checkpoint", "Explicitly checkpoint the registry using only the requested mode.")
		data := map[string]any{"plan": plan, "mode": mode, "frames_total": nil, "frames_checkpointed": nil, "status": "not_run"}
		if r.DryRun {
			return data, nil
		}
		if err = s.approve(r, plan); err != nil {
			return data, err
		}
		total, done, err := db.Checkpoint(ctx, mode, duration)
		data["frames_total"] = total
		data["frames_checkpointed"] = done
		data["status"] = "complete"
		if err != nil {
			data["status"] = "busy"
			return data, model.Fail("operation.busy", "The requested checkpoint did not complete; no stronger mode was attempted.")
		}
		if done < total {
			data["status"] = "partial"
			return data, model.Fail("operation.partial", "The requested checkpoint left frames pending.")
		}
		return data, nil
	case "state.inventory.refresh", "state.migrate":
		data := map[string]any{"plan": plan, "operation": nil, "registry_id": reg.ID, "old_generation": reg.Generation, "new_generation": reg.Generation, "publication_pending": false, "inventory_pending": false, "recovery_required": false}
		if r.Command == "state.migrate" {
			if reg.Schema != 1 {
				return data, model.Fail("config.unsupported_schema", "No supported migration path exists for this database version.")
			}
			return data, nil
		}
		plan.Add("file_publication", "Refresh diagnostic-only recovery inventory from authoritative SQLite.")
		data["plan"] = plan
		if r.DryRun {
			return data, nil
		}
		if err = s.approve(r, plan); err != nil {
			return data, err
		}
		ids := []string{}
		if r.VMID != "" {
			if _, err = db.VM(ctx, r.VMID); err != nil {
				return data, err
			}
			ids = append(ids, r.VMID)
		} else {
			rows, e := db.DB.QueryContext(ctx, "SELECT id FROM registrations ORDER BY id")
			if e != nil {
				return data, e
			}
			for rows.Next() {
				var id string
				if e = rows.Scan(&id); e != nil {
					rows.Close()
					return data, e
				}
				ids = append(ids, id)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return data, err
			}
		}
		for _, id := range ids {
			if err = db.RefreshInventory(ctx, id); err != nil {
				data["inventory_pending"] = true
				return data, err
			}
		}
		return data, nil
	}
	return nil, model.Fail("input.invalid", "Unknown state maintenance action.")
}
func (s Service) restore(ctx context.Context, r Request) model.Result {
	out := model.NewResult(r.Command)
	backup, err := config.ResolvePath(r.Path, s.CWD)
	if err != nil {
		out.SetError(err)
		return out
	}
	plan := model.NewPlan(r.Command, model.Global())
	plan.Add("metadata", "Restore the equal-generation authoritative backup while retaining a fenced, recoverable copy of the previous database files.")
	m, err := store.ValidateRestore(ctx, s.Paths.State, backup)
	if _, e := os.Stat(filepath.Join(s.Paths.State, "restore.intent.json")); e == nil && r.Apply {
		err = nil
	}
	data := map[string]any{"plan": plan, "operation": nil, "registry_id": m.RegistryID, "old_generation": m.RecoveryGeneration, "new_generation": nil, "publication_pending": false, "inventory_pending": true, "recovery_required": false}
	out.Data = data
	if err != nil {
		out.SetError(err)
		return out
	}
	if r.DryRun || !r.Apply {
		out.Outcome = "planned"
		return out
	}
	if err = s.approve(r, plan); err != nil {
		out.SetError(err)
		return out
	}
	reg, err := store.Restore(ctx, s.Paths.State, backup)
	if err != nil {
		data["recovery_required"] = true
		out.SetError(err)
		return out
	}
	data["registry_id"] = reg.ID
	data["new_generation"] = reg.Generation
	db, err := store.Open(ctx, s.Paths.State, true)
	if err == nil {
		defer db.Close()
		_, err = s.maintenance(ctx, db, Request{Command: "state.inventory.refresh", Yes: true})
	}
	if err != nil {
		out.Warnings = append(out.Warnings, model.AsError(err).Issue)
	} else {
		data["inventory_pending"] = false
	}
	return out
}

func operationGuard(ctx context.Context, db *store.Store, id string) (*files.Guard, error) {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	g, err := files.Lock(c, filepath.Join(db.Dir, "operations", id+".lock"), true, true)
	if err != nil {
		return nil, model.Fail("operation.busy", "Another controller is still running this operation.")
	}
	return g, nil
}
