package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
)

type Hooks struct {
	OnConnect    func(peer string)
	OnDisconnect func(peer string)
	OnReconnect  func(peer string)
	OnMessage    func(peer string, payload []byte)
}

type Metrics struct {
	DialAttempts  uint64 `json:"dial_attempts"`
	Connections   uint64 `json:"connections"`
	Reconnects    uint64 `json:"reconnects"`
	Disconnects   uint64 `json:"disconnects"`
	BytesSent     uint64 `json:"bytes_sent"`
	BytesReceived uint64 `json:"bytes_received"`
}

type QUICTransport struct {
	mu            sync.RWMutex
	listener      *quic.Listener
	addr          string
	hooks         Hooks
	tlsConfig     *tls.Config
	clientTLS     *tls.Config
	quicConfig    *quic.Config
	connections   map[string]*quic.Conn
	hadConnection map[string]bool
	closed        bool

	metrics struct {
		dialAttempts  atomic.Uint64
		connections   atomic.Uint64
		reconnects    atomic.Uint64
		disconnects   atomic.Uint64
		bytesSent     atomic.Uint64
		bytesReceived atomic.Uint64
	}
}

func NewQUICTransport(addr string, hooks Hooks) (*QUICTransport, error) {
	serverTLS, clientTLS, err := generateTLSConfig()
	if err != nil {
		return nil, err
	}

	return &QUICTransport{
		addr:          addr,
		hooks:         hooks,
		tlsConfig:     serverTLS,
		clientTLS:     clientTLS,
		quicConfig:    &quic.Config{KeepAlivePeriod: 15 * time.Second},
		connections:   map[string]*quic.Conn{},
		hadConnection: map[string]bool{},
	}, nil
}

func (t *QUICTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return errors.New("transport is closed")
	}
	if t.listener != nil {
		t.mu.Unlock()
		return nil
	}

	listener, err := quic.ListenAddr(t.addr, t.tlsConfig, t.quicConfig)
	if err != nil {
		t.mu.Unlock()
		return err
	}
	t.listener = listener
	t.mu.Unlock()

	go t.acceptLoop(ctx, listener)
	return nil
}

func (t *QUICTransport) Addr() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.listener == nil {
		return t.addr
	}
	return t.listener.Addr().String()
}

func (t *QUICTransport) Send(ctx context.Context, remoteAddr string, payload []byte) error {
	conn, err := t.getOrDial(ctx, remoteAddr)
	if err != nil {
		return err
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		// One reconnect attempt.
		_ = t.ClosePeer(remoteAddr)
		conn, redialErr := t.getOrDial(ctx, remoteAddr)
		if redialErr != nil {
			return redialErr
		}
		stream, err = conn.OpenStreamSync(ctx)
		if err != nil {
			return err
		}
	}
	defer stream.Close()

	written, err := stream.Write(payload)
	if err != nil {
		return err
	}
	t.metrics.bytesSent.Add(uint64(written))
	return nil
}

func (t *QUICTransport) ClosePeer(remoteAddr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	conn, exists := t.connections[remoteAddr]
	if !exists {
		return nil
	}
	delete(t.connections, remoteAddr)
	t.metrics.disconnects.Add(1)
	if t.hooks.OnDisconnect != nil {
		t.hooks.OnDisconnect(remoteAddr)
	}

	return conn.CloseWithError(0, "peer closed")
}

func (t *QUICTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true

	listener := t.listener
	t.listener = nil

	connections := make([]*quic.Conn, 0, len(t.connections))
	for _, conn := range t.connections {
		connections = append(connections, conn)
	}
	t.connections = map[string]*quic.Conn{}
	t.mu.Unlock()

	for _, conn := range connections {
		_ = conn.CloseWithError(0, "transport closed")
	}
	if listener != nil {
		return listener.Close()
	}
	return nil
}

func (t *QUICTransport) Metrics() Metrics {
	return Metrics{
		DialAttempts:  t.metrics.dialAttempts.Load(),
		Connections:   t.metrics.connections.Load(),
		Reconnects:    t.metrics.reconnects.Load(),
		Disconnects:   t.metrics.disconnects.Load(),
		BytesSent:     t.metrics.bytesSent.Load(),
		BytesReceived: t.metrics.bytesReceived.Load(),
	}
}

func (t *QUICTransport) acceptLoop(ctx context.Context, listener *quic.Listener) {
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			return
		}

		peer := conn.RemoteAddr().String()
		t.trackConnection(peer, conn, false)
		go t.handleConnection(ctx, peer, conn)
	}
}

func (t *QUICTransport) handleConnection(ctx context.Context, peer string, conn *quic.Conn) {
	defer t.unregisterConnection(peer, conn)

	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			return
		}

		payload, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			continue
		}

		t.metrics.bytesReceived.Add(uint64(len(payload)))
		if t.hooks.OnMessage != nil {
			t.hooks.OnMessage(peer, payload)
		}
	}
}

func (t *QUICTransport) getOrDial(ctx context.Context, remoteAddr string) (*quic.Conn, error) {
	t.mu.RLock()
	conn, exists := t.connections[remoteAddr]
	t.mu.RUnlock()
	if exists {
		return conn, nil
	}

	t.metrics.dialAttempts.Add(1)
	newConn, err := quic.DialAddr(ctx, remoteAddr, t.clientTLS, t.quicConfig)
	if err != nil {
		return nil, err
	}

	t.trackConnection(remoteAddr, newConn, true)
	go t.handleConnection(ctx, remoteAddr, newConn)
	return newConn, nil
}

func (t *QUICTransport) trackConnection(peer string, conn *quic.Conn, fromDial bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	previouslyKnown := t.hadConnection[peer]
	t.hadConnection[peer] = true
	t.connections[peer] = conn
	t.metrics.connections.Add(1)

	if fromDial && previouslyKnown {
		t.metrics.reconnects.Add(1)
		if t.hooks.OnReconnect != nil {
			t.hooks.OnReconnect(peer)
		}
	}
	if t.hooks.OnConnect != nil {
		t.hooks.OnConnect(peer)
	}
}

func (t *QUICTransport) unregisterConnection(peer string, conn *quic.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()

	current, exists := t.connections[peer]
	if !exists || current != conn {
		return
	}
	delete(t.connections, peer)
	t.metrics.disconnects.Add(1)
	if t.hooks.OnDisconnect != nil {
		t.hooks.OnDisconnect(peer)
	}
}

func generateTLSConfig() (*tls.Config, *tls.Config, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "nard-transport",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, err
	}

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		NextProtos:   []string{"nard-quic-v1"},
		MinVersion:   tls.VersionTLS13,
	}
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"nard-quic-v1"},
		MinVersion:         tls.VersionTLS13,
	}

	return serverTLS, clientTLS, nil
}
