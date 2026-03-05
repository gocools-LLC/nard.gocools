package identity

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestIdentityDocumentFormatAndVerification(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	now := base

	manager, err := NewManager(Config{
		NodeID:      "node-a",
		RotationTTL: 1 * time.Hour,
		Clock:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}

	document := manager.Current()
	if document.Version != DocumentVersion {
		t.Fatalf("unexpected document version: %s", document.Version)
	}
	if document.Algorithm != AlgorithmED25519 {
		t.Fatalf("unexpected algorithm: %s", document.Algorithm)
	}
	if document.Status != StatusActive {
		t.Fatalf("expected active status, got %s", document.Status)
	}
	if document.NodeID != "node-a" || document.KeyID == "" || document.PublicKey == "" || document.Signature == "" {
		t.Fatalf("document missing required values: %+v", document)
	}

	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(payload, &asMap); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	required := []string{
		"version",
		"node_id",
		"key_id",
		"algorithm",
		"public_key",
		"issued_at",
		"expires_at",
		"status",
		"signature",
	}
	for _, field := range required {
		if _, exists := asMap[field]; !exists {
			t.Fatalf("missing identity document field %q", field)
		}
	}

	if err := VerifyDocument(document, base.Add(5*time.Minute)); err != nil {
		t.Fatalf("verify document failed: %v", err)
	}
}

func TestRotationProcessCreatesLinkedDocument(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	now := base

	manager, err := NewManager(Config{
		NodeID:      "node-a",
		RotationTTL: 2 * time.Hour,
		Clock:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}

	initial := manager.Current()
	now = now.Add(45 * time.Minute)
	rotated, err := manager.Rotate()
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	if rotated.KeyID == initial.KeyID {
		t.Fatalf("expected new key id, old=%s new=%s", initial.KeyID, rotated.KeyID)
	}
	if rotated.PreviousKeyID != initial.KeyID {
		t.Fatalf("expected previous key link to %s, got %s", initial.KeyID, rotated.PreviousKeyID)
	}
	if rotated.Status != StatusActive {
		t.Fatalf("expected active rotated key, got %s", rotated.Status)
	}
	if err := VerifyDocument(rotated, now.Add(1*time.Minute)); err != nil {
		t.Fatalf("verify rotated document failed: %v", err)
	}

	history := manager.History()
	if len(history) != 1 {
		t.Fatalf("expected one historical key, got %d", len(history))
	}
	if history[0].Status != StatusRotated {
		t.Fatalf("expected history status rotated, got %s", history[0].Status)
	}
}

func TestCompromisedKeyResponseRevokesAndRotates(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	now := base

	manager, err := NewManager(Config{
		NodeID:      "node-a",
		RotationTTL: 90 * time.Minute,
		Clock:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}

	compromised := manager.Current()
	now = now.Add(10 * time.Minute)
	result, err := manager.HandleCompromise("suspected private key leak")
	if err != nil {
		t.Fatalf("handle compromise failed: %v", err)
	}

	if result.Revocation.KeyID != compromised.KeyID {
		t.Fatalf("expected revocation for %s, got %+v", compromised.KeyID, result.Revocation)
	}
	if result.NewDocument.KeyID == compromised.KeyID {
		t.Fatalf("expected replacement key to differ from compromised key")
	}
	if result.NewDocument.PreviousKeyID != compromised.KeyID {
		t.Fatalf("expected replacement previous key %s, got %s", compromised.KeyID, result.NewDocument.PreviousKeyID)
	}

	revocations := manager.Revocations()
	if _, exists := revocations[compromised.KeyID]; !exists {
		t.Fatalf("expected revocation map to include compromised key %s", compromised.KeyID)
	}

	err = VerifyDocumentWithRevocations(compromised, now.Add(1*time.Second), revocations)
	if !errors.Is(err, ErrRevokedDocument) {
		t.Fatalf("expected ErrRevokedDocument for compromised key, got %v", err)
	}

	if err := VerifyDocumentWithRevocations(result.NewDocument, now.Add(1*time.Second), revocations); err != nil {
		t.Fatalf("expected replacement key to verify, got %v", err)
	}
}
