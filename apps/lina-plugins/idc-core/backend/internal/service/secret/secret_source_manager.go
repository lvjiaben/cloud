// This file adapts the internal envelope service to the public source manager.

package secret

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type scopedManager struct {
	service *serviceImpl
	scope   Scope
}

var _ secretcap.Manager = (*scopedManager)(nil)
var _ secretcap.GeneratedManager = (*scopedManager)(nil)

// Manager binds lifecycle writes to one validated scope. Every operation still
// passes through the service authorizer, optimistic fences, and idempotency.
func (s *serviceImpl) Manager(scope Scope) (secretcap.Manager, error) {
	normalized, err := normalizeScope(scope)
	if err != nil {
		return nil, err
	}
	return &scopedManager{service: s, scope: normalized}, nil
}

func (m *scopedManager) Create(
	ctx context.Context,
	idempotencyKey string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	plaintext, err := consumeProducedPlaintext(produce)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	defer zeroBytes(plaintext)
	result, err := m.service.Create(ctx, CreateInput{
		Scope: m.scope, IdempotencyKey: idempotencyKey, Plaintext: plaintext,
	})
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	if result == nil {
		return secretcap.CreateResult{}, bizerr.NewCode(CodeUnavailable)
	}
	return secretcap.CreateResult{
		Handle: result.Handle, Metadata: publicMetadata(result.Metadata), Reused: result.Reused,
	}, nil
}

func (m *scopedManager) RecoverCreate(
	ctx context.Context,
	idempotencyKey string,
) (secretcap.CreateResult, error) {
	result, err := m.service.RecoverCreate(ctx, RecoverCreateInput{
		Scope: m.scope, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	if result == nil {
		return secretcap.CreateResult{}, bizerr.NewCode(CodeNotConfigured)
	}
	return secretcap.CreateResult{
		Handle: result.Handle, Metadata: publicMetadata(result.Metadata), Reused: true,
	}, nil
}

func (m *scopedManager) CreateGenerated(
	ctx context.Context,
	idempotencyKey string,
	policy string,
) (secretcap.CreateResult, error) {
	result, err := m.service.CreateGenerated(ctx, CreateGeneratedInput{
		Scope: m.scope, IdempotencyKey: idempotencyKey, Policy: policy,
	})
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	if result == nil {
		return secretcap.CreateResult{}, bizerr.NewCode(CodeUnavailable)
	}
	return secretcap.CreateResult{
		Handle: result.Handle, Metadata: publicMetadata(result.Metadata), Reused: result.Reused,
	}, nil
}

func (m *scopedManager) Replace(
	ctx context.Context,
	handle secretcap.Handle,
	expectedUpdatedAt time.Time,
	idempotencyKey string,
	produce secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	plaintext, err := consumeProducedPlaintext(produce)
	if err != nil {
		return secretcap.MutationResult{}, err
	}
	defer zeroBytes(plaintext)
	result, err := m.service.Update(ctx, UpdateInput{
		Scope: m.scope, Handle: handle, ExpectedUpdatedAt: expectedUpdatedAt,
		IdempotencyKey: idempotencyKey, Plaintext: plaintext,
	})
	if err != nil {
		return secretcap.MutationResult{}, err
	}
	if result == nil {
		return secretcap.MutationResult{}, bizerr.NewCode(CodeUnavailable)
	}
	return secretcap.MutationResult{Metadata: publicMetadata(result.Metadata), Reused: result.Reused}, nil
}

func (m *scopedManager) Metadata(ctx context.Context, handle secretcap.Handle) (secretcap.Metadata, error) {
	metadata, err := m.service.Metadata(ctx, m.scope, handle)
	return publicMetadata(metadata), err
}

func (m *scopedManager) Revoke(
	ctx context.Context,
	handle secretcap.Handle,
	expectedUpdatedAt time.Time,
) (secretcap.Metadata, error) {
	metadata, err := m.service.Revoke(ctx, RevokeInput{
		Scope: m.scope, Handle: handle, ExpectedUpdatedAt: expectedUpdatedAt,
	})
	return publicMetadata(metadata), err
}

func (m *scopedManager) Rotate(ctx context.Context, limit int) (secretcap.RotationResult, error) {
	result, err := m.service.Rotate(ctx, RotateInput{Scope: m.scope, Limit: limit})
	return secretcap.RotationResult{Rewrapped: result.Rewrapped, Remaining: result.Remaining}, err
}

func consumeProducedPlaintext(produce secretcap.PlaintextProducer) (plaintext []byte, err error) {
	if produce == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	calls := 0
	defer func() {
		if recover() != nil {
			zeroBytes(plaintext)
			plaintext = nil
			err = bizerr.NewCode(CodeConsumerFailed)
		}
	}()
	producerErr := produce(func(value []byte) error {
		calls++
		defer zeroBytes(value)
		if calls != 1 || len(value) == 0 || len(value) > secretcap.MaximumManagedPlaintextBytes {
			return bizerr.NewCode(CodeInvalid)
		}
		plaintext = append([]byte(nil), value...)
		return nil
	})
	if producerErr != nil || calls != 1 || len(plaintext) == 0 {
		zeroBytes(plaintext)
		return nil, bizerr.NewCode(CodeConsumerFailed)
	}
	return plaintext, nil
}

func publicMetadata(value Metadata) secretcap.Metadata {
	return secretcap.Metadata{
		Configured: value.Configured, MaskedTail: value.MaskedTail, UpdatedAt: value.UpdatedAt,
	}
}
