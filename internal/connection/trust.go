package connection

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
)

type Trust struct {
	Generation                                int64
	PublicKey, Algorithm, Fingerprint, Source string
}

func PublicText(key ssh.PublicKey) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}
func Fingerprint(key ssh.PublicKey) string { return ssh.FingerprintSHA256(key) }
func ReplaceTrust(ctx context.Context, s *store.Store, op store.OperationRecord, v store.VMRecord, endpoint Endpoint, public string) error {
	key, _, _, rest, err := ssh.ParseAuthorizedKey([]byte(public))
	if err != nil || len(strings.TrimSpace(string(rest))) > 0 {
		return model.Fail("config.invalid", "Expected one independently verified SSH host key.")
	}
	c, err := Dial(ctx, endpoint, key, false)
	if err != nil {
		return err
	}
	c.Close()
	var enrolledBy string
	if v.ActiveTrustGeneration != nil {
		if err = s.DB.QueryRowContext(ctx, "SELECT enrollment_operation_id FROM vm_trust WHERE registration_id=? AND trust_generation=?", v.VM.ID, *v.ActiveTrustGeneration).Scan(&enrolledBy); err != nil {
			return err
		}
		if enrolledBy == op.Operation.ID {
			current, e := ReadTrust(ctx, s, v.VM.ID)
			if e != nil {
				return e
			}
			if current.PublicKey != PublicText(key) {
				return model.Fail("trust.mismatch", "The resumed repair differs from its committed host key.")
			}
			return PublishTrust(ctx, s, op.Operation.ID, v.VM.ID)
		}
	}
	generation := int64(1)
	if v.ActiveTrustGeneration != nil {
		generation = *v.ActiveTrustGeneration + 1
	}
	err = s.Change(ctx, store.Change{Kind: "trust", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{v.VM.ID}}, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, "INSERT INTO vm_trust VALUES(?,?,?,?,?,?,?,?,?,?,?)", v.VM.ID, generation, v.VM.NamespaceID, v.VM.InstanceName, key.Type(), PublicText(key), Fingerprint(key), "explicit_repair", op.Operation.ID, files.Hash([]byte(endpoint.AddressPort())), model.Now()); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, "UPDATE registrations SET active_trust_generation=?,pending_trust_operation_id=NULL WHERE id=?", generation, v.VM.ID)
		return e
	})
	if err != nil {
		return err
	}
	return PublishTrust(ctx, s, op.Operation.ID, v.VM.ID)
}

func ReadTrust(ctx context.Context, s *store.Store, vmID string) (Trust, error) {
	var t Trust
	err := s.DB.QueryRowContext(ctx, "SELECT t.trust_generation,t.public_key,t.algorithm,t.fingerprint,t.source FROM vm_trust t JOIN registrations r ON r.id=t.registration_id AND r.active_trust_generation=t.trust_generation WHERE r.id=?", vmID).Scan(&t.Generation, &t.PublicKey, &t.Algorithm, &t.Fingerprint, &t.Source)
	if err != nil {
		return t, model.Fail("trust.mismatch", "The VM has no active recorded SSH host key.")
	}
	return t, nil
}
func (t Trust) Key() (ssh.PublicKey, error) {
	p, _, _, _, err := ssh.ParseAuthorizedKey([]byte(t.PublicKey))
	return p, err
}
func TrustPath(state, id string) string { return filepath.Join(state, "ssh", "known-hosts", id) }
func TrustToken(id string) string       { return "easy-lima-" + id }
func PublishTrust(ctx context.Context, s *store.Store, opID, id string) error {
	t, err := ReadTrust(ctx, s, id)
	if err != nil {
		return err
	}
	p := TrustPath(s.Dir, id)
	if err = files.PrivateDir(filepath.Dir(p)); err != nil {
		return err
	}
	b := []byte(TrustToken(id) + " " + strings.TrimSpace(t.PublicKey) + "\n")
	expected, err := s.Setting(ctx, id, p, "trust")
	if err != nil {
		return err
	}
	current, err := files.Digest(p)
	if err != nil {
		return err
	}
	if current != expected && current != files.Hash(b) {
		return model.Fail("settings.edited", "The VM known-hosts export was changed.")
	}
	if current != files.Hash(b) {
		if err = files.Replace(p, b, expected, 0600); err != nil {
			return err
		}
	}
	return s.OwnSetting(ctx, opID, id, "vm", p, "trust", files.Hash(b))
}
func Enroll(ctx context.Context, s *store.Store, op store.OperationRecord, v store.VMRecord, e Endpoint) (Trust, error) {
	if v.ActiveTrustGeneration != nil {
		t, err := ReadTrust(ctx, s, v.VM.ID)
		if err != nil {
			return t, err
		}
		var enrolledBy string
		if err = s.DB.QueryRowContext(ctx, "SELECT enrollment_operation_id FROM vm_trust WHERE registration_id=? AND trust_generation=?", v.VM.ID, t.Generation).Scan(&enrolledBy); err != nil {
			return t, err
		}
		if enrolledBy == op.Operation.ID {
			err = PublishTrust(ctx, s, op.Operation.ID, v.VM.ID)
		} else {
			err = VerifyTrustExport(ctx, s, v.VM.ID)
		}
		return t, err
	}
	client, err := Dial(ctx, e, nil, true)
	if err != nil {
		return Trust{}, err
	}
	key := client.HostKey
	client.Close()
	t := Trust{Generation: 1, PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))), Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key), Source: "local_first_use"}
	err = s.Change(ctx, store.Change{Kind: "trust", OperationID: op.Operation.ID, Namespace: true, VMIDs: []string{v.VM.ID}}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO vm_trust VALUES(?,?,?,?,?,?,?,?,?,?,?)", v.VM.ID, t.Generation, v.VM.NamespaceID, v.VM.InstanceName, t.Algorithm, t.PublicKey, t.Fingerprint, t.Source, op.Operation.ID, files.Hash([]byte(e.AddressPort())), model.Now()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE registrations SET active_trust_generation=?,pending_trust_operation_id=NULL WHERE id=?", t.Generation, v.VM.ID)
		return err
	})
	if err != nil {
		return t, err
	}
	if err = PublishTrust(ctx, s, op.Operation.ID, v.VM.ID); err != nil {
		return t, err
	}
	p, _ := t.Key()
	client, err = Dial(ctx, e, p, false)
	if err != nil {
		return t, err
	}
	client.Close()
	return t, nil
}
func VerifyTrustExport(ctx context.Context, s *store.Store, id string) error {
	p := TrustPath(s.Dir, id)
	expected, err := s.Setting(ctx, id, p, "trust")
	if err != nil {
		return err
	}
	actual, err := files.Digest(p)
	if err != nil || actual == "" || actual != expected {
		return model.Fail("settings.edited", "The VM's trust export is missing or changed; use explicit trust-export repair.")
	}
	return nil
}
