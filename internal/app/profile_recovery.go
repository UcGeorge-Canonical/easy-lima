package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/agent"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/credential"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/identity"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

func (s Service) resumeProfileCreation(ctx context.Context, db *store.Store, op store.OperationRecord, in ProfileCreation, r Request) error {
	p := in.Profile
	if !model.ValidID(p.ID) || in.Fingerprint == "" {
		return model.Fail("record.invalid", "The saved profile attempt lacks a frozen identity.")
	}
	if existing, err := db.Profile(ctx, p.ID); err == nil {
		if *existing.Profile.Fingerprint != in.Fingerprint {
			return model.Fail("identity.key_mismatch", "The saved profile fingerprint differs from its committed key.")
		}
		return db.Finish(ctx, op, "completed")
	}
	if _, err := os.Lstat(p.Key.Path); os.IsNotExist(err) && in.KeyOwnership == "owned" {
		if err = db.Finish(ctx, op, "failed_known"); err != nil {
			return err
		}
		return model.Fail("operation.failed", "The generated key was never published. The attempt is safely closed; create the profile again to generate a new key.")
	}
	key, err := identity.ParsePrivate(p.Key.Path, nil)
	var pass []byte
	if err != nil && model.AsError(err).Code == "profile.unlock_required" {
		if p.Unlock.Remember && s.Credential != nil {
			pass, err = s.Credential.Get(ctx, p.ID, r.Interactive)
		}
		if len(pass) == 0 {
			pass, err = s.secret(r, "Passphrase for "+p.Label)
		}
		if err == nil {
			key, err = identity.ParsePrivate(p.Key.Path, pass)
		}
	}
	defer credential.Clear(pass)
	if err != nil {
		return err
	}
	if key.Fingerprint != in.Fingerprint {
		return model.Fail("identity.key_mismatch", "The generated/imported key differs from this operation's frozen public fingerprint.")
	}
	pub, err := identity.ReadPublic(p.Key.PublicPath)
	if os.IsNotExist(err) {
		if err = files.PrivateDir(filepath.Dir(p.Key.PublicPath)); err != nil {
			return err
		}
		if err = files.WriteNew(p.Key.PublicPath, []byte(in.PublicKey+"\n")); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if ssh.FingerprintSHA256(pub) != in.Fingerprint {
		return model.Fail("identity.key_mismatch", "The public key file changed during profile preparation.")
	}
	binding, err := agent.Ensure(ctx, p.ID, in.Agent)
	if err != nil {
		return err
	}
	if binding != in.Agent {
		return model.Fail("ownership.conflict", "The profile agent binding differs from the saved attempt.")
	}
	c, err := agent.Connect(ctx, binding.SocketPath)
	if err != nil {
		return err
	}
	defer c.Close()
	keys, err := c.Agent.List()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		if err = c.Agent.Add(sshagent.AddedKey{PrivateKey: key.Private, Comment: "easy-lima:" + p.ID}); err != nil {
			return model.Fail("credential.denied", "The owned agent refused its key.")
		}
	}
	if err = c.Verify(in.Fingerprint); err != nil {
		return err
	}
	if p.Unlock.Remember {
		if len(pass) == 0 {
			return model.Fail("secret.input_invalid", "An encryption passphrase is required for remembering.")
		}
		if err = s.Credential.Put(ctx, p.ID, pass, r.Interactive); err != nil {
			return err
		}
	}
	b, err := config.Encode(p)
	if err != nil {
		return err
	}
	path := filepath.Join(s.Paths.Config, "profiles", p.ID+".toml")
	if err = files.PrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	hash, err := files.Digest(path)
	if err != nil {
		return err
	}
	if hash != "" && hash != files.Hash(b) {
		return model.Fail("settings.edited", "The desired profile file differs from the saved creation attempt.")
	}
	if hash == "" {
		if err = files.WriteNew(path, b); err != nil {
			return err
		}
	}
	if err = db.SaveProfile(ctx, op, p, in.PublicKey, in.Fingerprint, files.Hash(b), in.KeyOwnership, binding); err != nil {
		return err
	}
	if err = db.RecordStep(ctx, op, "profile.prepare", "exited", "satisfied", "profile.ready", map[string]string{"fingerprint": in.Fingerprint}); err != nil {
		return err
	}
	return db.Finish(ctx, op, "completed")
}
