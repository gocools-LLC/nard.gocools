package peers

import (
	"testing"
	"time"
)

func TestDiscoveryDualStackPeerAddresses(t *testing.T) {
	service := NewService(Config{
		Bootstrap: []Peer{
			{NodeID: "node-v6", Address: "[2001:db8::10]:9000"},
		},
		TTL: 5 * time.Minute,
	})

	service.Upsert(Peer{NodeID: "node-v4", Address: "10.0.0.10:9000"})
	service.Upsert(Peer{NodeID: "node-v6-local", Address: "[::1]:9000"})

	discovered := service.Discover(0)
	if len(discovered) != 3 {
		t.Fatalf("expected 3 discovered peers, got %d", len(discovered))
	}

	expected := map[string]string{
		"node-v4":       "10.0.0.10:9000",
		"node-v6":       "[2001:db8::10]:9000",
		"node-v6-local": "[::1]:9000",
	}
	for _, peer := range discovered {
		addr, ok := expected[peer.NodeID]
		if !ok {
			t.Fatalf("unexpected peer in discovery output: %+v", peer)
		}
		if peer.Address != addr {
			t.Fatalf("expected %s address %s, got %s", peer.NodeID, addr, peer.Address)
		}
	}
}
