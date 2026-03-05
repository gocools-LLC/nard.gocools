package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	DocumentVersion    = "nard.identity/v1"
	AlgorithmED25519   = "ed25519"
	DefaultRotationTTL = 24 * time.Hour
)

var (
	ErrInvalidDocument  = errors.New("invalid identity document")
	ErrInvalidSignature = errors.New("invalid identity signature")
	ErrExpiredDocument  = errors.New("identity document expired")
	ErrRevokedDocument  = errors.New("identity document revoked")
)

type KeyStatus string

const (
	StatusActive  KeyStatus = "active"
	StatusRotated KeyStatus = "rotated"
	StatusRevoked KeyStatus = "revoked"
)

type Document struct {
	Version       string    `json:"version"`
	NodeID        string    `json:"node_id"`
	KeyID         string    `json:"key_id"`
	Algorithm     string    `json:"algorithm"`
	PublicKey     string    `json:"public_key"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	PreviousKeyID string    `json:"previous_key_id,omitempty"`
	Status        KeyStatus `json:"status"`
	Signature     string    `json:"signature"`
}

type Revocation struct {
	NodeID    string    `json:"node_id"`
	KeyID     string    `json:"key_id"`
	Reason    string    `json:"reason"`
	RevokedAt time.Time `json:"revoked_at"`
}

type CompromiseResult struct {
	Revocation  Revocation `json:"revocation"`
	NewDocument Document   `json:"new_document"`
}

type Config struct {
	NodeID      string
	RotationTTL time.Duration
	Clock       func() time.Time
}

type Manager struct {
	mu             sync.RWMutex
	nodeID         string
	rotationTTL    time.Duration
	now            func() time.Time
	sequence       uint64
	currentPrivKey ed25519.PrivateKey
	currentDoc     Document
	history        []Document
	revocations    map[string]Revocation
}

func NewManager(cfg Config) (*Manager, error) {
	nodeID := strings.TrimSpace(cfg.NodeID)
	if nodeID == "" {
		return nil, errors.New("node id is required")
	}

	rotationTTL := cfg.RotationTTL
	if rotationTTL <= 0 {
		rotationTTL = DefaultRotationTTL
	}

	nowFn := cfg.Clock
	if nowFn == nil {
		nowFn = time.Now
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	issuedAt := nowFn().UTC()
	sequence := uint64(1)
	keyID := formatKeyID(nodeID, sequence)
	document, err := createDocument(documentParams{
		NodeID:        nodeID,
		KeyID:         keyID,
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		IssuedAt:      issuedAt,
		ExpiresAt:     issuedAt.Add(rotationTTL),
		PreviousKeyID: "",
		Status:        StatusActive,
	})
	if err != nil {
		return nil, err
	}

	return &Manager{
		nodeID:         nodeID,
		rotationTTL:    rotationTTL,
		now:            nowFn,
		sequence:       sequence,
		currentPrivKey: privateKey,
		currentDoc:     document,
		history:        []Document{},
		revocations:    map[string]Revocation{},
	}, nil
}

func (m *Manager) Current() Document {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentDoc
}

func (m *Manager) History() []Document {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cloned := make([]Document, len(m.history))
	copy(cloned, m.history)
	return cloned
}

func (m *Manager) Revocations() map[string]Revocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cloned := make(map[string]Revocation, len(m.revocations))
	for keyID, revocation := range m.revocations {
		cloned[keyID] = revocation
	}
	return cloned
}

func (m *Manager) Rotate() (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now().UTC()

	previous := m.currentDoc
	previous.Status = StatusRotated
	m.history = append(m.history, previous)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Document{}, err
	}

	m.sequence++
	keyID := formatKeyID(m.nodeID, m.sequence)
	next, err := createDocument(documentParams{
		NodeID:        m.nodeID,
		KeyID:         keyID,
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		IssuedAt:      now,
		ExpiresAt:     now.Add(m.rotationTTL),
		PreviousKeyID: previous.KeyID,
		Status:        StatusActive,
	})
	if err != nil {
		return Document{}, err
	}

	m.currentPrivKey = privateKey
	m.currentDoc = next
	return next, nil
}

func (m *Manager) HandleCompromise(reason string) (CompromiseResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now().UTC()
	compromised := m.currentDoc
	compromised.Status = StatusRevoked
	compromised.ExpiresAt = now
	m.history = append(m.history, compromised)

	revocation := Revocation{
		NodeID:    compromised.NodeID,
		KeyID:     compromised.KeyID,
		Reason:    normalizeReason(reason),
		RevokedAt: now,
	}
	m.revocations[compromised.KeyID] = revocation

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return CompromiseResult{}, err
	}

	m.sequence++
	keyID := formatKeyID(m.nodeID, m.sequence)
	replacement, err := createDocument(documentParams{
		NodeID:        m.nodeID,
		KeyID:         keyID,
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		IssuedAt:      now,
		ExpiresAt:     now.Add(m.rotationTTL),
		PreviousKeyID: compromised.KeyID,
		Status:        StatusActive,
	})
	if err != nil {
		return CompromiseResult{}, err
	}

	m.currentPrivKey = privateKey
	m.currentDoc = replacement

	return CompromiseResult{
		Revocation:  revocation,
		NewDocument: replacement,
	}, nil
}

func VerifyDocument(document Document, asOf time.Time) error {
	return VerifyDocumentWithRevocations(document, asOf, nil)
}

func VerifyDocumentWithRevocations(document Document, asOf time.Time, revocations map[string]Revocation) error {
	if err := validateDocument(document); err != nil {
		return err
	}

	if !document.IssuedAt.IsZero() && asOf.Before(document.IssuedAt) {
		return fmt.Errorf("%w: issued in the future", ErrInvalidDocument)
	}
	if !document.ExpiresAt.IsZero() && !document.ExpiresAt.After(asOf) {
		return ErrExpiredDocument
	}
	if document.Status == StatusRevoked {
		return ErrRevokedDocument
	}
	if revocation, exists := revocations[document.KeyID]; exists {
		if !revocation.RevokedAt.After(asOf) {
			return ErrRevokedDocument
		}
	}

	publicKey, err := decodePublicKey(document.PublicKey)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(document.Signature)
	if err != nil {
		return fmt.Errorf("%w: decode signature: %v", ErrInvalidDocument, err)
	}

	payload, err := canonicalPayload(document)
	if err != nil {
		return err
	}

	if !ed25519.Verify(publicKey, payload, signature) {
		return ErrInvalidSignature
	}
	return nil
}

type documentParams struct {
	NodeID        string
	KeyID         string
	PublicKey     ed25519.PublicKey
	PrivateKey    ed25519.PrivateKey
	IssuedAt      time.Time
	ExpiresAt     time.Time
	PreviousKeyID string
	Status        KeyStatus
}

func createDocument(params documentParams) (Document, error) {
	publicKeyEncoded := base64.StdEncoding.EncodeToString(params.PublicKey)
	document := Document{
		Version:       DocumentVersion,
		NodeID:        params.NodeID,
		KeyID:         params.KeyID,
		Algorithm:     AlgorithmED25519,
		PublicKey:     publicKeyEncoded,
		IssuedAt:      params.IssuedAt.UTC(),
		ExpiresAt:     params.ExpiresAt.UTC(),
		PreviousKeyID: params.PreviousKeyID,
		Status:        params.Status,
	}

	payload, err := canonicalPayload(document)
	if err != nil {
		return Document{}, err
	}

	signature := ed25519.Sign(params.PrivateKey, payload)
	document.Signature = base64.StdEncoding.EncodeToString(signature)

	if err := validateDocument(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

type signableDocument struct {
	Version       string    `json:"version"`
	NodeID        string    `json:"node_id"`
	KeyID         string    `json:"key_id"`
	Algorithm     string    `json:"algorithm"`
	PublicKey     string    `json:"public_key"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	PreviousKeyID string    `json:"previous_key_id,omitempty"`
	Status        KeyStatus `json:"status"`
}

func canonicalPayload(document Document) ([]byte, error) {
	payload := signableDocument{
		Version:       document.Version,
		NodeID:        document.NodeID,
		KeyID:         document.KeyID,
		Algorithm:     document.Algorithm,
		PublicKey:     document.PublicKey,
		IssuedAt:      document.IssuedAt.UTC(),
		ExpiresAt:     document.ExpiresAt.UTC(),
		PreviousKeyID: document.PreviousKeyID,
		Status:        document.Status,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal payload: %v", ErrInvalidDocument, err)
	}
	return bytes, nil
}

func validateDocument(document Document) error {
	if strings.TrimSpace(document.Version) == "" ||
		strings.TrimSpace(document.NodeID) == "" ||
		strings.TrimSpace(document.KeyID) == "" ||
		strings.TrimSpace(document.Algorithm) == "" ||
		strings.TrimSpace(document.PublicKey) == "" ||
		strings.TrimSpace(document.Signature) == "" {
		return fmt.Errorf("%w: missing required fields", ErrInvalidDocument)
	}
	if document.Version != DocumentVersion {
		return fmt.Errorf("%w: unsupported version", ErrInvalidDocument)
	}
	if document.Algorithm != AlgorithmED25519 {
		return fmt.Errorf("%w: unsupported algorithm", ErrInvalidDocument)
	}
	if document.Status == "" {
		return fmt.Errorf("%w: missing status", ErrInvalidDocument)
	}
	if document.ExpiresAt.IsZero() || document.IssuedAt.IsZero() {
		return fmt.Errorf("%w: issued_at and expires_at are required", ErrInvalidDocument)
	}
	if !document.ExpiresAt.After(document.IssuedAt) {
		return fmt.Errorf("%w: expires_at must be after issued_at", ErrInvalidDocument)
	}

	if _, err := decodePublicKey(document.PublicKey); err != nil {
		return err
	}
	return nil
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: decode public key: %v", ErrInvalidDocument, err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: invalid public key length", ErrInvalidDocument)
	}
	return ed25519.PublicKey(decoded), nil
}

func formatKeyID(nodeID string, sequence uint64) string {
	return fmt.Sprintf("%s-k%06d", nodeID, sequence)
}

func normalizeReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "compromised key detected"
	}
	return trimmed
}
