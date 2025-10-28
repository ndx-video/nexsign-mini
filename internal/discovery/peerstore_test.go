// Package discovery tests exercise the PeerStore behavior used by the
// mDNS discovery component. Tests cover basic add/list semantics and
// concurrency under concurrent add/remove operations.
package discovery

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/grandcat/zeroconf"
)

func TestPeerStoreAddAndList(t *testing.T) {
	ps := NewPeerStore()

	entry := &zeroconf.ServiceEntry{}
	entry.Instance = "node-1"
	entry.Port = 8080
	entry.AddrIPv4 = []net.IP{net.ParseIP("192.0.2.10")}
	entry.Text = []string{"pub=deadbeef", "ver=0.1"}

	ps.AddFromServiceEntry(entry)

	peers := ps.List()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	p := peers[0]
	if p.Instance != "node-1" {
		t.Errorf("unexpected instance: %s", p.Instance)
	}
	if p.Port != 8080 {
		t.Errorf("unexpected port: %d", p.Port)
	}
	if len(p.Addrs) != 1 || !p.Addrs[0].Equal(net.ParseIP("192.0.2.10")) {
		t.Errorf("unexpected address: %+v", p.Addrs)
	}
	if p.Txt["pub"] != "deadbeef" {
		t.Errorf("txt pub missing or wrong: %v", p.Txt)
	}
}

func TestPeerStoreConcurrency(t *testing.T) {
	ps := NewPeerStore()
	var wg sync.WaitGroup
	add := func(id string, ip string, port int) {
		defer wg.Done()
		entry := &zeroconf.ServiceEntry{}
		entry.Instance = id
		entry.Port = port
		entry.AddrIPv4 = []net.IP{net.ParseIP(ip)}
		entry.Text = []string{"pub=deadbeef"}
		ps.AddFromServiceEntry(entry)
	}
	remove := func(id string) {
		defer wg.Done()
		ps.Remove(id)
	}

	// spawn adds
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go add(fmt.Sprintf("node-%d", i), "192.0.2.1", 8000+i)
	}
	// spawn removes for half of them
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go remove(fmt.Sprintf("node-%d", i))
	}
	wg.Wait()

	peers := ps.List()
	// Due to concurrency timing, final count may vary between 25 (all removes after adds)
	// and 50 (all removes happened before adds). Assert it is within expected bounds.
	if len(peers) < 25 || len(peers) > 50 {
		t.Fatalf("expected between 25 and 50 peers remaining, got %d", len(peers))
	}
}
