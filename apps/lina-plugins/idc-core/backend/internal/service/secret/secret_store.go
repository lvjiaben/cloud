// This file implements transactional secret creation, update, metadata, and revocation.

package secret

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

var errRecheckIdempotency = errors.New("recheck secret idempotency outside transaction")

// Create encrypts and persists a new secret, or safely reuses an identical
// request. Number reservation, aggregate, payload, wrapper, and assignment are
// committed atomically.
func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*CreateResult, error) {
	normalized, err := s.normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(normalized.plaintext)
	if err = s.authorize(ctx, normalized.scope, ActionCreate, secretcap.Handle{}, ""); err != nil {
		return nil, err
	}
	if reused, found, reuseErr := s.findCreateReplay(ctx, normalized); found || reuseErr != nil {
		return reused, reuseErr
	}
	var result *CreateResult
	err = dao.Secret.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var createErr error
		result, createErr = s.createInTransaction(ctx, normalized)
		return createErr
	})
	if errors.Is(err, errRecheckIdempotency) {
		reused, found, reuseErr := s.findCreateReplay(ctx, normalized)
		if found || reuseErr != nil {
			return reused, reuseErr
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if err != nil {
		return nil, publicSecretError(err)
	}
	return result, nil
}

// RecoverCreate returns the already committed create replay for one exact
// scope and idempotency key. Missing, partial, cross-scope, and non-create
// records deliberately share the same fail-closed result.
func (s *serviceImpl) RecoverCreate(ctx context.Context, input RecoverCreateInput) (*CreateResult, error) {
	scope, err := normalizeScope(input.Scope)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest, err := normalizeIdempotencyKey(input.IdempotencyKey)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if err = s.authorize(ctx, scope, ActionRecoverCreate, secretcap.Handle{}, ""); err != nil {
		return nil, err
	}
	normalized := normalizedSecretInput{
		scope:             scope,
		idempotencyDigest: idempotencyDigest,
		reservationScope:  digestString(scope.OwnerPluginID + "\x00" + scope.CallerScope),
	}
	secretRef, err := findReservedSecretNumber(ctx, normalized)
	if err != nil || secretRef == "" {
		return nil, recoverCreateUnavailable(err)
	}
	row, err := loadScopedSecret(ctx, scope, secretRef, false)
	if err != nil || row == nil || row.StatusCode != statusConfigured {
		return nil, recoverCreateUnavailable(err)
	}
	version, err := loadVersionByIdempotency(ctx, scope.TenantID, secretRef, idempotencyDigest)
	if err != nil || version == nil || version.Version != 1 || !persistedFenceMatches(version, requestFence{
		operation: requestOperationCreate, idempotencyDigest: idempotencyDigest,
	}) {
		return nil, recoverCreateUnavailable(err)
	}
	handle, err := secretcap.NewHandle(secretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeNotConfigured)
	}
	return &CreateResult{Handle: handle, Metadata: metadataFromSecret(row), Reused: true}, nil
}

func recoverCreateUnavailable(err error) error {
	if err != nil {
		return publicSecretError(err)
	}
	return bizerr.NewCode(CodeNotConfigured)
}

func (s *serviceImpl) createInTransaction(
	ctx context.Context,
	input normalizedSecretInput,
) (*CreateResult, error) {
	return s.createPlaintextInTransaction(ctx, input, requestOperationCreate)
}

func (s *serviceImpl) createPlaintextInTransaction(
	ctx context.Context,
	input normalizedSecretInput,
	operation string,
) (*CreateResult, error) {
	secretRef, err := s.reserveSecretNumber(ctx, input)
	if err != nil {
		return nil, err
	}
	existing, err := loadScopedSecret(ctx, input.scope, secretRef, true)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errRecheckIdempotency
	}
	insertResult, err := dao.Secret.Ctx(ctx).Data(do.Secret{
		TenantId:       input.scope.TenantID,
		SecretNo:       secretRef,
		OwnerPluginId:  input.scope.OwnerPluginID,
		CallerScope:    input.scope.CallerScope,
		StatusCode:     statusConfigured,
		CurrentVersion: 1,
		MaskedTail:     maskedTail(input.plaintext),
	}).InsertIgnore()
	if err != nil {
		return nil, gerror.Wrap(err, "insert secret aggregate")
	}
	affected, err := insertResult.RowsAffected()
	if err != nil {
		return nil, gerror.Wrap(err, "read secret aggregate insert result")
	}
	if affected == 0 {
		return nil, errRecheckIdempotency
	}
	secretRow, err := loadScopedSecret(ctx, input.scope, secretRef, false)
	if err != nil || secretRow == nil {
		return nil, gerror.New("secret aggregate insert did not produce a scoped row")
	}
	active, err := s.activeWriteKey(ctx, input.scope)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(active.Key)
	fence := requestFence{
		operation: operation, idempotencyDigest: input.idempotencyDigest,
	}
	envelope, err := s.encryptEnvelope(active, input.scope, secretRef, 1, fence, input.plaintext)
	if err != nil {
		return nil, err
	}
	if err = insertEnvelope(ctx, input.scope.TenantID, secretRef, 1, fence, envelope); err != nil {
		return nil, err
	}
	if err = assignSecretNumber(ctx, secretRef, secretRow.Id, databaseTime(s.now())); err != nil {
		return nil, err
	}
	handle, err := secretcap.NewHandle(secretRef)
	if err != nil {
		return nil, gerror.Wrap(err, "construct persisted secret handle")
	}
	return &CreateResult{
		Handle:   handle,
		Metadata: metadataFromSecret(secretRow),
	}, nil
}

// Update appends one immutable encrypted version and advances the aggregate
// with a scoped optimistic compare-and-swap.
func (s *serviceImpl) Update(ctx context.Context, input UpdateInput) (*UpdateResult, error) {
	normalized, secretRef, expectedUpdatedAt, err := s.normalizeUpdateInput(input)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(normalized.plaintext)
	if err = s.authorize(ctx, normalized.scope, ActionUpdate, input.Handle, ""); err != nil {
		return nil, err
	}
	if reused, found, reuseErr := s.findUpdateReplay(ctx, normalized, secretRef, expectedUpdatedAt); found || reuseErr != nil {
		return reused, reuseErr
	}

	var result *UpdateResult
	err = dao.Secret.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := loadScopedSecret(ctx, normalized.scope, secretRef, true)
		if loadErr != nil {
			return loadErr
		}
		if row == nil || row.StatusCode != statusConfigured {
			return bizerr.NewCode(CodeNotConfigured)
		}
		replay, replayErr := loadVersionByIdempotency(ctx, normalized.scope.TenantID, secretRef, normalized.idempotencyDigest)
		if replayErr != nil {
			return replayErr
		}
		if replay != nil {
			return errRecheckIdempotency
		}
		if row.UpdatedAt == nil || !databaseTime(*row.UpdatedAt).Equal(expectedUpdatedAt) {
			return bizerr.NewCode(CodeVersionConflict)
		}
		newVersion := row.CurrentVersion + 1
		active, keyErr := s.activeWriteKey(ctx, normalized.scope)
		if keyErr != nil {
			return keyErr
		}
		defer zeroBytes(active.Key)
		fence := requestFence{
			operation: requestOperationUpdate, idempotencyDigest: normalized.idempotencyDigest,
			parentVersion: row.CurrentVersion, expectedUpdatedAt: expectedUpdatedAt,
		}
		envelope, encryptErr := s.encryptEnvelope(
			active,
			normalized.scope,
			secretRef,
			newVersion,
			fence,
			normalized.plaintext,
		)
		if encryptErr != nil {
			return encryptErr
		}
		if insertErr := insertEnvelope(
			ctx,
			normalized.scope.TenantID,
			secretRef,
			newVersion,
			fence,
			envelope,
		); insertErr != nil {
			return insertErr
		}
		updateResult, updateErr := dao.Secret.Ctx(ctx).
			Where(do.Secret{
				TenantId:       normalized.scope.TenantID,
				SecretNo:       secretRef,
				OwnerPluginId:  normalized.scope.OwnerPluginID,
				CallerScope:    normalized.scope.CallerScope,
				StatusCode:     statusConfigured,
				CurrentVersion: row.CurrentVersion,
			}).
			Data(do.Secret{
				CurrentVersion: newVersion,
				MaskedTail:     maskedTail(normalized.plaintext),
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "advance secret aggregate")
		}
		if updateErr = requireOneRow(updateResult, "advance secret aggregate"); updateErr != nil {
			return bizerr.NewCode(CodeVersionConflict)
		}
		persisted, reloadErr := loadScopedSecret(ctx, normalized.scope, secretRef, false)
		if reloadErr != nil || persisted == nil || persisted.CurrentVersion != newVersion {
			return firstError(reloadErr, gerror.New("advanced secret aggregate could not be reloaded"))
		}
		result = &UpdateResult{Metadata: metadataFromSecret(persisted)}
		return nil
	})
	if errors.Is(err, errRecheckIdempotency) {
		reused, found, reuseErr := s.findUpdateReplay(ctx, normalized, secretRef, expectedUpdatedAt)
		if found || reuseErr != nil {
			return reused, reuseErr
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if err != nil {
		return nil, publicSecretError(err)
	}
	return result, nil
}

// Metadata returns only the configured flag, masked tail, and update time for
// one already-held scoped handle. It offers no list or discovery operation.
func (s *serviceImpl) Metadata(
	ctx context.Context,
	scope Scope,
	handle secretcap.Handle,
) (Metadata, error) {
	normalized, err := normalizeScope(scope)
	if err != nil || handle.IsZero() || !validSecretRef(handle.OpaqueRef()) {
		return Metadata{}, bizerr.NewCode(CodeInvalid)
	}
	if err = s.authorize(ctx, normalized, ActionMetadata, handle, ""); err != nil {
		return Metadata{}, err
	}
	row, err := loadScopedSecret(ctx, normalized, handle.OpaqueRef(), false)
	if err != nil {
		return Metadata{}, publicSecretError(err)
	}
	return metadataFromSecret(row), nil
}

// Revoke permanently disables access while retaining authenticated ciphertext
// and wrappers for audit-safe operational retention.
func (s *serviceImpl) Revoke(ctx context.Context, input RevokeInput) (Metadata, error) {
	scope, err := normalizeScope(input.Scope)
	if err != nil || input.Handle.IsZero() || !validSecretRef(input.Handle.OpaqueRef()) || input.ExpectedUpdatedAt.IsZero() {
		return Metadata{}, bizerr.NewCode(CodeInvalid)
	}
	if err = s.authorize(ctx, scope, ActionRevoke, input.Handle, ""); err != nil {
		return Metadata{}, err
	}
	secretRef := input.Handle.OpaqueRef()
	expectedUpdatedAt := databaseTime(input.ExpectedUpdatedAt)
	var metadata Metadata
	err = dao.Secret.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := loadScopedSecret(ctx, scope, secretRef, true)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeNotConfigured)
		}
		if row.StatusCode == statusRevoked {
			metadata = metadataFromSecret(row)
			return nil
		}
		if row.StatusCode != statusConfigured {
			return bizerr.NewCode(CodeNotConfigured)
		}
		if row.UpdatedAt == nil || !databaseTime(*row.UpdatedAt).Equal(expectedUpdatedAt) {
			return bizerr.NewCode(CodeVersionConflict)
		}
		now := databaseTime(s.now())
		result, updateErr := dao.Secret.Ctx(ctx).
			Where(do.Secret{
				TenantId:       scope.TenantID,
				SecretNo:       secretRef,
				OwnerPluginId:  scope.OwnerPluginID,
				CallerScope:    scope.CallerScope,
				StatusCode:     statusConfigured,
				CurrentVersion: row.CurrentVersion,
			}).
			Data(do.Secret{StatusCode: statusRevoked, MaskedTail: "", RevokedAt: &now}).
			Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "revoke secret")
		}
		if updateErr = requireOneRow(result, "revoke secret"); updateErr != nil {
			return bizerr.NewCode(CodeVersionConflict)
		}
		persisted, reloadErr := loadScopedSecret(ctx, scope, secretRef, false)
		if reloadErr != nil || persisted == nil || persisted.StatusCode != statusRevoked {
			return firstError(reloadErr, gerror.New("revoked secret aggregate could not be reloaded"))
		}
		metadata = metadataFromSecret(persisted)
		return nil
	})
	if err != nil {
		return Metadata{}, publicSecretError(err)
	}
	return metadata, nil
}

func (s *serviceImpl) findCreateReplay(
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
	if row == nil || row.StatusCode != statusConfigured {
		return nil, true, bizerr.NewCode(CodeNotConfigured)
	}
	version, err := loadVersionByIdempotency(ctx, input.scope.TenantID, secretRef, input.idempotencyDigest)
	if err != nil || version == nil {
		return nil, true, publicSecretError(firstError(err, gerror.New("secret create replay version is missing")))
	}
	fence := requestFence{
		operation: requestOperationCreate, idempotencyDigest: input.idempotencyDigest,
	}
	if err = s.verifyReplayFingerprint(ctx, input.scope, secretRef, version, fence, input.plaintext); err != nil {
		return nil, true, err
	}
	handle, err := secretcap.NewHandle(secretRef)
	if err != nil {
		return nil, true, bizerr.NewCode(CodeUnavailable)
	}
	return &CreateResult{Handle: handle, Metadata: metadataFromSecret(row), Reused: true}, true, nil
}

func (s *serviceImpl) findUpdateReplay(
	ctx context.Context,
	input normalizedSecretInput,
	secretRef string,
	expectedUpdatedAt time.Time,
) (*UpdateResult, bool, error) {
	row, err := loadScopedSecret(ctx, input.scope, secretRef, false)
	if err != nil {
		return nil, true, publicSecretError(err)
	}
	if row == nil || row.StatusCode != statusConfigured {
		return nil, true, bizerr.NewCode(CodeNotConfigured)
	}
	version, err := loadVersionByIdempotency(ctx, input.scope.TenantID, secretRef, input.idempotencyDigest)
	if err != nil {
		return nil, true, publicSecretError(err)
	}
	if version == nil {
		return nil, false, nil
	}
	fence := requestFence{
		operation: requestOperationUpdate, idempotencyDigest: input.idempotencyDigest,
		parentVersion: version.Version - 1, expectedUpdatedAt: expectedUpdatedAt,
	}
	if err = s.verifyReplayFingerprint(ctx, input.scope, secretRef, version, fence, input.plaintext); err != nil {
		return nil, true, err
	}
	return &UpdateResult{Metadata: metadataFromSecret(row), Reused: true}, true, nil
}

func (s *serviceImpl) verifyReplayFingerprint(
	ctx context.Context,
	scope Scope,
	secretRef string,
	version *entity.SecretVersion,
	fence requestFence,
	plaintext []byte,
) error {
	if !persistedFenceMatches(version, fence) {
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
	expected, err := fingerprint(dek, scope, secretRef, version.Version, fence, plaintext)
	if err != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	if !constantTimeStringEqual(expected, version.RequestFingerprint) {
		return bizerr.NewCode(CodeIdempotencyConflict)
	}
	return nil
}

func insertEnvelope(
	ctx context.Context,
	tenantID int,
	secretRef string,
	version int,
	fence requestFence,
	envelope envelopeWrite,
) error {
	if _, err := dao.SecretVersion.Ctx(ctx).Data(do.SecretVersion{
		TenantId:                 tenantID,
		SecretNo:                 secretRef,
		Version:                  version,
		EnvelopeVersion:          envelopeVersion,
		CipherAlgorithm:          cipherAlgorithm,
		PayloadNonce:             hex.EncodeToString(envelope.payloadNonce),
		Ciphertext:               hex.EncodeToString(envelope.ciphertext),
		RequestFingerprint:       envelope.requestFingerprint,
		IdempotencyKeyDigest:     fence.idempotencyDigest,
		RequestOperation:         fence.operation,
		RequestParentVersion:     fence.parentVersion,
		RequestExpectedUpdatedAt: nullableRequestFenceTime(fence),
	}).Insert(); err != nil {
		return gerror.Wrap(err, "insert encrypted secret payload")
	}
	if _, err := dao.SecretKeyWrap.Ctx(ctx).Data(do.SecretKeyWrap{
		TenantId:        tenantID,
		SecretNo:        secretRef,
		SecretVersion:   version,
		EnvelopeVersion: envelopeVersion,
		MasterKeyId:     envelope.masterKeyID,
		WrapAlgorithm:   cipherAlgorithm,
		WrapNonce:       hex.EncodeToString(envelope.wrapNonce),
		WrappedDek:      hex.EncodeToString(envelope.wrappedDEK),
	}).Insert(); err != nil {
		return gerror.Wrap(err, "insert encrypted secret key wrapper")
	}
	return nil
}

func nullableRequestFenceTime(fence requestFence) *time.Time {
	if fence.expectedUpdatedAt.IsZero() {
		return nil
	}
	value := databaseTime(fence.expectedUpdatedAt)
	return &value
}

func persistedFenceMatches(version *entity.SecretVersion, expected requestFence) bool {
	if version == nil || version.RequestOperation != expected.operation ||
		version.IdempotencyKeyDigest != expected.idempotencyDigest ||
		version.RequestParentVersion != expected.parentVersion {
		return false
	}
	if expected.expectedUpdatedAt.IsZero() {
		return version.RequestExpectedUpdatedAt == nil
	}
	return version.RequestExpectedUpdatedAt != nil &&
		databaseTime(*version.RequestExpectedUpdatedAt).Equal(databaseTime(expected.expectedUpdatedAt))
}

func loadScopedSecret(ctx context.Context, scope Scope, secretRef string, lock bool) (*entity.Secret, error) {
	model := dao.Secret.Ctx(ctx).Where(do.Secret{
		TenantId:      scope.TenantID,
		SecretNo:      secretRef,
		OwnerPluginId: scope.OwnerPluginID,
		CallerScope:   scope.CallerScope,
	})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.Secret
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load scoped secret")
	}
	return row, nil
}

func loadVersionByIdempotency(
	ctx context.Context,
	tenantID int,
	secretRef string,
	idempotencyDigest string,
) (*entity.SecretVersion, error) {
	var row *entity.SecretVersion
	if err := dao.SecretVersion.Ctx(ctx).Where(do.SecretVersion{
		TenantId:             tenantID,
		SecretNo:             secretRef,
		IdempotencyKeyDigest: idempotencyDigest,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load secret version by idempotency scope")
	}
	return row, nil
}

func (s *serviceImpl) reserveSecretNumber(ctx context.Context, input normalizedSecretInput) (string, error) {
	if existing, err := findReservedSecretNumber(ctx, input); err != nil || existing != "" {
		return existing, err
	}
	for range maximumNumberAttempts {
		candidate, err := newOpaqueSecretNumber(s.random)
		if err != nil {
			return "", err
		}
		result, err := dao.BusinessNumber.Ctx(ctx).Data(do.BusinessNumber{
			TenantId:               idccap.DefaultTenantID,
			NumberType:             secretNumberType,
			BusinessNo:             candidate,
			ReservationScopeDigest: input.reservationScope,
			IdempotencyKeyDigest:   input.idempotencyDigest,
		}).InsertIgnore()
		if err != nil {
			return "", gerror.Wrap(err, "reserve secret business number")
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return "", gerror.Wrap(err, "read secret number reservation result")
		}
		if affected == 1 {
			return candidate, nil
		}
		if existing, findErr := findReservedSecretNumber(ctx, input); findErr != nil || existing != "" {
			return existing, findErr
		}
	}
	return "", gerror.New("secret business number collision retry limit reached")
}

func findReservedSecretNumber(ctx context.Context, input normalizedSecretInput) (string, error) {
	var row *entity.BusinessNumber
	if err := dao.BusinessNumber.Ctx(ctx).Where(do.BusinessNumber{
		TenantId:               idccap.DefaultTenantID,
		NumberType:             secretNumberType,
		ReservationScopeDigest: input.reservationScope,
		IdempotencyKeyDigest:   input.idempotencyDigest,
	}).Scan(&row); err != nil {
		return "", gerror.Wrap(err, "load secret number reservation")
	}
	if row == nil {
		return "", nil
	}
	return row.BusinessNo, nil
}

func assignSecretNumber(ctx context.Context, secretRef string, ownerID int64, now time.Time) error {
	result, err := dao.BusinessNumber.Ctx(ctx).
		Where(do.BusinessNumber{
			TenantId:   idccap.DefaultTenantID,
			NumberType: secretNumberType,
			BusinessNo: secretRef,
		}).
		Data(do.BusinessNumber{
			OwnerType:       secretNumberOwnerType,
			OwnerInternalId: ownerID,
			AssignedAt:      &now,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "assign secret business number")
	}
	return requireOneRow(result, "assign secret business number")
}

func newOpaqueSecretNumber(randomSource io.Reader) (string, error) {
	var content [16]byte
	if randomSource == nil {
		randomSource = rand.Reader
	}
	if _, err := io.ReadFull(randomSource, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate opaque secret business number")
	}
	return secretNumberPrefix + hex.EncodeToString(content[:]), nil
}

func metadataFromSecret(row *entity.Secret) Metadata {
	if row == nil || row.StatusCode != statusConfigured || row.UpdatedAt == nil || !validMaskedTail(row.MaskedTail) {
		return Metadata{}
	}
	return Metadata{Configured: true, MaskedTail: row.MaskedTail, UpdatedAt: databaseTime(*row.UpdatedAt)}
}

func requireOneRow(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, operation+" result")
	}
	if affected != 1 {
		return gerror.New(operation + " affected an unexpected row count")
	}
	return nil
}

func publicSecretError(err error) error {
	if err == nil {
		return nil
	}
	// Secret persistence errors can embed encrypted wrappers, nonces and SQL.
	// Preserve only this service's stable public codes, never their raw cause.
	for _, code := range []*bizerr.Code{CodeInvalid, CodeAccessDenied, CodeNotConfigured,
		CodeIdempotencyConflict, CodeVersionConflict, CodeConsumerFailed, CodeUnavailable} {
		if bizerr.Is(err, code) {
			return bizerr.NewCode(code)
		}
	}
	return bizerr.NewCode(CodeUnavailable)
}

func publicOptionalSecretError(err error) error {
	if err == nil {
		return nil
	}
	return publicSecretError(err)
}

func firstError(primary error, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
