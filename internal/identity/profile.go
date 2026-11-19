// Package identity handles explicit, reusable project identities.
package identity

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"os"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/credential"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/crypto/ssh"
)

type Key struct {
	Private                 any
	Public                  ssh.PublicKey
	PublicText, Fingerprint string
}

func ParsePrivate(path string, passphrase []byte) (Key, error) {
	b, err := files.Read(path, 1<<20)
	if err != nil {
		return Key{}, model.Fail("profile.unlock_required", "The selected profile key file is unavailable.")
	}
	defer credential.Clear(b)
	i, err := os.Stat(path)
	if err != nil {
		return Key{}, err
	}
	if i.Mode().Perm()&0077 != 0 {
		return Key{}, model.Fail("settings.edited", "Private key permissions must exclude group and other access.")
	}
	var raw any
	if passphrase == nil {
		raw, err = ssh.ParseRawPrivateKey(b)
	} else {
		raw, err = ssh.ParseRawPrivateKeyWithPassphrase(b, passphrase)
	}
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		return Key{}, model.Fail("profile.unlock_required", "The selected key requires its passphrase.")
	}
	if err != nil {
		return Key{}, model.Fail("credential.denied", "The selected key could not be decrypted or parsed.")
	}
	switch k := raw.(type) {
	case *rsa.PrivateKey:
		if k.N.BitLen() < 2048 {
			return Key{}, model.Fail("dependency.unsupported", "RSA keys must have at least 2048 bits.")
		}
	case *ed25519.PrivateKey, ed25519.PrivateKey, *ecdsa.PrivateKey:
	default:
		return Key{}, model.Fail("dependency.unsupported", "This key format is not supported; use a software Ed25519, RSA or ECDSA key.")
	}
	signer, err := ssh.NewSignerFromKey(raw)
	if err != nil {
		return Key{}, err
	}
	p := signer.PublicKey()
	return Key{raw, p, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(p))), ssh.FingerprintSHA256(p)}, nil
}
func Generate(path string, passphrase []byte) (Key, error) {
	key, encoded, err := GenerateMaterial(passphrase)
	if err != nil {
		return key, err
	}
	defer credential.Clear(encoded)
	if err = files.WriteNew(path, encoded); err != nil {
		return Key{}, err
	}
	if err = files.WriteNew(path+".pub", []byte(key.PublicText+"\n")); err != nil {
		return Key{}, err
	}
	return key, nil
}

// GenerateMaterial does no I/O. Its public fingerprint can be frozen in an
// operation intent before the encrypted private bytes are published once.
func GenerateMaterial(passphrase []byte) (Key, []byte, error) {
	if len(passphrase) == 0 {
		return Key{}, nil, model.Fail("secret.input_invalid", "A new profile requires a nonempty encryption passphrase.")
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Key{}, nil, err
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "easy-lima", passphrase)
	if err != nil {
		return Key{}, nil, err
	}
	b := pem.EncodeToMemory(block)
	pub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		credential.Clear(b)
		return Key{}, nil, err
	}
	text := ssh.MarshalAuthorizedKey(pub)
	return Key{Private: key, Public: pub, PublicText: strings.TrimSpace(string(text)), Fingerprint: ssh.FingerprintSHA256(pub)}, b, nil
}
func ReadPublic(path string) (ssh.PublicKey, error) {
	b, err := files.Read(path, 64<<10)
	if err != nil {
		return nil, err
	}
	p, _, _, rest, err := ssh.ParseAuthorizedKey(b)
	if err != nil || len(strings.TrimSpace(string(rest))) > 0 {
		return nil, model.Fail("config.invalid", "Expected exactly one supported SSH public key.")
	}
	if strings.Contains(p.Type(), "-cert-") || strings.HasPrefix(p.Type(), "sk-") {
		return nil, model.Fail("dependency.unsupported", "SSH certificates and hardware-backed keys are deferred.")
	}
	return p, nil
}
