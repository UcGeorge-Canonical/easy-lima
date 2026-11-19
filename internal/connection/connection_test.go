package connection

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"golang.org/x/crypto/ssh"
)

// Exercise the SSH protocol, including a peer that leaves channel creation or
// an exec request unanswered. Both must obey the caller's cancellation bound.
func TestRunCancelsStalledSSHSession(t *testing.T) {
	for _, phase := range []string{"channel", "exec"} {
		t.Run(phase, func(t *testing.T) {
			_, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			signer, err := ssh.NewSignerFromKey(private)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			stalled := make(chan struct{})
			serverDone := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					serverDone <- err
					return
				}
				defer conn.Close()
				cfg := &ssh.ServerConfig{NoClientAuth: true}
				cfg.AddHostKey(signer)
				server, channels, requests, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					serverDone <- err
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				channel, ok := <-channels
				if !ok {
					serverDone <- errors.New("missing session request")
					return
				}
				if phase == "exec" {
					_, requests, err := channel.Accept()
					if err != nil {
						serverDone <- err
						return
					}
					request, ok := <-requests
					if !ok || request.Type != "exec" {
						serverDone <- errors.New("missing exec request")
						return
					}
				}
				close(stalled)
				// The client's cancellation must close this connection. No reply
				// from the server is needed to make the client return.
				_ = server.Wait()
				serverDone <- nil
			}()
			client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{User: "fixture", HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()), Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			c := &Client{SSH: client}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := c.Run(ctx, []string{"true"}, nil, nil, nil); result <- err }()
			select {
			case <-stalled:
			case <-time.After(3 * time.Second):
				t.Fatal("SSH fixture never reached the blocked phase")
			}
			cancel()
			select {
			case err := <-result:
				var failure *model.Error
				want := "check.incomplete"
				if phase == "exec" {
					want = "operation.unknown"
				}
				if !errors.As(err, &failure) || failure.Code != want {
					t.Fatalf("got %v; want %s", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation left the SSH request blocked")
			}
			select {
			case err := <-serverDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled transport was not closed")
			}
		})
	}
}
