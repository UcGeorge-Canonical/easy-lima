package agent

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSigningChallengeIsGitSSHSIG(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("OpenSSH unavailable")
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("a diagnostic message, never a user commit")
	sig, err := signer.Sign(rand.Reader, signingChallenge(message))
	if err != nil {
		t.Fatal(err)
	}
	blob := append([]byte("SSHSIG"), ssh.Marshal(struct {
		Version                   uint32
		Public                    []byte
		Namespace, Reserved, Hash string
		Signature                 []byte
	}{1, signer.PublicKey().Marshal(), "git", "", "sha512", ssh.Marshal(sig)})...)
	path := filepath.Join(t.TempDir(), "signature")
	if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "SSH SIGNATURE", Bytes: blob}), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(keygen, "-Y", "check-novalidate", "-n", "git", "-s", path)
	command.Stdin = bytes.NewReader(message)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("OpenSSH rejected signature: %v %s", err, out)
	}
	command = exec.Command(keygen, "-Y", "check-novalidate", "-n", "file", "-s", path)
	command.Stdin = bytes.NewReader(message)
	if err = command.Run(); err == nil {
		t.Fatal("signature accepted in the wrong namespace")
	}
}
