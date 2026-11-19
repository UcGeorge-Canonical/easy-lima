package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Networks struct {
	LXD    string `json:"lxd"`
	Docker string `json:"docker"`
	Pool   string `json:"pool"`
}

func networkCandidates(ctx context.Context, subnets []string) ([]Networks, error) {
	req := process.Request{Env: process.Environment(nil), Timeout: 5 * time.Second, Limit: 256 << 10}
	if runtime.GOOS == "darwin" {
		req.Executable = "/usr/sbin/netstat"
		req.Args = []string{"-rn", "-f", "inet"}
	} else {
		req.Executable = "ip"
		req.Args = []string{"-j", "-4", "route", "show", "table", "all"}
	}
	result, err := (process.OS{}).Run(ctx, req)
	if err != nil || result.ExitCode != 0 || result.Truncated {
		return nil, model.Fail("check.incomplete", "Host IPv4 routes could not be inspected before network allocation.")
	}
	routes, err := routePrefixes(result.Stdout, runtime.GOOS)
	if err != nil {
		return nil, err
	}
	return chooseNetworkCandidates(subnets, routes)
}

func routePrefixes(raw []byte, platform string) ([]netip.Prefix, error) {
	destinations := []string{}
	if platform == "linux" {
		var routes []struct {
			Dst string `json:"dst"`
		}
		if json.Unmarshal(raw, &routes) != nil {
			return nil, model.Fail("check.incomplete", "Host routes are not valid JSON.")
		}
		for _, r := range routes {
			destinations = append(destinations, r.Dst)
		}
	} else {
		found := false
		for _, line := range strings.Split(string(raw), "\n") {
			f := strings.Fields(line)
			if len(f) == 0 {
				continue
			}
			if f[0] == "Destination" {
				found = true
				continue
			}
			if found && len(f) >= 4 {
				destinations = append(destinations, f[0])
			}
		}
		if !found {
			return nil, model.Fail("check.incomplete", "Host route table format is unsupported.")
		}
	}
	out := []netip.Prefix{}
	for _, d := range destinations {
		if d == "" || d == "default" || d == "0.0.0.0/0" {
			continue
		}
		address, bits, has := strings.Cut(d, "/")
		parts := strings.Split(address, ".")
		if !has {
			bits = strconv.Itoa(len(parts) * 8)
		}
		for len(parts) < 4 {
			parts = append(parts, "0")
		}
		p, err := netip.ParsePrefix(strings.Join(parts, ".") + "/" + bits)
		if err != nil {
			return nil, model.Fail("check.incomplete", "An IPv4 host route cannot be interpreted safely.")
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func chooseNetworkCandidates(subnets []string, routes []netip.Prefix) ([]Networks, error) {
	out := []Networks{}
	for i := range subnets {
		if len(subnets) < 3 {
			break
		}
		l, e := netip.ParsePrefix(subnets[i])
		if e != nil {
			return nil, e
		}
		d, e := netip.ParsePrefix(subnets[(i+1)%len(subnets)])
		if e != nil {
			return nil, e
		}
		p, e := netip.ParsePrefix(subnets[(i+2)%len(subnets)])
		if e != nil {
			return nil, e
		}
		p = netip.PrefixFrom(p.Addr(), 16).Masked()
		if l.Overlaps(d) || l.Overlaps(p) || d.Overlaps(p) {
			continue
		}
		free := true
		for _, r := range routes {
			if l.Overlaps(r) || d.Overlaps(r) || p.Overlaps(r) {
				free = false
				break
			}
		}
		if free {
			out = append(out, Networks{l.String(), d.String(), p.String()})
		}
	}
	if len(out) == 0 {
		return nil, model.Fail("resource.unsupported", fmt.Sprintf("The %d configured subnet candidates overlap host routes; no network was allocated.", len(subnets)))
	}
	return out, nil
}

func PrepareNetworks(ctx context.Context, in *Input) error {
	c, err := LoadCatalog()
	if err != nil {
		return err
	}
	in.NetworkCandidates, err = networkCandidates(ctx, c.Subnets)
	return err
}
