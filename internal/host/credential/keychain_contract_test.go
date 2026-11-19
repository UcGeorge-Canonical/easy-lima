//go:build darwin && cgo && contracts

package credential

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"testing"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func TestNativeKeychainRoundTrip(t *testing.T) {
	if os.Getenv("EASY_LIMA_TEST_KEYCHAIN") != "1" {
		t.Skip("explicit opt-in for a temporary owned Keychain item")
	}
	ctx := context.Background()
	id := model.NewID()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	defer Clear(secret)
	store := Native()
	if err := store.Put(ctx, id, secret, false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Delete(ctx, id, false); err != nil {
			t.Error(err)
		}
	})
	got, err := store.Get(ctx, id, false)
	if err != nil {
		t.Fatal(err)
	}
	defer Clear(got)
	if !bytes.Equal(got, secret) {
		t.Fatal("credential bytes changed")
	}
	if err = store.Delete(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Get(ctx, id, false); err == nil || model.AsError(err).Code != "profile.unlock_required" {
		t.Fatal("deleted item still available or missing result misclassified")
	}
}
