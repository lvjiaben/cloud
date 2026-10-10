package channelsecret

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/internal/service/interfacerevision"
)

const (
	bindingStatusPreparing       = "preparing"
	bindingStatusRetryableFailed = "retryable_failed"
	bindingStatusActive          = "active"
	bindingStatusSuperseded      = "superseded"
	bindingStatusRevoked         = "revoked"
)

type bindingRow struct {
	ID           int64      `orm:"id"`
	BindingNo    string     `orm:"binding_no"`
	ChannelNo    string     `orm:"channel_no"`
	BindingName  string     `orm:"binding_name"`
	SecretRef    string     `orm:"secret_ref"`
	StatusCode   string     `orm:"status_code"`
	BoundAt      *time.Time `orm:"bound_at"`
	Version      int64      `orm:"version"`
	UpdatedAt    *time.Time `orm:"updated_at"`
	SupersededAt *time.Time `orm:"superseded_at"`
	RevokedAt    *time.Time `orm:"revoked_at"`
}

func (s *serviceImpl) Create(ctx context.Context, input MutationInput) (*Status, error) {
	defer clear(input.Secret)
	request, actor, err := s.consume(ctx, OperationCreate, input)
	if err != nil {
		return nil, err
	}
	manager, err := s.manager(request.ChannelNo)
	if err != nil || nilDependency(manager) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	bindingNo := bindingNumber(request.ChannelNo, request.BindingName, request.IdempotencyKey)
	row, replayed, err := s.prepareCreate(ctx, request, bindingNo, actor)
	if err != nil {
		return nil, err
	}
	if replayed {
		return s.statusFromRow(ctx, manager, row)
	}
	producer, release := ownedProducer(input.Secret)
	defer release()
	created, err := manager.Create(ctx, "resource-channel-secret:create:"+bindingNo, producer)
	if err != nil || created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
		s.markRetryable(ctx, bindingNo, actor, "secret_create_failed")
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	row, err = s.completeCreate(ctx, request, bindingNo, actor, created)
	if err != nil {
		s.markRetryable(ctx, bindingNo, actor, "binding_commit_failed")
		return nil, err
	}
	return statusFromMetadata(row, created.Metadata), nil
}

func (s *serviceImpl) Replace(ctx context.Context, input MutationInput) (*Status, error) {
	defer clear(input.Secret)
	request, actor, err := s.consume(ctx, OperationReplace, input)
	if err != nil {
		return nil, err
	}
	manager, err := s.manager(request.ChannelNo)
	if err != nil || nilDependency(manager) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	bindingNo := bindingNumber(request.ChannelNo, request.BindingName, request.IdempotencyKey)
	pending, err := s.loadByNumber(ctx, bindingNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if pending != nil && pending.StatusCode == bindingStatusActive {
		return s.statusFromRow(ctx, manager, pending)
	}
	active, err := s.loadActive(ctx, request.ChannelNo, request.BindingName, false)
	if err != nil || active == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	handle, err := secretcap.NewHandle(active.SecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	metadata, err := manager.Metadata(ctx, handle)
	if err != nil || !metadata.Configured {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if pending == nil && !metadata.UpdatedAt.Equal(request.ExpectedUpdatedAt) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	row, replayed, err := s.prepareReplace(ctx, request, active, bindingNo, actor)
	if err != nil {
		return nil, err
	}
	if replayed {
		return s.statusFromRow(ctx, manager, row)
	}
	producer, release := ownedProducer(input.Secret)
	defer release()
	created, err := manager.Create(ctx, "resource-channel-secret:replace:"+bindingNo, producer)
	if err != nil || created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
		s.markRetryable(ctx, bindingNo, actor, "secret_replace_failed")
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	row, err = s.completeReplace(ctx, request, active, bindingNo, actor, created)
	if err != nil {
		s.markRetryable(ctx, bindingNo, actor, "binding_commit_failed")
		return nil, err
	}
	return statusFromMetadata(row, created.Metadata), nil
}

func (s *serviceImpl) Revoke(ctx context.Context, input MutationInput) (*Status, error) {
	request, actor, err := s.consume(ctx, OperationRevoke, input)
	if err != nil {
		return nil, err
	}
	manager, err := s.manager(request.ChannelNo)
	if err != nil || nilDependency(manager) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	active, err := s.loadActive(ctx, request.ChannelNo, request.BindingName, false)
	if err != nil || active == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	handle, err := secretcap.NewHandle(active.SecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	metadata, metadataErr := manager.Metadata(ctx, handle)
	if metadataErr != nil || metadata.UpdatedAt.IsZero() ||
		(metadata.Configured && !metadata.UpdatedAt.Equal(request.ExpectedUpdatedAt)) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if metadata.Configured {
		metadata, err = manager.Revoke(ctx, handle, request.ExpectedUpdatedAt)
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		current, loadErr := s.loadActive(ctx, request.ChannelNo, request.BindingName, true)
		if loadErr != nil || current == nil || current.ID != active.ID || current.Version != active.Version {
			return bizerr.NewCode(CodeConflict)
		}
		now := time.Now().UTC()
		result, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_resource_channel_secret_binding
SET status_code = ?, revoked_at = ?, updated_by = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND id = ? AND version = ? AND status_code = ?`,
			bindingStatusRevoked, now, actor, current.ID, current.Version, bindingStatusActive)
		if updateErr != nil || requireOneRow(result) != nil {
			return firstError(updateErr, bizerr.NewCode(CodeConflict))
		}
		if _, revisionErr := interfacerevision.Append(ctx, tx, request.ChannelNo, actor); revisionErr != nil {
			return revisionErr
		}
		after := *current
		after.StatusCode = bindingStatusRevoked
		after.Version++
		return s.appendBindingAudit(ctx, OperationRevoke, current, &after)
	})
	if err != nil {
		return nil, publicError(err)
	}
	return &Status{ChannelNo: request.ChannelNo, BindingName: request.BindingName,
		Configured: false, UpdatedAt: metadata.UpdatedAt.UTC()}, nil
}

func (s *serviceImpl) Rotate(ctx context.Context, input MutationInput) (secretcap.RotationResult, error) {
	request, _, err := s.consume(ctx, OperationRotate, input)
	if err != nil {
		return secretcap.RotationResult{}, err
	}
	if err = s.requireChannel(ctx, request.ChannelNo, false); err != nil {
		return secretcap.RotationResult{}, err
	}
	manager, err := s.manager(request.ChannelNo)
	if err != nil || nilDependency(manager) {
		return secretcap.RotationResult{}, bizerr.NewCode(CodeUnavailable)
	}
	var result secretcap.RotationResult
	// The secret owner joins this ambient transaction, so wrapping progress and
	// the caller's secret-free audit evidence commit or roll back together.
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rotateErr error
		result, rotateErr = manager.Rotate(ctx, request.RotateLimit)
		if rotateErr != nil || result.Rewrapped < 0 || result.Rewrapped > request.RotateLimit {
			return bizerr.NewCode(CodeUnavailable)
		}
		return s.appendRotationAudit(ctx, request.ChannelNo, input.AccessContext, result)
	})
	if err != nil {
		return secretcap.RotationResult{}, publicError(err)
	}
	return result, nil
}

func ownedProducer(value []byte) (secretcap.PlaintextProducer, func()) {
	owned := append([]byte(nil), value...)
	return func(consume secretcap.Consumer) error {
		if consume == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		view := append([]byte(nil), owned...)
		defer clear(view)
		return consume(view)
	}, func() { clear(owned) }
}

func statusFromMetadata(row *bindingRow, metadata secretcap.Metadata) *Status {
	return &Status{
		ChannelNo: row.ChannelNo, BindingName: row.BindingName,
		Configured: metadata.Configured, MaskedTail: metadata.MaskedTail,
		UpdatedAt: metadata.UpdatedAt.UTC(),
	}
}

func (s *serviceImpl) statusFromRow(
	ctx context.Context,
	manager secretcap.Manager,
	row *bindingRow,
) (*Status, error) {
	if row == nil || row.StatusCode != bindingStatusActive {
		return nil, bizerr.NewCode(CodeConflict)
	}
	handle, err := secretcap.NewHandle(row.SecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	metadata, err := manager.Metadata(ctx, handle)
	if err != nil || !metadata.Configured {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return statusFromMetadata(row, metadata), nil
}

func (s *serviceImpl) requireChannel(ctx context.Context, channelNo string, lock bool) error {
	query := `SELECT channel_no FROM plugin_idc_resource_channel
WHERE tenant_id = 0 AND channel_no = ? AND deleted_at IS NULL`
	if lock {
		query += ` FOR UPDATE`
	}
	var row struct {
		ChannelNo string `orm:"channel_no"`
	}
	if err := s.database.GetScan(ctx, &row, query, channelNo); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return bizerr.NewCode(CodeUnavailable)
	}
	if row.ChannelNo == "" {
		return bizerr.NewCode(CodeNotAccessible)
	}
	return nil
}

func (s *serviceImpl) loadActive(
	ctx context.Context,
	channelNo string,
	bindingName string,
	lock bool,
) (*bindingRow, error) {
	query := `SELECT id,binding_no,channel_no,binding_name,secret_ref,status_code,bound_at,
       version,updated_at,superseded_at,revoked_at
FROM plugin_idc_resource_channel_secret_binding
WHERE tenant_id = 0 AND channel_no = ? AND binding_name = ? AND status_code = 'active'
LIMIT 1`
	if lock {
		query += ` FOR UPDATE`
	}
	var row bindingRow
	if err := s.database.GetScan(ctx, &row, query, channelNo, bindingName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (s *serviceImpl) loadByNumber(ctx context.Context, bindingNo string, lock bool) (*bindingRow, error) {
	query := `SELECT id,binding_no,channel_no,binding_name,secret_ref,status_code,bound_at,
       version,updated_at,superseded_at,revoked_at
FROM plugin_idc_resource_channel_secret_binding
WHERE tenant_id = 0 AND binding_no = ? LIMIT 1`
	if lock {
		query += ` FOR UPDATE`
	}
	var row bindingRow
	if err := s.database.GetScan(ctx, &row, query, bindingNo); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func requireOneRow(result sql.Result) error {
	if result == nil {
		return bizerr.NewCode(CodeConflict)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}

func publicError(err error) error {
	if err == nil {
		return nil
	}
	for _, code := range []*bizerr.Code{
		CodeInvalid, CodeNotAccessible, CodeRecentAuthenticationRequired,
		CodeConflict, CodeUnavailable,
	} {
		if bizerr.Is(err, code) {
			return bizerr.NewCode(code)
		}
	}
	return bizerr.NewCode(CodeUnavailable)
}
