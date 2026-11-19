package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	db "github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/vfs/readervfs"
)

const snapshotLimit = 256 << 20

type sizedReader struct{ *bytes.Reader }

func (r sizedReader) Size() (int64, error) { return r.Reader.Size(), nil }

// snapshot assembles a private immutable view of a quiescent database plus its
// committed WAL prefix. Every application writer/checkpointer takes publication
// exclusive, while this copy holds it shared. SQLite never opens the live files
// on the diagnostic path: even mode=ro can create/truncate a shared-memory file.
// The WAL checksum/commit algorithm follows sqlite.org/fileformat2.html#walformat.
func snapshot(ctx context.Context, dir string) (*sql.DB, func(), error) {
	g, err := files.Lock(ctx, filepath.Join(dir, "publication.lock"), false, false)
	if err != nil {
		return nil, nil, err
	}
	defer g.Close()
	if _, err = os.Stat(filepath.Join(dir, "restore.intent.json")); err == nil {
		return nil, nil, model.Fail("state.unavailable", "A restore is interrupted; use the explicit restore workflow.")
	}
	path := filepath.Join(dir, "state.sqlite3")
	main, err := files.Read(path, snapshotLimit)
	if err != nil {
		return nil, nil, err
	}
	if j, e := os.Stat(path + "-journal"); e == nil && j.Size() > 0 {
		return nil, nil, model.Fail("state.unavailable", "A rollback journal requires explicit state recovery.")
	}
	wal, err := files.Read(path+"-wal", snapshotLimit)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	b, err := mergeWAL(main, wal)
	if err != nil {
		return nil, nil, err
	}
	return memorySnapshot(ctx, b)
}
func memorySnapshot(ctx context.Context, b []byte) (*sql.DB, func(), error) {
	name := "easy-lima-" + model.NewID()
	readervfs.Create(name, sizedReader{bytes.NewReader(b)})
	d, err := driver.Open("file:"+name+"?vfs=reader&mode=ro", func(c *db.Conn) error {
		_, err := c.Config(db.DBCONFIG_NO_CKPT_ON_CLOSE, true)
		if err != nil {
			return err
		}
		return c.Exec("PRAGMA query_only=ON; PRAGMA foreign_keys=ON; PRAGMA temp_store=MEMORY; PRAGMA trusted_schema=OFF;")
	})
	cleanup := func() {
		if d != nil {
			d.Close()
		}
		readervfs.Delete(name)
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	d.SetMaxOpenConns(1)
	if err = d.PingContext(ctx); err != nil {
		cleanup()
		return nil, nil, err
	}
	return d, cleanup, nil
}
func mergeWAL(main, wal []byte) ([]byte, error) {
	bad := func() ([]byte, error) {
		return nil, model.Fail("state.unavailable", "SQLite file or WAL evidence is invalid; preserve the file group for explicit recovery.")
	}
	if len(main) < 100 || string(main[:16]) != "SQLite format 3\x00" {
		return bad()
	}
	page := int(binary.BigEndian.Uint16(main[16:18]))
	if page == 1 {
		page = 65536
	}
	if page < 512 || page > 65536 || page&(page-1) != 0 {
		return bad()
	}
	if len(wal) == 0 {
		return main, nil
	}
	if len(wal) < 32 {
		return bad()
	}
	magic := binary.BigEndian.Uint32(wal)
	if magic != 0x377f0682 && magic != 0x377f0683 {
		return bad()
	}
	if binary.BigEndian.Uint32(wal[4:]) != 3007000 || int(binary.BigEndian.Uint32(wal[8:])) != page {
		return bad()
	}
	var order binary.ByteOrder = binary.LittleEndian
	if magic&1 == 1 {
		order = binary.BigEndian
	}
	var s0, s1 uint32
	sum := func(b []byte) {
		for i := 0; i < len(b); i += 8 {
			s0 += order.Uint32(b[i:]) + s1
			s1 += order.Uint32(b[i+4:]) + s0
		}
	}
	sum(wal[:24])
	if s0 != binary.BigEndian.Uint32(wal[24:]) || s1 != binary.BigEndian.Uint32(wal[28:]) {
		return bad()
	}
	lastEnd, dbPages := 32, 0
	frameSize := 24 + page
	for off := 32; off+frameSize <= len(wal); off += frameSize {
		h := wal[off : off+24]
		if !bytes.Equal(h[8:16], wal[16:24]) {
			break
		}
		sum(h[:8])
		sum(wal[off+24 : off+frameSize])
		if s0 != binary.BigEndian.Uint32(h[16:]) || s1 != binary.BigEndian.Uint32(h[20:]) {
			break
		}
		pg := int(binary.BigEndian.Uint32(h))
		n := int(binary.BigEndian.Uint32(h[4:]))
		if pg < 1 || pg > snapshotLimit/page || n > snapshotLimit/page {
			return bad()
		}
		if n > 0 {
			lastEnd = off + frameSize
			dbPages = n
		}
	}
	if dbPages == 0 {
		return main, nil
	}
	out := make([]byte, dbPages*page)
	copy(out, main)
	for off := 32; off < lastEnd; off += frameSize {
		pg := int(binary.BigEndian.Uint32(wal[off:]))
		if pg <= dbPages {
			copy(out[(pg-1)*page:pg*page], wal[off+24:off+frameSize])
		}
	}
	// This is a detached snapshot; the live DB/WAL remain byte-for-byte unchanged.
	out[18] = 1
	out[19] = 1
	return out, nil
}
