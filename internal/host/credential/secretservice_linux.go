//go:build linux

package credential

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"os"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/godbus/dbus/v5"
)

const service = "org.freedesktop.secrets"
const servicePath = dbus.ObjectPath("/org/freedesktop/secrets")

type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}
type secretSession struct {
	conn *dbus.Conn
	path dbus.ObjectPath
	key  []byte
}
type linuxStore struct{}

func Native() Store { return linuxStore{} }
func openSession(ctx context.Context) (*secretSession, error) {
	address := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if !strings.HasPrefix(address, "unix:") {
		return nil, model.Fail("profile.unlock_required", "An existing Unix Secret Service session is unavailable; use manual unlocking.")
	}
	c, err := dbus.Connect(address)
	if err != nil {
		return nil, model.Fail("profile.unlock_required", "Secret Service is unavailable.")
	}
	fail := func(err error) (*secretSession, error) { c.Close(); return nil, err }
	// IETF MODP group 2, as specified by Secret Service's encrypted session.
	p, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFF", 16)
	x, err := rand.Int(rand.Reader, new(big.Int).Sub(p, big.NewInt(3)))
	if err != nil {
		return fail(err)
	}
	x.Add(x, big.NewInt(2))
	pub := new(big.Int).Exp(big.NewInt(2), x, p).Bytes()
	var output dbus.Variant
	var path dbus.ObjectPath
	err = c.Object(service, servicePath).CallWithContext(ctx, service+".Service.OpenSession", 0, "dh-ietf1024-sha256-aes128-cbc-pkcs7", dbus.MakeVariant(pub)).Store(&output, &path)
	if err != nil {
		return fail(model.Fail("profile.unlock_required", "Secret Service could not establish an encrypted session."))
	}
	server, ok := output.Value().([]byte)
	if !ok {
		return fail(model.Fail("credential.denied", "Secret Service returned an invalid exchange."))
	}
	y := new(big.Int).SetBytes(server)
	if y.Cmp(big.NewInt(2)) < 0 || y.Cmp(new(big.Int).Sub(p, big.NewInt(2))) > 0 {
		return fail(model.Fail("credential.denied", "Secret Service returned an invalid public key."))
	}
	shared := new(big.Int).Exp(y, x, p).FillBytes(make([]byte, 128))
	key, err := hkdf.Key(sha256.New, shared, nil, "", 16)
	Clear(shared)
	x.SetInt64(0)
	if err != nil {
		return fail(err)
	}
	return &secretSession{c, path, key}, nil
}
func (s *secretSession) close() {
	_ = s.conn.Object(service, s.path).Call(service+".Session.Close", 0).Err
	s.conn.Close()
	Clear(s.key)
}
func (s *secretSession) prompt(ctx context.Context, path dbus.ObjectPath, interactive bool) (dbus.Variant, error) {
	if path == "/" {
		return dbus.Variant{}, nil
	}
	if !interactive {
		return dbus.Variant{}, model.Fail("credential.interaction_required", "Secret Service requires an interactive unlock.")
	}
	signals := make(chan *dbus.Signal, 2)
	s.conn.Signal(signals)
	defer s.conn.RemoveSignal(signals)
	if err := s.conn.AddMatchSignal(dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(service+".Prompt"), dbus.WithMatchMember("Completed")); err != nil {
		return dbus.Variant{}, err
	}
	defer s.conn.RemoveMatchSignal(dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(service+".Prompt"), dbus.WithMatchMember("Completed"))
	if err := s.conn.Object(service, path).CallWithContext(ctx, service+".Prompt.Prompt", 0, "").Err; err != nil {
		return dbus.Variant{}, model.Fail("credential.denied", "Secret Service prompt was denied.")
	}
	for {
		select {
		case <-ctx.Done():
			_ = s.conn.Object(service, path).Call(service+".Prompt.Dismiss", 0).Err
			return dbus.Variant{}, model.Fail("credential.denied", "Credential request was cancelled.")
		case sig := <-signals:
			if sig == nil {
				continue
			}
			if sig.Path == path && len(sig.Body) == 2 {
				dismissed, ok := sig.Body[0].(bool)
				v, vok := sig.Body[1].(dbus.Variant)
				if !ok || !vok || dismissed {
					return dbus.Variant{}, model.Fail("credential.denied", "Credential request was dismissed.")
				}
				return v, nil
			}
		}
	}
}
func (s *secretSession) find(ctx context.Context, id string, interactive bool) (dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath
	err := s.conn.Object(service, servicePath).CallWithContext(ctx, service+".Service.SearchItems", 0, map[string]string{"application": "easy-lima", "profile_id": id}).Store(&unlocked, &locked)
	if err != nil {
		return "", model.Fail("credential.denied", "Saved credentials could not be searched; their absence is not confirmed.")
	}
	if len(unlocked)+len(locked) > 1 {
		return "", model.Fail("ownership.conflict", "Multiple credential items match this profile.")
	}
	if len(unlocked) == 1 {
		return unlocked[0], nil
	}
	if len(locked) == 0 {
		return "", model.Fail("profile.unlock_required", "No remembered passphrase exists for this profile.")
	}
	if !interactive {
		return "", model.Fail("credential.interaction_required", "The remembered credential is locked.")
	}
	var paths []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err = s.conn.Object(service, servicePath).CallWithContext(ctx, service+".Service.Unlock", 0, locked).Store(&paths, &prompt); err != nil {
		return "", model.Fail("credential.denied", "Secret Service unlock failed.")
	}
	if _, err = s.prompt(ctx, prompt, interactive); err != nil {
		return "", err
	}
	return locked[0], nil
}
func (linuxStore) Get(ctx context.Context, id string, interactive bool) ([]byte, error) {
	s, err := openSession(ctx)
	if err != nil {
		return nil, err
	}
	defer s.close()
	p, err := s.find(ctx, id, interactive)
	if err != nil {
		return nil, err
	}
	var v secret
	if err = s.conn.Object(service, p).CallWithContext(ctx, service+".Item.GetSecret", 0, s.path).Store(&v); err != nil {
		return nil, model.Fail("credential.denied", "Saved passphrase could not be retrieved.")
	}
	if v.Session != s.path || len(v.Parameters) != 16 || len(v.Value) == 0 || len(v.Value) > 65552 || len(v.Value)%16 != 0 {
		return nil, model.Fail("credential.denied", "Secret Service returned invalid encrypted data.")
	}
	block, _ := aes.NewCipher(s.key)
	cipher.NewCBCDecrypter(block, v.Parameters).CryptBlocks(v.Value, v.Value)
	pad := int(v.Value[len(v.Value)-1])
	if pad < 1 || pad > 16 || pad > len(v.Value) {
		Clear(v.Value)
		return nil, model.Fail("credential.denied", "Saved credential padding is invalid.")
	}
	for _, b := range v.Value[len(v.Value)-pad:] {
		if int(b) != pad {
			Clear(v.Value)
			return nil, model.Fail("credential.denied", "Saved credential padding is invalid.")
		}
	}
	return v.Value[:len(v.Value)-pad], nil
}
func (linuxStore) Put(ctx context.Context, id string, b []byte, interactive bool) error {
	s, err := openSession(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	var coll dbus.ObjectPath
	if err = s.conn.Object(service, servicePath).CallWithContext(ctx, service+".Service.ReadAlias", 0, "default").Store(&coll); err != nil || coll == "/" {
		return model.Fail("credential.interaction_required", "No default Secret Service collection is available.")
	}
	pad := 16 - len(b)%16
	data := make([]byte, len(b)+pad)
	copy(data, b)
	for i := len(b); i < len(data); i++ {
		data[i] = byte(pad)
	}
	defer Clear(data)
	iv := make([]byte, 16)
	if _, err = rand.Read(iv); err != nil {
		return err
	}
	block, _ := aes.NewCipher(s.key)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(data, data)
	props := map[string]dbus.Variant{service + ".Item.Label": dbus.MakeVariant("easy-lima profile " + id), service + ".Item.Attributes": dbus.MakeVariant(map[string]string{"application": "easy-lima", "profile_id": id})}
	var item, prompt dbus.ObjectPath
	if err = s.conn.Object(service, coll).CallWithContext(ctx, service+".Collection.CreateItem", 0, props, secret{s.path, iv, data, "text/plain; charset=utf-8"}, true).Store(&item, &prompt); err != nil {
		return model.Fail("credential.denied", "The passphrase could not be saved in Secret Service.")
	}
	_, err = s.prompt(ctx, prompt, interactive)
	return err
}
func (linuxStore) Delete(ctx context.Context, id string, interactive bool) error {
	s, err := openSession(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	p, err := s.find(ctx, id, interactive)
	if err != nil {
		if model.AsError(err).Code == "profile.unlock_required" {
			return nil
		}
		return err
	}
	var prompt dbus.ObjectPath
	if err = s.conn.Object(service, p).CallWithContext(ctx, service+".Item.Delete", 0).Store(&prompt); err != nil {
		return model.Fail("credential.denied", "The owned saved credential could not be removed.")
	}
	_, err = s.prompt(ctx, prompt, interactive)
	return err
}
