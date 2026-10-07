package composition

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type captureManager struct {
	keys      []string
	plaintext [][]byte
	handle    secretcap.Handle
	err       error
}

func (m *captureManager) Create(
	_ context.Context,
	idempotencyKey string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	m.keys = append(m.keys, idempotencyKey)
	if m.err != nil {
		return secretcap.CreateResult{}, m.err
	}
	err := produce(func(value []byte) error {
		m.plaintext = append(m.plaintext, append([]byte(nil), value...))
		return nil
	})
	return secretcap.CreateResult{Handle: m.handle}, err
}

func (m *captureManager) RecoverCreate(context.Context, string) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{Handle: m.handle, Reused: true}, m.err
}

func (*captureManager) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, nil
}
func (*captureManager) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}
func (*captureManager) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}
func (*captureManager) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func TestChannelSecretSinkEncryptsConfidentialOutput(t *testing.T) {
	handle, err := secretcap.NewHandle("SEC_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	manager := &captureManager{handle: handle}
	sink := &channelSecretSink{manager: manager}
	request := resourcecap.SecretStoreRequest{
		RequestID:  "operation_123",
		Capability: resourcecap.CapabilityReinstall,
		Name:       "generated-credential",
		Policy:     resourcecap.SecretOutputPolicyOneTimeCredential,
	}
	plaintext := []byte("generated-password-value")
	stored, err := resourcecap.StoreSecret(context.Background(), sink, request, func(consume resourcecap.SecretConsumer) error {
		return consume(plaintext)
	})
	if err != nil || !stored.Equal(handle) {
		t.Fatalf("store confidential output: handle=%v err=%v", stored, err)
	}
	if len(manager.keys) != 1 || len(manager.plaintext) != 1 ||
		!bytes.Equal(manager.plaintext[0], []byte("generated-password-value")) {
		t.Fatalf("manager did not receive one bounded plaintext: keys=%v plaintext=%q", manager.keys, manager.plaintext)
	}
	if !allZeroBytes(plaintext) {
		t.Fatalf("Provider plaintext was not cleared: %x", plaintext)
	}
	if strings.Contains(manager.keys[0], request.RequestID) || strings.Contains(manager.keys[0], request.Name) {
		t.Fatalf("idempotency key leaked request metadata: %q", manager.keys[0])
	}
	if manager.keys[0] != secretOutputIdempotencyKey(request) {
		t.Fatalf("idempotency key is not deterministic: got=%q", manager.keys[0])
	}
}

func TestChannelSecretSinkCollapsesManagerFailure(t *testing.T) {
	const sensitive = "password=must-not-leak"
	sink := &channelSecretSink{manager: &captureManager{err: errors.New(sensitive)}}
	request := resourcecap.SecretStoreRequest{
		RequestID:  "operation_456",
		Capability: resourcecap.CapabilityConsole,
		Name:       "console-ticket",
		Policy:     resourcecap.SecretOutputPolicyShortLivedTicket,
		ExpiresAt:  time.Now().UTC().Add(time.Minute),
	}
	_, err := sink.Store(context.Background(), request, func(resourcecap.SecretConsumer) error { return nil })
	if err == nil || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("manager error was not safely collapsed: %v", err)
	}
}

func allZeroBytes(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}
