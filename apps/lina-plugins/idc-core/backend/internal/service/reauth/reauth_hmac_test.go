// This file verifies key rotation and key-buffer handling for the HMAC signer.

package reauth

import (
	"bytes"
	"context"
	"testing"
)

func TestHMACSignerVerifiesActiveAndRetainedKeys(t *testing.T) {
	provider := &reauthTestKeyProvider{
		active: "key-v1",
		keys: map[string][]byte{
			"key-v1": []byte("11111111111111111111111111111111"),
			"key-v2": []byte("22222222222222222222222222222222"),
		},
	}
	signerValue, err := NewHMACSigner(provider)
	if err != nil {
		t.Fatalf("create HMAC signer: %v", err)
	}
	message := []byte("operation-bound-proof")
	keyID, err := signerValue.ActiveKeyID(t.Context())
	if err != nil {
		t.Fatalf("resolve active key: %v", err)
	}
	signature, err := signerValue.Sign(t.Context(), keyID, message)
	if err != nil {
		t.Fatalf("sign proof: %v", err)
	}
	provider.active = "key-v2"
	verified, err := signerValue.Verify(t.Context(), "key-v1", message, signature)
	if err != nil || !verified {
		t.Fatalf("verify retained key: verified=%v err=%v", verified, err)
	}
	verified, err = signerValue.Verify(t.Context(), "key-v1", []byte("different"), signature)
	if err != nil || verified {
		t.Fatalf("tampered message verification: verified=%v err=%v", verified, err)
	}
	if len(provider.returned) == 0 {
		t.Fatal("expected provider buffers to be observed")
	}
	for index, returned := range provider.returned {
		if !bytes.Equal(returned, make([]byte, len(returned))) {
			t.Fatalf("provider key buffer %d was not cleared", index)
		}
	}
}

func TestHMACSignerRejectsKeyChangeBetweenSelectionAndSign(t *testing.T) {
	provider := &reauthTestKeyProvider{
		active: "key-v1",
		keys: map[string][]byte{
			"key-v1": []byte("11111111111111111111111111111111"),
			"key-v2": []byte("22222222222222222222222222222222"),
		},
	}
	signerValue, err := NewHMACSigner(provider)
	if err != nil {
		t.Fatalf("create HMAC signer: %v", err)
	}
	keyID, err := signerValue.ActiveKeyID(t.Context())
	if err != nil {
		t.Fatalf("select active key: %v", err)
	}
	provider.active = "key-v2"
	if _, err = signerValue.Sign(t.Context(), keyID, []byte("message")); err == nil {
		t.Fatal("expected changed active key to reject signing")
	}
}

func TestHMACSignerTreatsUnknownVerificationKeyAsInvalidProof(t *testing.T) {
	provider := &reauthTestKeyProvider{
		active: "key-v1",
		keys: map[string][]byte{
			"key-v1": []byte("11111111111111111111111111111111"),
		},
	}
	signerValue, err := NewHMACSigner(provider)
	if err != nil {
		t.Fatalf("create HMAC signer: %v", err)
	}
	verified, err := signerValue.Verify(
		t.Context(), "retired-key", []byte("message"), make([]byte, 32),
	)
	if err != nil || verified {
		t.Fatalf("unknown verification key: verified=%v err=%v", verified, err)
	}
}

type reauthTestKeyProvider struct {
	active   string
	keys     map[string][]byte
	returned [][]byte
}

func (p *reauthTestKeyProvider) Active(context.Context) (ActiveSigningKey, error) {
	key := append([]byte(nil), p.keys[p.active]...)
	p.returned = append(p.returned, key)
	return ActiveSigningKey{KeyID: p.active, Key: key}, nil
}

func (p *reauthTestKeyProvider) Resolve(_ context.Context, keyID string) ([]byte, error) {
	key := append([]byte(nil), p.keys[keyID]...)
	p.returned = append(p.returned, key)
	return key, nil
}
