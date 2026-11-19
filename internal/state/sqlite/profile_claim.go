package sqlite

import (
	"context"
	"database/sql"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

// HoldProfile fences credential changes while an existing operation establishes
// its connection. Its operation guard must already be held by the caller.
func (s *Store) HoldProfile(ctx context.Context, op OperationRecord, profileID string) error {
	if !model.ValidID(profileID) {
		return model.Fail("record.invalid", "Invalid profile claim.")
	}
	var held int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM resource_claims WHERE operation_id=? AND kind='profile' AND resource_key=? AND token=?", op.Operation.ID, profileID, op.Token).Scan(&held); err != nil {
		return err
	}
	if held == 1 {
		return nil
	}
	return s.Change(ctx, Change{Kind: "profile", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
		var valid, conflicts int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM operations WHERE id=? AND token=? AND status NOT IN ('completed','failed_known','cancelled_before_dispatch')", op.Operation.ID, op.Token).Scan(&valid); err != nil {
			return err
		}
		if valid != 1 {
			return model.Fail("operation.busy", "The operation no longer owns its claim token.")
		}
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM resource_claims WHERE kind='profile' AND resource_key=? AND operation_id<>? AND mode='exclusive'", profileID, op.Operation.ID).Scan(&conflicts); err != nil {
			return err
		}
		if conflicts != 0 {
			return model.Fail("operation.busy", "The profile is being changed by another operation.")
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO resource_claims VALUES(?,'profile',?,'shared',?) ON CONFLICT DO NOTHING", op.Operation.ID, profileID, op.Token)
		return err
	})
}
