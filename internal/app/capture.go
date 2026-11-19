package app

import (
	"github.com/UcGeorge-Canonical/easy-lima/internal/events"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"io"
	"path/filepath"
)

func capture(db *store.Store, op store.OperationRecord, r Request) (io.WriteCloser, error) {
	if !r.CaptureOutput {
		return nil, nil
	}
	return events.Open(filepath.Join(db.Dir, "logs"), op.Operation.ID)
}

// Display is separate from persistence. Raw output is saved only with explicit capture.
func (s Service) stageOutput(raw io.Writer) io.Writer {
	if raw == nil {
		return s.Err
	}
	if s.Err == nil {
		return raw
	}
	return io.MultiWriter(s.Err, raw)
}
