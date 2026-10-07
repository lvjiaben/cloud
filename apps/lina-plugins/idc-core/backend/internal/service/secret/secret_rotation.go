// This file implements database-fenced, additive master-key rotation.

package secret

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

var (
	errRotationLeaseMoved  = errors.New("secret rotation lease moved")
	errRotationMaintenance = errors.New("secret wrapper history requires authenticated operator compaction")
)

// rotationLease is a database-owned verification generation. It contains no
// secret handle and is valid only while all fields still match the locked row.
type rotationLease struct {
	stateID                int64
	epoch                  int64
	activeKeyID            string
	writeFence             int64
	verificationCursorID   int64
	verificationWriteFence int64
}

type preparedRotationBatch struct {
	versions []entity.SecretVersion
	wrappers map[string][]keyWrapValue
	keys     map[string][]byte
	hasMore  bool
}

// Rotate authenticates one bounded payload batch with the actual active key.
// Remaining is false only after one uninterrupted database-fenced pass has
// authenticated every active wrapper and its payload. Historical wrappers are
// append-only; reaching MaximumKeyWraps fails closed until an explicit,
// authenticated operator compaction workflow is available.
func (s *serviceImpl) Rotate(ctx context.Context, input RotateInput) (RotateResult, error) {
	scope, err := normalizeScope(input.Scope)
	if err != nil || input.Limit < 0 || input.Limit > s.config.MaximumRotationBatch {
		return RotateResult{}, bizerr.NewCode(CodeInvalid)
	}
	if input.Limit == 0 {
		input.Limit = s.config.MaximumRotationBatch
	}
	if err = s.authorize(ctx, scope, ActionRotate, secretcap.Handle{}, ""); err != nil {
		return RotateResult{}, err
	}

	active, lease, err := s.beginRotation(ctx, scope)
	if err != nil {
		return RotateResult{}, publicSecretError(err)
	}
	defer zeroBytes(active.Key)
	prepared, err := s.prepareRotationBatch(ctx, scope, lease, active.ID, input.Limit)
	if err != nil {
		return RotateResult{}, publicSecretError(err)
	}
	defer zeroKeyMap(prepared.keys)

	result, err := s.commitRotationBatch(ctx, scope, active, lease, prepared, input.Limit)
	if errors.Is(err, errRotationLeaseMoved) {
		return RotateResult{Remaining: true}, nil
	}
	if err != nil {
		return RotateResult{}, publicSecretError(err)
	}
	return result, nil
}

// activeWriteKey is called from the create/update transaction. The shared row
// is locked before Active is read a second time, so a writer that captured an
// old key can never commit after a newer generation has reported completion.
func (s *serviceImpl) activeWriteKey(ctx context.Context, scope Scope) (KeyMaterial, error) {
	candidate, err := s.activeKey(ctx)
	if err != nil {
		return KeyMaterial{}, err
	}
	defer zeroBytes(candidate.Key)
	if err = initializeRotationState(ctx, scope, candidate.ID); err != nil {
		return KeyMaterial{}, err
	}
	state, err := loadRotationState(ctx, scope, true)
	if err != nil || !validRotationState(state) {
		return KeyMaterial{}, firstError(err, gerror.New("secret rotation state is invalid"))
	}
	confirmed, err := s.activeKey(ctx)
	if err != nil {
		return KeyMaterial{}, err
	}
	if candidate.ID == confirmed.ID && !constantTimeKeyEqual(candidate.Key, confirmed.Key) {
		zeroBytes(confirmed.Key)
		return KeyMaterial{}, gerror.New("secret key provider reused one key id with different material")
	}
	if state.ActiveKeyId != confirmed.ID {
		if err = promoteRotationState(ctx, state, confirmed.ID); err != nil {
			zeroBytes(confirmed.Key)
			return KeyMaterial{}, err
		}
	}
	return confirmed, nil
}

func (s *serviceImpl) beginRotation(
	ctx context.Context,
	scope Scope,
) (KeyMaterial, rotationLease, error) {
	candidate, err := s.activeKey(ctx)
	if err != nil {
		return KeyMaterial{}, rotationLease{}, err
	}
	defer zeroBytes(candidate.Key)
	var active KeyMaterial
	var lease rotationLease
	err = dao.SecretRotationState.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if initializeErr := initializeRotationState(ctx, scope, candidate.ID); initializeErr != nil {
			return initializeErr
		}
		state, loadErr := loadRotationState(ctx, scope, true)
		if loadErr != nil || !validRotationState(state) {
			return firstError(loadErr, gerror.New("secret rotation state is invalid"))
		}
		confirmed, activeErr := s.activeKey(ctx)
		if activeErr != nil {
			return activeErr
		}
		if candidate.ID == confirmed.ID && !constantTimeKeyEqual(candidate.Key, confirmed.Key) {
			zeroBytes(confirmed.Key)
			return gerror.New("secret key provider reused one key id with different material")
		}
		needsNewEpoch := state.ActiveKeyId != confirmed.ID ||
			state.StatusCode == rotationStatusStable ||
			state.VerificationWriteFence != state.WriteFence
		if needsNewEpoch {
			if transitionErr := promoteRotationState(ctx, state, confirmed.ID); transitionErr != nil {
				zeroBytes(confirmed.Key)
				return transitionErr
			}
		}
		active = confirmed
		lease = leaseFromRotationState(state)
		return nil
	})
	if err != nil {
		zeroBytes(active.Key)
		return KeyMaterial{}, rotationLease{}, err
	}
	return active, lease, nil
}

func initializeRotationState(ctx context.Context, scope Scope, activeKeyID string) error {
	result, err := dao.SecretRotationState.Ctx(ctx).Data(do.SecretRotationState{
		TenantId:               scope.TenantID,
		OwnerPluginId:          scope.OwnerPluginID,
		CallerScope:            scope.CallerScope,
		Epoch:                  1,
		ActiveKeyId:            activeKeyID,
		StatusCode:             rotationStatusRotating,
		WriteFence:             0,
		VerificationCursorId:   0,
		VerificationWriteFence: 0,
	}).InsertIgnore()
	if err != nil {
		return gerror.Wrap(err, "initialize secret rotation state")
	}
	if _, err = result.RowsAffected(); err != nil {
		return gerror.Wrap(err, "read secret rotation state insert result")
	}
	return nil
}

func loadRotationState(
	ctx context.Context,
	scope Scope,
	lock bool,
) (*entity.SecretRotationState, error) {
	model := dao.SecretRotationState.Ctx(ctx).Where(do.SecretRotationState{
		TenantId:      scope.TenantID,
		OwnerPluginId: scope.OwnerPluginID,
		CallerScope:   scope.CallerScope,
	})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.SecretRotationState
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load secret rotation state")
	}
	return row, nil
}

func promoteRotationState(
	ctx context.Context,
	state *entity.SecretRotationState,
	activeKeyID string,
) error {
	if !validRotationState(state) || !validKeyID(activeKeyID) || state.Epoch == math.MaxInt64 {
		return gerror.New("secret rotation state cannot advance")
	}
	nextEpoch := state.Epoch + 1
	result, err := dao.SecretRotationState.Ctx(ctx).
		Where(do.SecretRotationState{Id: state.Id, Epoch: state.Epoch}).
		Data(do.SecretRotationState{
			Epoch:                  nextEpoch,
			ActiveKeyId:            activeKeyID,
			StatusCode:             rotationStatusRotating,
			VerificationCursorId:   0,
			VerificationWriteFence: state.WriteFence,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "advance secret rotation generation")
	}
	if err = requireOneRow(result, "advance secret rotation generation"); err != nil {
		return errRotationLeaseMoved
	}
	state.Epoch = nextEpoch
	state.ActiveKeyId = activeKeyID
	state.StatusCode = rotationStatusRotating
	state.VerificationCursorId = 0
	state.VerificationWriteFence = state.WriteFence
	return nil
}

func validRotationState(state *entity.SecretRotationState) bool {
	return state != nil && state.Id > 0 && state.Epoch > 0 &&
		validKeyID(state.ActiveKeyId) &&
		(state.StatusCode == rotationStatusRotating || state.StatusCode == rotationStatusStable) &&
		state.WriteFence >= 0 && state.VerificationCursorId >= 0 &&
		state.VerificationWriteFence >= 0 && state.VerificationWriteFence <= state.WriteFence
}

func leaseFromRotationState(state *entity.SecretRotationState) rotationLease {
	return rotationLease{
		stateID: state.Id, epoch: state.Epoch, activeKeyID: state.ActiveKeyId,
		writeFence: state.WriteFence, verificationCursorID: state.VerificationCursorId,
		verificationWriteFence: state.VerificationWriteFence,
	}
}

func (s *serviceImpl) prepareRotationBatch(
	ctx context.Context,
	scope Scope,
	lease rotationLease,
	activeKeyID string,
	limit int,
) (preparedRotationBatch, error) {
	versions, err := loadRotationVersions(ctx, scope, lease.verificationCursorID, limit+1)
	if err != nil {
		return preparedRotationBatch{}, err
	}
	prepared := preparedRotationBatch{hasMore: len(versions) > limit}
	if prepared.hasMore {
		versions = versions[:limit]
	}
	prepared.versions = versions
	if len(versions) == 0 {
		return prepared, nil
	}
	prepared.wrappers, err = s.loadRotationWrappers(ctx, scope, versions)
	if err != nil {
		return preparedRotationBatch{}, err
	}
	keyIDs := make([]string, 0)
	for _, version := range versions {
		wrappers := prepared.wrappers[rotationVersionKey(version)]
		if len(wrappers) == 0 || len(wrappers) > s.config.MaximumKeyWraps {
			return preparedRotationBatch{}, errRotationMaintenance
		}
		if hasWrapperForKey(wrappers, activeKeyID) {
			continue
		}
		if len(wrappers) >= s.config.MaximumKeyWraps {
			return preparedRotationBatch{}, errRotationMaintenance
		}
		for _, wrapper := range wrappers {
			keyIDs = append(keyIDs, wrapper.masterKeyID)
		}
	}
	if len(keyIDs) > 0 {
		prepared.keys, err = s.resolveKeys(ctx, keyIDs)
		if err != nil {
			return preparedRotationBatch{}, err
		}
	}
	return prepared, nil
}

func (s *serviceImpl) commitRotationBatch(
	ctx context.Context,
	scope Scope,
	active KeyMaterial,
	lease rotationLease,
	prepared preparedRotationBatch,
	limit int,
) (RotateResult, error) {
	var result RotateResult
	err := dao.SecretRotationState.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		state, err := loadRotationState(ctx, scope, true)
		if err != nil {
			return err
		}
		if !rotationLeaseMatches(state, lease) {
			return errRotationLeaseMoved
		}
		confirmed, err := s.activeKey(ctx)
		if err != nil {
			return err
		}
		defer zeroBytes(confirmed.Key)
		if confirmed.ID != active.ID {
			return errRotationLeaseMoved
		}
		if !constantTimeKeyEqual(confirmed.Key, active.Key) {
			return gerror.New("secret key provider changed material for the active key id")
		}

		versions, err := loadRotationVersions(ctx, scope, lease.verificationCursorID, limit+1)
		if err != nil {
			return err
		}
		hasMore := len(versions) > limit
		if hasMore {
			versions = versions[:limit]
		}
		if hasMore != prepared.hasMore || !sameRotationVersions(versions, prepared.versions) {
			return errRotationLeaseMoved
		}
		wrappers, err := s.loadRotationWrappers(ctx, scope, versions)
		if err != nil {
			return err
		}
		rows, err := s.buildMissingActiveWrappers(scope, active, versions, wrappers, prepared.keys)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			insertResult, insertErr := dao.SecretKeyWrap.Ctx(ctx).Data(rows).InsertIgnore()
			if insertErr != nil {
				return gerror.Wrap(insertErr, "append rotated secret key wrappers")
			}
			affected, affectedErr := insertResult.RowsAffected()
			if affectedErr != nil {
				return gerror.Wrap(affectedErr, "read rotated wrapper insert result")
			}
			result.Rewrapped = int(affected)
		}

		// Reload and authenticate the persisted winner. InsertIgnore convergence is
		// never treated as proof that a same-key-id wrapper is valid.
		persistedWrappers, err := s.loadRotationWrappers(ctx, scope, versions)
		if err != nil {
			return err
		}
		for index := range versions {
			version := &versions[index]
			group := persistedWrappers[rotationVersionKey(*version)]
			if len(group) == 0 || len(group) > s.config.MaximumKeyWraps {
				return errRotationMaintenance
			}
			if err = authenticateActiveEnvelope(active, scope, version, group); err != nil {
				return err
			}
		}

		cursor := lease.verificationCursorID
		if len(versions) > 0 {
			cursor = versions[len(versions)-1].Id
		}
		status := rotationStatusStable
		result.Remaining = false
		if hasMore {
			status = rotationStatusRotating
			result.Remaining = true
		}
		updateResult, updateErr := dao.SecretRotationState.Ctx(ctx).
			Where(do.SecretRotationState{
				Id:                     lease.stateID,
				Epoch:                  lease.epoch,
				ActiveKeyId:            lease.activeKeyID,
				StatusCode:             rotationStatusRotating,
				WriteFence:             lease.writeFence,
				VerificationCursorId:   lease.verificationCursorID,
				VerificationWriteFence: lease.verificationWriteFence,
			}).
			Data(do.SecretRotationState{
				StatusCode:           status,
				VerificationCursorId: cursor,
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "checkpoint secret rotation verification")
		}
		if updateErr = requireOneRow(updateResult, "checkpoint secret rotation verification"); updateErr != nil {
			return errRotationLeaseMoved
		}
		return nil
	})
	return result, err
}

func rotationLeaseMatches(state *entity.SecretRotationState, lease rotationLease) bool {
	return validRotationState(state) && state.Id == lease.stateID &&
		state.Epoch == lease.epoch && state.ActiveKeyId == lease.activeKeyID &&
		state.StatusCode == rotationStatusRotating && state.WriteFence == lease.writeFence &&
		state.VerificationCursorId == lease.verificationCursorID &&
		state.VerificationWriteFence == lease.verificationWriteFence
}

func loadRotationVersions(
	ctx context.Context,
	scope Scope,
	afterID int64,
	limit int,
) ([]entity.SecretVersion, error) {
	var versions []entity.SecretVersion
	err := dao.SecretVersion.Ctx(ctx).
		As("payload").
		InnerJoin(
			dao.Secret.Table(),
			"aggregate",
			"aggregate.tenant_id = payload.tenant_id AND aggregate.secret_no = payload.secret_no",
		).
		Fields("payload.*").
		Where("aggregate.tenant_id", scope.TenantID).
		Where("aggregate.owner_plugin_id", scope.OwnerPluginID).
		Where("aggregate.caller_scope", scope.CallerScope).
		Where("payload.tenant_id", scope.TenantID).
		WhereGT("payload.id", afterID).
		OrderAsc("payload.id").
		Limit(limit).
		Scan(&versions)
	if err != nil {
		return nil, gerror.Wrap(err, "load bounded secret rotation versions")
	}
	for index := range versions {
		if versions[index].Id <= afterID || !validSecretRef(versions[index].SecretNo) ||
			!validPersistedVersion(&versions[index], versions[index].Version) {
			return nil, gerror.New("secret rotation version is invalid")
		}
	}
	return versions, nil
}

func sameRotationVersions(left []entity.SecretVersion, right []entity.SecretVersion) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Id != right[index].Id || left[index].SecretNo != right[index].SecretNo ||
			left[index].Version != right[index].Version {
			return false
		}
	}
	return true
}

func (s *serviceImpl) buildMissingActiveWrappers(
	scope Scope,
	active KeyMaterial,
	versions []entity.SecretVersion,
	groups map[string][]keyWrapValue,
	keys map[string][]byte,
) ([]do.SecretKeyWrap, error) {
	rows := make([]do.SecretKeyWrap, 0, len(versions))
	for index := range versions {
		version := &versions[index]
		wrappers := groups[rotationVersionKey(*version)]
		if len(wrappers) == 0 || len(wrappers) > s.config.MaximumKeyWraps {
			return nil, errRotationMaintenance
		}
		if hasWrapperForKey(wrappers, active.ID) {
			if err := authenticateActiveEnvelope(active, scope, version, wrappers); err != nil {
				return nil, err
			}
			continue
		}
		if len(wrappers) >= s.config.MaximumKeyWraps {
			return nil, errRotationMaintenance
		}
		dek, err := unwrapDEK(keys, scope, version.SecretNo, version.Version, wrappers)
		if err != nil {
			return nil, gerror.New("no historical wrapper authenticated during secret rotation")
		}
		if err = authenticatePayload(dek, scope, version); err != nil {
			zeroBytes(dek)
			return nil, err
		}
		nonce, wrappedDEK, err := sealRandom(
			active.Key,
			dek,
			keyWrapAAD(scope, version.SecretNo, version.Version, active.ID),
			s.random,
		)
		zeroBytes(dek)
		if err != nil {
			return nil, gerror.New("create active secret key wrapper")
		}
		rows = append(rows, do.SecretKeyWrap{
			TenantId:        scope.TenantID,
			SecretNo:        version.SecretNo,
			SecretVersion:   version.Version,
			EnvelopeVersion: envelopeVersion,
			MasterKeyId:     active.ID,
			WrapAlgorithm:   cipherAlgorithm,
			WrapNonce:       hex.EncodeToString(nonce),
			WrappedDek:      hex.EncodeToString(wrappedDEK),
		})
	}
	return rows, nil
}

func authenticateActiveEnvelope(
	active KeyMaterial,
	scope Scope,
	version *entity.SecretVersion,
	wrappers []keyWrapValue,
) error {
	var activeWrapper *keyWrapValue
	for index := range wrappers {
		if wrappers[index].masterKeyID == active.ID {
			if activeWrapper != nil {
				return gerror.New("multiple active wrappers exist for one secret version")
			}
			activeWrapper = &wrappers[index]
		}
	}
	if activeWrapper == nil {
		return gerror.New("active wrapper is missing after secret rotation")
	}
	dek, err := openSealed(
		active.Key,
		activeWrapper.nonce,
		activeWrapper.ciphertext,
		keyWrapAAD(scope, version.SecretNo, version.Version, active.ID),
	)
	if err != nil || len(dek) != dekSize {
		zeroBytes(dek)
		return gerror.New("active secret key wrapper failed authentication")
	}
	defer zeroBytes(dek)
	return authenticatePayload(dek, scope, version)
}

func authenticatePayload(dek []byte, scope Scope, version *entity.SecretVersion) error {
	plaintext, err := openSealed(
		dek,
		mustDecodePersistedHex(version.PayloadNonce),
		mustDecodePersistedHex(version.Ciphertext),
		payloadAAD(scope, version.SecretNo, version.Version),
	)
	zeroBytes(plaintext)
	if err != nil {
		return gerror.New("secret payload failed authentication during rotation")
	}
	return nil
}

func hasWrapperForKey(wrappers []keyWrapValue, keyID string) bool {
	for _, wrapper := range wrappers {
		if wrapper.masterKeyID == keyID {
			return true
		}
	}
	return false
}

func (s *serviceImpl) loadRotationWrappers(
	ctx context.Context,
	scope Scope,
	versions []entity.SecretVersion,
) (map[string][]keyWrapValue, error) {
	groups := make(map[string][]keyWrapValue, len(versions))
	if len(versions) == 0 {
		return groups, nil
	}
	conditions := make([]string, 0, len(versions))
	arguments := make([]any, 0, len(versions)*2)
	for _, version := range versions {
		conditions = append(conditions, "(secret_no = ? AND secret_version = ?)")
		arguments = append(arguments, version.SecretNo, version.Version)
	}
	var rows []entity.SecretKeyWrap
	err := dao.SecretKeyWrap.Ctx(ctx).
		Where(do.SecretKeyWrap{TenantId: scope.TenantID}).
		Where("("+strings.Join(conditions, " OR ")+")", arguments...).
		OrderAsc(dao.SecretKeyWrap.Columns().SecretNo).
		OrderAsc(dao.SecretKeyWrap.Columns().SecretVersion).
		OrderDesc(dao.SecretKeyWrap.Columns().Id).
		Limit(len(versions)*(s.config.MaximumKeyWraps+1) + 1).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load bounded secret rotation wrappers")
	}
	for index := range rows {
		row := &rows[index]
		if !validPersistedWrap(row, row.SecretVersion) {
			return nil, gerror.New("secret rotation wrapper is invalid")
		}
		key := rotationKey(row.SecretNo, row.SecretVersion)
		groups[key] = append(groups[key], keyWrapValue{
			masterKeyID: row.MasterKeyId,
			nonce:       mustDecodePersistedHex(row.WrapNonce),
			ciphertext:  mustDecodePersistedHex(row.WrappedDek),
		})
		if len(groups[key]) > s.config.MaximumKeyWraps {
			return nil, errRotationMaintenance
		}
	}
	return groups, nil
}

func rotationVersionKey(version entity.SecretVersion) string {
	return rotationKey(version.SecretNo, version.Version)
}

func rotationKey(secretRef string, version int) string {
	return secretRef + "\x00" + strconv.Itoa(version)
}

func constantTimeKeyEqual(left []byte, right []byte) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare(left, right) == 1
}
