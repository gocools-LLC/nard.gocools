package transport

import (
	"context"
	"testing"
	"time"
)

func TestQUICTransportMultiNodeConnectivity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageCh := make(chan []byte, 1)

	nodeB, err := NewQUICTransport("127.0.0.1:0", Hooks{
		OnMessage: func(_ string, payload []byte) {
			messageCh <- payload
		},
	})
	if err != nil {
		t.Fatalf("create nodeB transport failed: %v", err)
	}
	defer nodeB.Close()
	if err := nodeB.Start(ctx); err != nil {
		t.Fatalf("start nodeB failed: %v", err)
	}

	nodeA, err := NewQUICTransport("127.0.0.1:0", Hooks{})
	if err != nil {
		t.Fatalf("create nodeA transport failed: %v", err)
	}
	defer nodeA.Close()
	if err := nodeA.Start(ctx); err != nil {
		t.Fatalf("start nodeA failed: %v", err)
	}

	if err := nodeA.Send(ctx, nodeB.Addr(), []byte("hello")); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	select {
	case payload := <-messageCh:
		if string(payload) != "hello" {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}

	metricsA := nodeA.Metrics()
	metricsB := nodeB.Metrics()
	if metricsA.BytesSent == 0 {
		t.Fatalf("expected bytes sent metric to be >0, got %+v", metricsA)
	}
	if metricsB.BytesReceived == 0 {
		t.Fatalf("expected bytes received metric to be >0, got %+v", metricsB)
	}
}

func TestQUICTransportReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageCh := make(chan []byte, 2)

	nodeB, err := NewQUICTransport("127.0.0.1:0", Hooks{
		OnMessage: func(_ string, payload []byte) {
			messageCh <- payload
		},
	})
	if err != nil {
		t.Fatalf("create nodeB transport failed: %v", err)
	}
	defer nodeB.Close()
	if err := nodeB.Start(ctx); err != nil {
		t.Fatalf("start nodeB failed: %v", err)
	}

	nodeA, err := NewQUICTransport("127.0.0.1:0", Hooks{})
	if err != nil {
		t.Fatalf("create nodeA transport failed: %v", err)
	}
	defer nodeA.Close()
	if err := nodeA.Start(ctx); err != nil {
		t.Fatalf("start nodeA failed: %v", err)
	}

	remote := nodeB.Addr()
	if err := nodeA.Send(ctx, remote, []byte("first")); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	<-messageCh

	if err := nodeA.ClosePeer(remote); err != nil {
		t.Fatalf("close peer failed: %v", err)
	}

	if err := nodeA.Send(ctx, remote, []byte("second")); err != nil {
		t.Fatalf("second send failed: %v", err)
	}

	select {
	case payload := <-messageCh:
		if string(payload) != "second" {
			t.Fatalf("unexpected payload after reconnect: %q", string(payload))
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for reconnect payload")
	}

	metrics := nodeA.Metrics()
	if metrics.Reconnects == 0 {
		t.Fatalf("expected reconnect metric >0, got %+v", metrics)
	}
}
