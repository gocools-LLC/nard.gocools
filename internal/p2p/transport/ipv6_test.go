package transport

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestQUICTransportIPv6LoopbackConnectivity(t *testing.T) {
	requireIPv6Loopback(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageCh := make(chan string, 1)

	nodeB, err := NewQUICTransport("[::1]:0", Hooks{
		OnMessage: func(_ string, payload []byte) {
			messageCh <- string(payload)
		},
	})
	if err != nil {
		t.Fatalf("create IPv6 nodeB transport failed: %v", err)
	}
	defer nodeB.Close()
	if err := nodeB.Start(ctx); err != nil {
		t.Fatalf("start IPv6 nodeB failed: %v", err)
	}

	nodeA, err := NewQUICTransport("[::1]:0", Hooks{})
	if err != nil {
		t.Fatalf("create IPv6 nodeA transport failed: %v", err)
	}
	defer nodeA.Close()
	if err := nodeA.Start(ctx); err != nil {
		t.Fatalf("start IPv6 nodeA failed: %v", err)
	}

	if err := nodeA.Send(ctx, nodeB.Addr(), []byte("ipv6")); err != nil {
		t.Fatalf("IPv6 send failed: %v", err)
	}

	select {
	case payload := <-messageCh:
		if payload != "ipv6" {
			t.Fatalf("unexpected IPv6 payload: %q", payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for IPv6 payload")
	}

	metrics := nodeB.Metrics()
	if metrics.BytesReceived == 0 {
		t.Fatalf("expected IPv6 bytes received metric > 0, got %+v", metrics)
	}
}

func TestQUICTransportDualStackConnectivity(t *testing.T) {
	requireIPv6Loopback(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	v4MessageCh := make(chan string, 1)
	v6MessageCh := make(chan string, 1)

	nodeV4, err := NewQUICTransport("127.0.0.1:0", Hooks{
		OnMessage: func(_ string, payload []byte) {
			v4MessageCh <- string(payload)
		},
	})
	if err != nil {
		t.Fatalf("create v4 transport failed: %v", err)
	}
	defer nodeV4.Close()
	if err := nodeV4.Start(ctx); err != nil {
		t.Fatalf("start v4 transport failed: %v", err)
	}

	nodeV6, err := NewQUICTransport("[::1]:0", Hooks{
		OnMessage: func(_ string, payload []byte) {
			v6MessageCh <- string(payload)
		},
	})
	if err != nil {
		t.Fatalf("create v6 transport failed: %v", err)
	}
	defer nodeV6.Close()
	if err := nodeV6.Start(ctx); err != nil {
		t.Fatalf("start v6 transport failed: %v", err)
	}

	client, err := NewQUICTransport("127.0.0.1:0", Hooks{})
	if err != nil {
		t.Fatalf("create dual-stack client transport failed: %v", err)
	}
	defer client.Close()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("start dual-stack client transport failed: %v", err)
	}

	if err := client.Send(ctx, nodeV4.Addr(), []byte("dual-v4")); err != nil {
		t.Fatalf("send to v4 failed: %v", err)
	}
	if err := client.Send(ctx, nodeV6.Addr(), []byte("dual-v6")); err != nil {
		t.Fatalf("send to v6 failed: %v", err)
	}

	select {
	case payload := <-v4MessageCh:
		if payload != "dual-v4" {
			t.Fatalf("unexpected v4 payload: %q", payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for v4 payload")
	}

	select {
	case payload := <-v6MessageCh:
		if payload != "dual-v6" {
			t.Fatalf("unexpected v6 payload: %q", payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for v6 payload")
	}
}

func requireIPv6Loopback(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v; diagnostics=%s", err, ipv6Diagnostics())
		return
	}
	_ = listener.Close()
}

func ipv6Diagnostics() string {
	return fmt.Sprintf(
		"goos=%s goarch=%s github_actions=%s",
		runtime.GOOS,
		runtime.GOARCH,
		os.Getenv("GITHUB_ACTIONS"),
	)
}
