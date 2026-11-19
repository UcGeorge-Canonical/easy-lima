package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Inventory struct {
	SchemaVersion  int      `json:"schema_version"`
	DiagnosticOnly bool     `json:"diagnostic_only"`
	RegistryID     string   `json:"registry_id"`
	Generation     int64    `json:"generation"`
	SnapshotAt     string   `json:"snapshot_at"`
	VM             model.VM `json:"vm"`
}

func (s *Store) RefreshInventory(ctx context.Context, id string) error {
	if s.ReadOnly {
		return model.Fail("state.unavailable", "Inventory publication requires an explicit writing action.")
	}
	g, err := files.Lock(ctx, filepath.Join(s.Dir, "publication.lock"), true, false)
	if err != nil {
		return err
	}
	defer g.Close()
	r, err := s.Registry(ctx)
	if err != nil {
		return err
	}
	ids := []string{}
	if id != "" {
		ids = append(ids, id)
	} else {
		rows, err := s.DB.QueryContext(ctx, "SELECT id FROM registrations ORDER BY id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var v string
			if err = rows.Scan(&v); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	for _, id := range ids {
		v, err := s.VM(ctx, id)
		if err != nil {
			return err
		}
		inv := Inventory{1, true, r.ID, r.Generation, model.Now(), v.VM}
		b, err := files.Wrap(inv)
		if err != nil {
			return err
		}
		p := filepath.Join(s.Dir, "recovery-inventory", id+".json")
		old, err := files.Digest(p)
		if err != nil {
			return err
		}
		if old != "" {
			var prior Inventory
			pb, err := files.Read(p, 65536)
			if err != nil {
				return err
			}
			if err = files.Unwrap(pb, &prior); err != nil {
				return err
			}
			if prior.RegistryID != r.ID || prior.Generation > r.Generation {
				return model.Fail("ownership.conflict", "Inventory has foreign or newer evidence; it was preserved.")
			}
		}
		if _, err = s.DB.ExecContext(ctx, "INSERT INTO inventory_exports(registration_id,desired_generation,payload,digest) VALUES(?,?,?,?) ON CONFLICT(registration_id) DO UPDATE SET desired_generation=excluded.desired_generation,payload=excluded.payload,digest=excluded.digest", id, r.Generation, JSON(inv), files.Hash(b)); err != nil {
			return err
		}
		if err = files.Replace(p, b, old, 0600); err != nil {
			return err
		}
		if _, err = s.DB.ExecContext(ctx, "UPDATE inventory_exports SET published_generation=?,published_digest=?,last_error=NULL WHERE registration_id=? AND desired_generation=?", r.Generation, files.Hash(b), id, r.Generation); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) InventoryStatus(ctx context.Context, id string) string {
	r, err := s.Registry(ctx)
	if err != nil {
		return "unknown"
	}
	b, err := files.Read(filepath.Join(s.Dir, "recovery-inventory", id+".json"), 65536)
	if errors.Is(err, os.ErrNotExist) {
		return "missing"
	}
	if err != nil {
		return "invalid"
	}
	var inv Inventory
	if files.Unwrap(b, &inv) != nil || inv.RegistryID != r.ID || inv.VM.ID != id || !inv.DiagnosticOnly {
		return "invalid"
	}
	if inv.Generation != r.Generation {
		return "stale"
	}
	return "current"
}
func ReadInventory(dir string, limit int) model.ListData[model.VM] {
	out := model.NewList[model.VM]()
	out.Complete = false
	entries, err := os.ReadDir(filepath.Join(dir, "recovery-inventory"))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if len(out.Items)+len(out.InvalidRecords) >= limit {
			break
		}
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := files.Read(filepath.Join(dir, "recovery-inventory", e.Name()), 65536)
		var inv Inventory
		if err != nil || files.Unwrap(b, &inv) != nil || !inv.DiagnosticOnly || !model.ValidID(inv.VM.ID) || e.Name() != inv.VM.ID+".json" {
			out.InvalidRecords = append(out.InvalidRecords, model.RecordError{Source: "inventory", Code: "record.invalid", Message: "Diagnostic inventory entry is invalid."})
			continue
		}
		inv.VM.State.Observed = model.Ptr("unknown")
		out.Items = append(out.Items, inv.VM)
	}
	return out
}

// Direct ID lookup does not hide a requested record beyond a listing page.
func ReadInventoryVM(dir, id string) model.ListData[model.VM] {
	out := model.NewList[model.VM]()
	out.Complete = false
	if !model.ValidID(id) {
		return out
	}
	b, err := files.Read(filepath.Join(dir, "recovery-inventory", id+".json"), 65536)
	var inv Inventory
	if err != nil || files.Unwrap(b, &inv) != nil || !inv.DiagnosticOnly || inv.VM.ID != id {
		out.InvalidRecords = append(out.InvalidRecords, model.RecordError{Source: "inventory", Code: "record.invalid", Message: "Requested diagnostic inventory is unavailable or invalid."})
		return out
	}
	inv.VM.State.Observed = model.Ptr("unknown")
	out.Items = append(out.Items, inv.VM)
	return out
}
