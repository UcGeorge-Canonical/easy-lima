package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

type Destination struct {
	Path      string       `json:"path"`
	Phase     string       `json:"phase"`
	PeakBytes int64        `json:"peak_bytes"`
	Purpose   string       `json:"purpose"`
	Volume    files.Volume `json:"volume"`
}
type Group struct {
	ID             string `json:"id"`
	AvailableBytes int64  `json:"available_bytes"`
	PendingBytes   int64  `json:"pending_bytes"`
	RequiredBytes  int64  `json:"required_bytes"`
	QuotaKnown     bool   `json:"quota_known"`
}
type Ledger struct {
	Destinations         []Destination `json:"destinations"`
	Groups               []Group       `json:"groups"`
	FutureGrowthReserved bool          `json:"future_growth_reserved"`
}

func Plan(root, state string, imageBound, initialDisk int64) (Ledger, error) {
	l := Ledger{Destinations: []Destination{}, Groups: []Group{}}
	for _, d := range []Destination{{Path: filepath.Join(root, "_easy-lima", "assets"), Phase: "create", PeakBytes: imageBound, Purpose: "verified image"}, {Path: filepath.Join(root, "_easy-lima", "tmp"), Phase: "create", PeakBytes: imageBound, Purpose: "download and conversion staging"}, {Path: root, Phase: "create", PeakBytes: initialDisk, Purpose: "initial disk and guest setup"}, {Path: state, Phase: "create", PeakBytes: 256 << 20, Purpose: "metadata, WAL, backup and publication"}} {
		v, err := files.InspectVolume(d.Path)
		if err != nil {
			return l, err
		}
		d.Volume = v
		l.Destinations = append(l.Destinations, d)
	}
	index := map[string]int{}
	for _, d := range l.Destinations {
		i, ok := index[d.Volume.CapacityGroup]
		if !ok {
			i = len(l.Groups)
			index[d.Volume.CapacityGroup] = i
			l.Groups = append(l.Groups, Group{ID: d.Volume.CapacityGroup, AvailableBytes: d.Volume.AvailableBytes, QuotaKnown: d.Volume.QuotaKnown})
		}
		g := &l.Groups[i]
		if d.Volume.AvailableBytes < g.AvailableBytes {
			g.AvailableBytes = d.Volume.AvailableBytes
		}
		g.RequiredBytes += d.PeakBytes
	}
	for i := range l.Groups {
		g := &l.Groups[i]
		margin := g.RequiredBytes / 10
		if margin < 1<<30 {
			margin = 1 << 30
		}
		g.RequiredBytes += margin
		if g.RequiredBytes > g.AvailableBytes {
			return l, model.Fail("storage.insufficient", fmt.Sprintf("Creation requires %d bytes in a shared capacity group with %d available; future disk growth is unreserved.", g.RequiredBytes, g.AvailableBytes))
		}
	}
	return l, nil
}
func CheckRoot(n store.Namespace) error {
	if _, err := os.Stat(n.Root); err != nil {
		return model.Fail("storage.unavailable", "The selected VM storage root is unavailable. No fallback directory will be created.")
	}
	id, err := files.RootIdentity(n.Root)
	if err != nil {
		return err
	}
	if id != n.VolumeIdentity {
		return model.Fail("storage.unavailable", "The configured path is on a different volume or is a replacement directory.")
	}
	return nil
}
func Reserve(ctx context.Context, s *store.Store, op store.OperationRecord, n store.Namespace, l Ledger) error {
	return s.Change(ctx, store.Change{Kind: "intent", OperationID: op.Operation.ID}, func(tx *sql.Tx) error {
		b, err := files.Canonical(l)
		if err != nil {
			return err
		}
		for _, g := range l.Groups {
			var held int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM capacity_claims WHERE operation_id=? AND capacity_group_id=? AND state='held'", op.Operation.ID, g.ID).Scan(&held); err != nil {
				return err
			}
			if held > 0 {
				continue
			}
			var pending int64
			if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(remaining_bytes),0) FROM capacity_claims WHERE capacity_group_id=? AND state='held'", g.ID).Scan(&pending); err != nil {
				return err
			}
			if g.AvailableBytes-pending < g.RequiredBytes {
				return model.Fail("storage.insufficient", "Other active creations have claimed the remaining shared filesystem budget.")
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO capacity_claims VALUES(?,?,?,?,?,?,?,?, 'held',0,NULL)", op.Operation.ID, g.ID, "create", n.ID, files.Hash(b), store.JSON(l.Destinations), g.RequiredBytes, g.RequiredBytes)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
