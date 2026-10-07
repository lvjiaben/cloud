package composition

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestBindVerificationSecretRecoversOnlyTheDeterministicGeneratedSecret(t *testing.T) {
	handle, err := secretcap.NewHandle("sec_verification_recovered")
	if err != nil {
		t.Fatal(err)
	}
	manager := &verificationManagerStub{handle: handle, createErr: errors.New("commit response lost")}
	source := &verificationSourceStub{manager: manager, accessor: verificationAccessorStub{}}
	accessor, recovered, err := bindVerificationSecret(t.Context(), source)
	if err != nil {
		t.Fatalf("bind verification secret: %v", err)
	}
	if accessor == nil || !recovered.Equal(handle) || manager.createCalls != 1 || manager.recoverCalls != 1 {
		t.Fatalf("unexpected generated-secret recovery: handle=%v create=%d recover=%d", recovered, manager.createCalls, manager.recoverCalls)
	}
	if manager.idempotencyKey != "notify-verification-hmac-v2" || manager.policy != secretcap.GeneratedSecretPolicyURLSafe256 {
		t.Fatalf("unexpected generated-secret request: key=%q policy=%q", manager.idempotencyKey, manager.policy)
	}
	if source.managerScope != source.accessorScope || source.managerScope.TenantID != 0 || source.managerScope.OwnerPluginID != notifyOwnerPluginID ||
		!regexp.MustCompile(`^notify\.verification\.c[0-9a-f]{32}$`).MatchString(source.managerScope.CallerScope) {
		t.Fatalf("verification secret scope escaped the exact contract: %+v", source.managerScope)
	}
}

type verificationSourceStub struct {
	manager       *verificationManagerStub
	accessor      verificationAccessorStub
	managerScope  secretcap.Scope
	accessorScope secretcap.Scope
}

func (s *verificationSourceStub) Manager(scope secretcap.Scope) (secretcap.Manager, error) {
	s.managerScope = scope
	return s.manager, nil
}
func (s *verificationSourceStub) Accessor(scope secretcap.Scope) (secretcap.Accessor, error) {
	s.accessorScope = scope
	return s.accessor, nil
}

type verificationManagerStub struct {
	handle         secretcap.Handle
	createErr      error
	createCalls    int
	recoverCalls   int
	idempotencyKey string
	policy         string
}

func (m *verificationManagerStub) CreateGenerated(_ context.Context, key, policy string) (secretcap.CreateResult, error) {
	m.createCalls++
	m.idempotencyKey, m.policy = key, policy
	return secretcap.CreateResult{}, m.createErr
}
func (m *verificationManagerStub) RecoverCreate(_ context.Context, key string) (secretcap.CreateResult, error) {
	m.recoverCalls++
	if key != m.idempotencyKey {
		return secretcap.CreateResult{}, errors.New("wrong recovery key")
	}
	return secretcap.CreateResult{Handle: m.handle, Metadata: secretcap.Metadata{Configured: true}, Reused: true}, nil
}
func (m *verificationManagerStub) Create(context.Context, string, secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{}, errors.New("not allowed")
}
func (m *verificationManagerStub) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("not allowed")
}
func (m *verificationManagerStub) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, errors.New("unused")
}
func (m *verificationManagerStub) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, errors.New("not allowed")
}
func (m *verificationManagerStub) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, errors.New("unused")
}

type verificationAccessorStub struct{}

func (verificationAccessorStub) Access(context.Context, secretcap.AccessRequest, secretcap.Consumer) error {
	return nil
}
