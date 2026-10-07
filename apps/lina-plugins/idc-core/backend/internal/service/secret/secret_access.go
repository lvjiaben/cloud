// This file implements owner-scoped callback access without returning plaintext.

package secret

import (
	"context"
	"encoding/hex"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// scopedAccessor binds the caller scope at composition time and offers no lookup or listing surface.
type scopedAccessor struct {
	service *serviceImpl
	scope   Scope
}

// Ensure scopedAccessor implements the public callback contract.
var _ secretcap.Accessor = (*scopedAccessor)(nil)

// Accessor validates and binds one least-privilege scope. Authorization still
// runs for every callback access so revoked caller permissions fail closed.
func (s *serviceImpl) Accessor(scope Scope) (secretcap.Accessor, error) {
	normalized, err := normalizeScope(scope)
	if err != nil {
		return nil, err
	}
	return &scopedAccessor{service: s, scope: normalized}, nil
}

// Access decrypts into one short-lived buffer, invokes consume at most once,
// clears the buffer after return or panic, and collapses callback errors.
func (a *scopedAccessor) Access(
	ctx context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if a == nil || a.service == nil || secretcap.ValidateAccess(request, consume) != nil ||
		!validSecretRef(request.Handle.OpaqueRef()) {
		return bizerr.NewCode(CodeInvalid)
	}
	if err := a.service.authorize(
		ctx,
		a.scope,
		ActionAccess,
		request.Handle,
		request.Purpose,
	); err != nil {
		return err
	}
	secretRef := request.Handle.OpaqueRef()
	row, version, wrappers, err := a.service.loadCurrentEnvelope(ctx, a.scope, secretRef)
	if err != nil {
		return publicSecretError(err)
	}
	keyIDs := make([]string, 0, len(wrappers))
	for _, wrapper := range wrappers {
		keyIDs = append(keyIDs, wrapper.masterKeyID)
	}
	keys, err := a.service.resolveKeys(ctx, keyIDs)
	if err != nil {
		return err
	}
	defer zeroKeyMap(keys)
	dek, err := unwrapDEK(keys, a.scope, secretRef, row.CurrentVersion, wrappers)
	if err != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	defer zeroBytes(dek)
	plaintext, err := openSealed(
		dek,
		mustDecodePersistedHex(version.PayloadNonce),
		mustDecodePersistedHex(version.Ciphertext),
		payloadAAD(a.scope, secretRef, row.CurrentVersion),
	)
	if err != nil {
		zeroBytes(plaintext)
		return bizerr.NewCode(CodeUnavailable)
	}
	defer zeroBytes(plaintext)

	// A second scoped state check narrows revoke/update races. Access already in
	// flight at this authorization point may complete; every later call observes
	// revocation or the newer payload version.
	current, err := loadScopedSecret(ctx, a.scope, secretRef, false)
	if err != nil {
		return publicSecretError(err)
	}
	if current == nil || current.StatusCode != statusConfigured {
		return bizerr.NewCode(CodeNotConfigured)
	}
	if current.CurrentVersion != row.CurrentVersion {
		return bizerr.NewCode(CodeVersionConflict)
	}
	return invokeConsumer(consume, plaintext)
}

// invokeConsumer collapses both ordinary callback errors and panics without
// formatting the callback-owned value, which might itself contain plaintext.
func invokeConsumer(consume secretcap.Consumer, plaintext []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = bizerr.NewCode(CodeConsumerFailed)
		}
	}()
	if consumeErr := consume(plaintext); consumeErr != nil {
		return bizerr.NewCode(CodeConsumerFailed)
	}
	return nil
}

func (s *serviceImpl) loadCurrentEnvelope(
	ctx context.Context,
	scope Scope,
	secretRef string,
) (*entity.Secret, *entity.SecretVersion, []keyWrapValue, error) {
	row, err := loadScopedSecret(ctx, scope, secretRef, false)
	if err != nil {
		return nil, nil, nil, err
	}
	if row == nil || row.StatusCode != statusConfigured {
		return nil, nil, nil, bizerr.NewCode(CodeNotConfigured)
	}
	var highest struct {
		Version int `orm:"version"`
	}
	if err = dao.SecretVersion.Ctx(ctx).
		Fields(dao.SecretVersion.Columns().Version).
		Where(do.SecretVersion{TenantId: scope.TenantID, SecretNo: secretRef}).
		OrderDesc(dao.SecretVersion.Columns().Version).
		Limit(1).
		Scan(&highest); err != nil {
		return nil, nil, nil, gerror.Wrap(err, "load highest immutable secret version")
	}
	if highest.Version <= 0 || highest.Version != row.CurrentVersion {
		return nil, nil, nil, gerror.New("secret aggregate version watermark is inconsistent")
	}
	var version *entity.SecretVersion
	if err = dao.SecretVersion.Ctx(ctx).Where(do.SecretVersion{
		TenantId: scope.TenantID,
		SecretNo: secretRef,
		Version:  row.CurrentVersion,
	}).Scan(&version); err != nil {
		return nil, nil, nil, gerror.Wrap(err, "load current encrypted secret payload")
	}
	if !validPersistedVersion(version, row.CurrentVersion) {
		return nil, nil, nil, gerror.New("current encrypted secret payload is invalid")
	}
	wrappers, err := s.loadVersionWrappers(ctx, scope.TenantID, secretRef, row.CurrentVersion)
	if err != nil {
		return nil, nil, nil, err
	}
	return row, version, wrappers, nil
}

// loadVersionWrappers performs one bounded query for current access and
// historical idempotency verification alike. Exceeding the bound fails closed:
// an operator must run an authenticated wrapper-compaction workflow before
// another key generation can be declared retireable.
func (s *serviceImpl) loadVersionWrappers(
	ctx context.Context,
	tenantID int,
	secretRef string,
	version int,
) ([]keyWrapValue, error) {
	var wrapRows []entity.SecretKeyWrap
	if err := dao.SecretKeyWrap.Ctx(ctx).
		Where(do.SecretKeyWrap{
			TenantId:      tenantID,
			SecretNo:      secretRef,
			SecretVersion: version,
		}).
		OrderDesc(dao.SecretKeyWrap.Columns().Id).
		Limit(s.config.MaximumKeyWraps + 1).
		Scan(&wrapRows); err != nil {
		return nil, gerror.Wrap(err, "load encrypted secret key wrappers")
	}
	if len(wrapRows) == 0 || len(wrapRows) > s.config.MaximumKeyWraps {
		return nil, gerror.New("encrypted secret key wrapper count is invalid")
	}
	wrappers := make([]keyWrapValue, 0, len(wrapRows))
	for _, wrap := range wrapRows {
		if !validPersistedWrap(&wrap, version) {
			return nil, gerror.New("encrypted secret key wrapper is invalid")
		}
		wrappers = append(wrappers, keyWrapValue{
			masterKeyID: wrap.MasterKeyId,
			nonce:       mustDecodePersistedHex(wrap.WrapNonce),
			ciphertext:  mustDecodePersistedHex(wrap.WrappedDek),
		})
	}
	return wrappers, nil
}

func validPersistedVersion(version *entity.SecretVersion, expectedVersion int) bool {
	return version != nil &&
		version.Version == expectedVersion &&
		version.EnvelopeVersion == envelopeVersion &&
		version.CipherAlgorithm == cipherAlgorithm &&
		validPersistedHex(version.PayloadNonce, gcmNonceSize, gcmNonceSize) &&
		validPersistedHex(version.Ciphertext, 16, maximumPlaintextSize+16) &&
		validPersistedHex(version.RequestFingerprint, sha256DigestSize, sha256DigestSize) &&
		validPersistedHex(version.IdempotencyKeyDigest, sha256DigestSize, sha256DigestSize) &&
		validPersistedRequestFence(version)
}

func validPersistedRequestFence(version *entity.SecretVersion) bool {
	if version == nil {
		return false
	}
	switch version.RequestOperation {
	case requestOperationCreate, requestOperationGenerate256:
		return version.Version == 1 && version.RequestParentVersion == 0 &&
			version.RequestExpectedUpdatedAt == nil
	case requestOperationUpdate:
		return version.RequestParentVersion > 0 &&
			version.Version == version.RequestParentVersion+1 &&
			version.RequestExpectedUpdatedAt != nil
	default:
		return false
	}
}

func validPersistedWrap(wrap *entity.SecretKeyWrap, expectedVersion int) bool {
	return wrap != nil &&
		wrap.SecretVersion == expectedVersion &&
		wrap.EnvelopeVersion == envelopeVersion &&
		wrap.WrapAlgorithm == cipherAlgorithm &&
		validKeyID(wrap.MasterKeyId) &&
		validPersistedHex(wrap.WrapNonce, gcmNonceSize, gcmNonceSize) &&
		validPersistedHex(wrap.WrappedDek, dekSize+16, dekSize+16)
}

func validPersistedHex(value string, minimumBytes int, maximumBytes int) bool {
	if len(value)%2 != 0 || len(value) < minimumBytes*2 || len(value) > maximumBytes*2 {
		return false
	}
	for _, character := range []byte(value) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func mustDecodePersistedHex(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil
	}
	return decoded
}
