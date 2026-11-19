package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/lima"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

// A failed creation can leave only an authoritative reservation. Explicit
// deletion may retire it after proving that no instance or native controller
// exists. Partial disks/configurations are intentionally outside this path.
func (s Service) deleteReservation(ctx context.Context, db *store.Store, r Request, v store.VMRecord, adapter lima.Adapter) (model.MutationData, error) {
	plan := model.NewPlan("delete", model.VMScope(v.VM.ID, v.VM.SourcePath))
	plan.AffectedVMIDs = []string{v.VM.ID}
	plan.Add("metadata", "Retire this uncreated VM reservation and its reconciled creation attempt; no disk or checkout is deleted.")
	data := model.Mutation(plan)
	data.VM = &v.VM
	if r.ConfirmVM != v.VM.ID && !r.DryRun {
		return data, model.Fail("decision.required", "Deletion requires --confirm-vm with this VM's full immutable ID.")
	}
	for _, id := range v.VM.OperationIDs {
		if !r.DryRun {
			guard, err := operationGuard(ctx, db, id)
			if err != nil {
				return data, err
			}
			defer guard.Close()
		}
		op, err := db.Operation(ctx, id)
		if err != nil {
			return data, err
		}
		if op.Operation.Kind != "up" {
			return data, model.Fail("operation.busy", "A different operation still holds this reservation.")
		}
	}
	observation, err := adapter.Observe(ctx, v.VM.InstanceName)
	if err != nil {
		return data, err
	}
	if observation.Status != "Absent" {
		return data, model.Fail("operation.unknown", "The reserved instance exists; resume its recorded creation before deletion.")
	}
	if _, err = os.Lstat(filepath.Join(adapter.Root, v.VM.InstanceName)); !os.IsNotExist(err) {
		return data, model.Fail("operation.unknown", "Partial instance files remain; inspect them before retiring this reservation.")
	}
	if err = adapter.NoController(ctx, v.VM.InstanceName); err != nil {
		return data, err
	}
	if r.DryRun {
		return data, nil
	}
	if err = s.approve(r, plan); err != nil {
		return data, err
	}
	id := model.NewID()
	err = db.Change(ctx, store.Change{Kind: "retirement", OperationID: id, Namespace: true, VMIDs: []string{v.VM.ID}}, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, "DELETE FROM owned_settings WHERE owner_id=? AND field='port'", v.VM.ID); e != nil {
			return e
		}
		// New claims cannot intervene: the original exclusive VM/source claims
		// remain held until this single, witnessed retirement transaction commits.
		for _, old := range v.VM.OperationIDs {
			if _, e := tx.ExecContext(ctx, "UPDATE operations SET status='failed_known',updated_at=? WHERE id=?", model.Now(), old); e != nil {
				return e
			}
			for _, table := range []string{"resource_claims", "binding_reservations"} {
				if _, e := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE operation_id=?", old); e != nil {
					return e
				}
			}
			if _, e := tx.ExecContext(ctx, "UPDATE capacity_claims SET state='released',remaining_bytes=0,released_at=? WHERE operation_id=?", model.Now(), old); e != nil {
				return e
			}
		}
		result, e := tx.ExecContext(ctx, "UPDATE registrations SET ownership='retired',lifecycle='retired',retired_at=?,row_revision=row_revision+1 WHERE id=? AND ownership='reserved' AND applied_environment_revision_id IS NULL", model.Now(), v.VM.ID)
		if e != nil {
			return e
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return model.Fail("ownership.conflict", "The reservation changed before retirement.")
		}
		if v.VM.ProfileID != nil {
			if _, e = tx.ExecContext(ctx, "UPDATE profiles SET membership_generation=membership_generation+1 WHERE id=?", *v.VM.ProfileID); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO operations(id,kind,scope_kind,scope_id,scope_path,token,status,inputs,plan,created_at,updated_at) VALUES(?,'delete','vm',?,?,?,'completed',?,?,?,?)", id, v.VM.ID, v.VM.SourcePath, model.NewID(), store.JSON(map[string]any{"vm_id": v.VM.ID, "retired_creation_attempts": v.VM.OperationIDs}), store.JSON(plan), model.Now(), model.Now())
		return e
	})
	if err != nil {
		return data, err
	}
	op, err := db.Operation(ctx, id)
	if err != nil {
		return data, err
	}
	return s.operationResult(ctx, db, data, op), nil
}
