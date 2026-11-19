//go:build acceptance

package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/app"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

func networkPorts(t *testing.T, root string, reuse bool) []config.Port {
	t.Helper()
	path := filepath.Join(root, "acceptance-ports.json")
	if reuse {
		b, err := os.ReadFile(path)
		var ports []config.Port
		if err != nil || json.Unmarshal(b, &ports) != nil || len(ports) != 8 {
			t.Fatal("missing network fixture")
		}
		return ports
	}
	var ports []config.Port
	for _, protocol := range []string{"tcp", "udp"} {
		for _, guest := range []string{"127.0.0.1", "0.0.0.0", "::1", "::"} {
			host := "127.0.0.1"
			if guest == "::1" || guest == "::" {
				host = "::1"
			}
			ports = append(ports, config.Port{Guest: 18701 + len(ports), Local: availablePort(t, protocol, host), Protocol: protocol, HostAddress: host, GuestAddress: guest})
		}
	}
	b, _ := json.Marshal(ports)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	return ports
}

func availablePort(t *testing.T, protocol, host string) int {
	t.Helper()
	address := net.JoinHostPort(host, "0")
	if protocol == "udp" {
		c, err := net.ListenPacket("udp", address)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		return c.LocalAddr().(*net.UDPAddr).Port
	}
	l, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func verifyNetworks(t *testing.T, s app.Service, vm *model.VM, ports []config.Port) {
	t.Helper()
	ctx := context.Background()
	// A test-only echo service makes address matching and both transports observable.
	script := `import json,socket,threading,time
def serve(p):
    family=socket.AF_INET6 if ':' in p['guest_address'] else socket.AF_INET
    sock=socket.socket(family,socket.SOCK_DGRAM if p['protocol']=='udp' else socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
    if family==socket.AF_INET6: sock.setsockopt(socket.IPPROTO_IPV6,socket.IPV6_V6ONLY,1)
    sock.bind((p['guest_address'],p['guest']))
    if p['protocol']=='udp':
        while True:
            data,peer=sock.recvfrom(256); sock.sendto(data,peer)
    else:
        sock.listen()
        while True:
            conn,_=sock.accept()
            with conn: conn.sendall(conn.recv(256))
for p in json.loads(__import__('sys').argv[1]): threading.Thread(target=serve,args=(p,),daemon=True).start()
while True: time.sleep(60)
`
	b, _ := json.Marshal(ports)
	run := func(args ...string) {
		r := s.Execute(ctx, app.Request{Command: "exec", VMID: vm.ID, GuestArgs: args, PassphraseFD: -1})
		if r.ExitCode != 0 {
			t.Fatalf("network guest command: %+v", r.Errors)
		}
	}
	run("sudo", "systemd-run", "--unit=el-acceptance-network", "--property=Type=exec", "/usr/bin/python3", "-u", "-c", script, string(b))
	defer run("sudo", "systemctl", "stop", "el-acceptance-network")
	for _, p := range ports {
		t.Logf("%s guest %s:%d -> loopback %s:%d", p.Protocol, p.GuestAddress, p.Guest, p.HostAddress, p.Local)
		if err := waitEcho(p.Protocol, net.JoinHostPort(p.HostAddress, fmt.Sprint(p.Local))); err != nil {
			t.Fatal(err)
		}
	}
	forwardCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	local := availablePort(t, "tcp", "127.0.0.1")
	done := make(chan model.Result, 1)
	go func() {
		done <- s.Execute(forwardCtx, app.Request{Command: "forward", VMID: vm.ID, GuestPort: ports[0].Guest, LocalPort: local, Bind: "127.0.0.1", PassphraseFD: -1})
	}()
	if err := waitEcho("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(local))); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case r := <-done:
		if r.ExitCode != 0 && r.ExitCode != 130 {
			t.Fatalf("forward shutdown: %+v", r)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("forward did not stop")
	}
}

func waitEcho(protocol, address string) error {
	deadline := time.Now().Add(40 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout(protocol, address, time.Second)
		if err == nil {
			c.SetDeadline(time.Now().Add(time.Second))
			_, err = c.Write([]byte("easy-lima-network"))
			if err == nil {
				b := make([]byte, 256)
				n, e := c.Read(b)
				err = e
				if err == nil && string(b[:n]) != "easy-lima-network" {
					err = fmt.Errorf("unexpected echo")
				}
			}
			c.Close()
		}
		if err == nil {
			return nil
		}
		last = err
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("%s %s: %w", protocol, address, last)
}
