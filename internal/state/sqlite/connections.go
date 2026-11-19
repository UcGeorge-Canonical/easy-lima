// Package sqlite owns all SQLite driver, transaction and physical-file behavior.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	db "github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
)

func openDB(ctx context.Context, path, mode string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"mode": {mode}, "_txlock": {"immediate"}}
	u.RawQuery = q.Encode()
	d, err := driver.Open(u.String(), func(c *db.Conn) error {
		ok, err := c.Config(db.DBCONFIG_NO_CKPT_ON_CLOSE, true)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("SQLite did not disable close checkpointing")
		}
		if err = c.WALAutoCheckpoint(0); err != nil {
			return err
		}
		if err = c.BusyTimeout(2 * time.Second); err != nil {
			return err
		}
		if _, err = c.Config(db.DBCONFIG_ENABLE_FKEY, true); err != nil {
			return err
		}
		return c.Exec("PRAGMA synchronous=FULL; PRAGMA temp_store=MEMORY; PRAGMA trusted_schema=OFF;")
	})
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	d.SetMaxIdleConns(1)
	if err = d.PingContext(ctx); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func raw(ctx context.Context, d *sql.DB, fn func(*db.Conn) error) error {
	c, err := d.Conn(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Raw(func(v any) error { return fn(v.(driver.Conn).Raw()) })
}
