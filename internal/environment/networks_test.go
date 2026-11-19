package environment

import (
	"net/netip"
	"testing"
)

func TestHostRoutesExcludeContainerRanges(t *testing.T) {
	fixtures := map[string]string{
		"darwin": "Routing tables\nInternet:\nDestination Gateway Flags Netif Expire\ndefault 192.168.1.1 UGScg en0\n10.210/16 link#1 UCS utun0\n172.28 172.28.0.1 UH utun1\n127 127.0.0.1 UCS lo0\n",
		"linux":  `[{"dst":"default"},{"dst":"10.210.0.0/16"},{"dst":"172.28.0.0/16"},{"dst":"127.0.0.0/8"}]`,
	}
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for platform, raw := range fixtures {
		t.Run(platform, func(t *testing.T) {
			routes, err := routePrefixes([]byte(raw), platform)
			if err != nil {
				t.Fatal(err)
			}
			choices, err := chooseNetworkCandidates(catalog.Subnets, routes)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range choices {
				for _, s := range []string{c.LXD, c.Docker, c.Pool} {
					p := netip.MustParsePrefix(s)
					for _, r := range routes {
						if p.Overlaps(r) {
							t.Fatalf("allocated %s overlapping %s", p, r)
						}
					}
				}
			}
		})
	}
	if _, err = chooseNetworkCandidates(catalog.Subnets, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12")}); err == nil {
		t.Fatal("exhausted pool silently allocated")
	}
}

func TestStageManifestSurvivesNewBinary(t *testing.T) {
	in := Input{StageManifest: []Stage{{"old.stage.v1", "old"}}}
	got := Stages(in)
	if len(got) != 1 || got[0].ID != "old.stage.v1" {
		t.Fatal(got)
	}
	got[0].ID = "changed"
	if in.StageManifest[0].ID != "old.stage.v1" {
		t.Fatal("manifest aliased")
	}
}
