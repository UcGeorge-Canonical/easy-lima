package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

// publishLog is a bounded, diagnostic export. Never copy command output,
// operation inputs, credentials, or arbitrary evidence into operational logs.
// SQLite retains the complete event history; this view contains the latest
// 4096 events and final stage statuses for one operation.
func (s *Store) publishLog(ctx context.Context, id string) error {
	if !model.ValidID(id) {
		return nil
	}
	var status string
	if err := s.DB.QueryRowContext(ctx, "SELECT status FROM operations WHERE id=?", id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	rows, err := s.DB.QueryContext(ctx, `SELECT sequence,id,kind,created_at FROM (SELECT sequence,id,kind,created_at FROM namespace_events WHERE operation_id=? ORDER BY sequence DESC LIMIT 4096) ORDER BY sequence`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var seq int64
		var event, kind, at string
		if err = rows.Scan(&seq, &event, &kind, &at); err != nil {
			rows.Close()
			return err
		}
		if err = enc.Encode(map[string]any{"schema_version": 1, "operation_id": id, "sequence": seq, "event_id": event, "kind": kind, "at": at}); err != nil {
			rows.Close()
			return err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	steps, err := s.Steps(ctx, id)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if err = enc.Encode(map[string]any{"schema_version": 1, "operation_id": id, "stage": step.Stage, "attempt": step.Attempt, "dispatch": step.DispatchState, "effect": step.EffectState, "code": step.ResultCode}); err != nil {
			return err
		}
	}
	dir := filepath.Join(s.Dir, "logs")
	if err = files.PrivateDir(dir); err != nil {
		return err
	}
	target := filepath.Join(dir, id+".jsonl")
	old, err := files.Digest(target)
	if err != nil {
		return err
	}
	data := b.Bytes()
	if len(data) > 10<<20 {
		data = data[len(data)-(10<<20):]
		if boundary := bytes.IndexByte(data, '\n'); boundary >= 0 {
			data = data[boundary+1:]
		}
	}
	if err = files.Replace(target, data, old, 0600); err != nil {
		return err
	}
	if model.Terminal(status) {
		return s.pruneCompletedLogs(ctx, 100<<20)
	}
	return nil
}

// Only completed exports are rotated. Active evidence, captured output and the
// authoritative SQLite history are never removed by this diagnostic budget.
func (s *Store) pruneCompletedLogs(ctx context.Context, budget int64) error {
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM operations WHERE status IN ('completed','failed_known','cancelled_before_dispatch') ORDER BY updated_at DESC,id DESC")
	if err != nil {
		return err
	}
	defer rows.Close()
	var retained int64
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		if !model.ValidID(id) {
			continue
		}
		path := filepath.Join(s.Dir, "logs", id+".jsonl")
		info, e := os.Lstat(path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			continue
		}
		retained += info.Size()
		if retained > budget {
			if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	return rows.Err()
}
