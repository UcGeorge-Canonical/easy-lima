package app

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	store "github.com/UcGeorge-Canonical/easy-lima/internal/state/sqlite"
)

func portKey(p config.Port) string {
	return fmt.Sprintf("%s/%s/%d", p.Protocol, p.HostAddress, p.Local)
}
func portClaims(ports []config.Port) []store.Claim {
	out := []store.Claim{}
	for _, p := range ports {
		out = append(out, store.Claim{Kind: "port", Key: portKey(p), Mode: "exclusive"})
	}
	return out
}
func availablePorts(ports []config.Port) error {
	for _, p := range ports {
		address := net.JoinHostPort(p.HostAddress, fmt.Sprint(p.Local))
		var err error
		if p.Protocol == "tcp" {
			var l net.Listener
			l, err = net.Listen("tcp", address)
			if err == nil {
				l.Close()
			}
		} else {
			var l net.PacketConn
			l, err = net.ListenPacket("udp", address)
			if err == nil {
				l.Close()
			}
		}
		if err != nil {
			return model.Fail("port.unavailable", fmt.Sprintf("Requested %s endpoint %s is occupied or unavailable. Choose another local port explicitly.", p.Protocol, address))
		}
	}
	return nil
}
func (s Service) portChecks(ctx context.Context, ports []config.Port, scope model.Scope) []model.Check {
	if len(ports) == 0 {
		return []model.Check{model.NewCheck("ports.readiness", "skipped", "No persistent application ports are configured.", scope)}
	}
	out := []model.Check{}
	for _, p := range ports {
		address := net.JoinHostPort(p.HostAddress, fmt.Sprint(p.Local))
		result, err := s.Runner.Run(ctx, process.Request{Executable: "lsof", Args: []string{"-nP", "-i" + strings.ToUpper(p.Protocol) + "@" + p.HostAddress + ":" + fmt.Sprint(p.Local), "-Fpcn"}, Env: process.Environment(nil), Timeout: 3 * time.Second, Limit: 65536})
		status, message := "unavailable", "Cannot observe "+p.Protocol+" "+address+" (host lsof unavailable)."
		if err == nil && !result.Truncated {
			if result.ExitCode == 0 && len(result.Stdout) > 0 {
				status = "passed"
				message = "A host binding is present for " + p.Protocol + " " + address + "; application health is not inferred."
			} else if result.ExitCode == 1 {
				status = "failed"
				message = "No host binding observed for " + p.Protocol + " " + address
			}
		}
		out = append(out, model.NewCheck("ports.readiness", status, message, scope))
	}
	return out
}
