// This file implements envelope-owned idempotent random credential creation.

package secret

import (
	"context"
	"encoding/base64"
	"errors"
	"io"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const generatedURLSafe256Bytes = 32

// CreateGenerated creates one policy-bound random value inside the secret
// owner. Replays recover the original opaque handle without asking a caller to
// reproduce or observe the generated plaintext.
func (s *serviceImpl) CreateGenerated(
	ctx context.Context,
	input CreateGeneratedInput,
) (*CreateResult, error) {
	normalized, err := s.normalizeGeneratedCreateInput(input)
	if err != nil || s.config.MaximumPlaintextSize < base64.RawURLEncoding.EncodedLen(generatedURLSafe256Bytes) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if err = s.authorize(ctx, normalized.scope, ActionCreate, secretcap.Handle{}, ""); err != nil {
		return nil, err
	}
	if reused, found, replayErr := s.findGeneratedCreateReplay(ctx, normalized); found || replayErr != nil {
		return reused, replayErr
	}
	var result *CreateResult
	err = dao.Secret.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		plain := make([]byte, generatedURLSafe256Bytes)
		if _, readErr := io.ReadFull(s.random, plain); readErr != nil {
			zeroBytes(plain)
			return readErr
		}
		encoded := make([]byte, base64.RawURLEncoding.EncodedLen(len(plain)))
		base64.RawURLEncoding.Encode(encoded, plain)
		zeroBytes(plain)
		defer zeroBytes(encoded)
		normalized.plaintext = encoded
		var createErr error
		result, createErr = s.createPlaintextInTransaction(
			ctx, normalized, requestOperationGenerate256,
		)
		return createErr
	})
	if errors.Is(err, errRecheckIdempotency) {
		reused, found, replayErr := s.findGeneratedCreateReplay(ctx, normalized)
		if found || replayErr != nil {
			return reused, replayErr
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if err != nil {
		return nil, publicSecretError(err)
	}
	return result, nil
}

func (s *serviceImpl) findGeneratedCreateReplay(
	ctx context.Context,
	input normalizedSecretInput,
) (*CreateResult, bool, error) {
	secretRef, err := findReservedSecretNumber(ctx, input)
	if err != nil || secretRef == "" {
		return nil, false, publicOptionalSecretError(err)
	}
	row, err := loadScopedSecret(ctx, input.scope, secretRef, false)
	if err != nil {
		return nil, true, publicSecretError(err)
	}
	if row == nil || row.StatusCode != statusConfigured || row.CurrentVersion != 1 {
		return nil, true, bizerr.NewCode(CodeNotConfigured)
	}
	version, err := loadVersionByIdempotency(
		ctx, input.scope.TenantID, secretRef, input.idempotencyDigest,
	)
	if err != nil || version == nil {
		return nil, true, publicSecretError(firstError(err, errors.New("generated secret replay version is missing")))
	}
	fence := requestFence{
		operation: requestOperationGenerate256, idempotencyDigest: input.idempotencyDigest,
	}
	if err = s.verifyGeneratedReplay(ctx, input.scope, secretRef, version, fence); err != nil {
		return nil, true, err
	}
	handle, err := secretcap.NewHandle(secretRef)
	if err != nil {
		return nil, true, bizerr.NewCode(CodeUnavailable)
	}
	return &CreateResult{Handle: handle, Metadata: metadataFromSecret(row), Reused: true}, true, nil
}

func (s *serviceImpl) verifyGeneratedReplay(
	ctx context.Context,
	scope Scope,
	secretRef string,
	version *entity.SecretVersion,
	fence requestFence,
) error {
	if !validPersistedVersion(version, 1) || !persistedFenceMatches(version, fence) {
		return bizerr.NewCode(CodeIdempotencyConflict)
	}
	wrappers, err := s.loadVersionWrappers(ctx, scope.TenantID, secretRef, version.Version)
	if err != nil {
		return publicSecretError(err)
	}
	keyIDs := make([]string, 0, len(wrappers))
	for _, wrapper := range wrappers {
		keyIDs = append(keyIDs, wrapper.masterKeyID)
	}
	keys, err := s.resolveKeys(ctx, keyIDs)
	if err != nil {
		return err
	}
	defer zeroKeyMap(keys)
	dek, err := unwrapDEK(keys, scope, secretRef, version.Version, wrappers)
	if err != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	defer zeroBytes(dek)
	plaintext, err := openSealed(
		dek,
		mustDecodePersistedHex(version.PayloadNonce),
		mustDecodePersistedHex(version.Ciphertext),
		payloadAAD(scope, secretRef, version.Version),
	)
	if err != nil {
		zeroBytes(plaintext)
		return bizerr.NewCode(CodeUnavailable)
	}
	defer zeroBytes(plaintext)
	decoded := make([]byte, generatedURLSafe256Bytes)
	decodedCount, decodeErr := base64.RawURLEncoding.Decode(decoded, plaintext)
	zeroBytes(decoded)
	if decodeErr != nil || decodedCount != generatedURLSafe256Bytes {
		return bizerr.NewCode(CodeUnavailable)
	}
	expected, err := fingerprint(dek, scope, secretRef, version.Version, fence, plaintext)
	if err != nil || !constantTimeStringEqual(expected, version.RequestFingerprint) {
		return bizerr.NewCode(CodeUnavailable)
	}
	return nil
}
