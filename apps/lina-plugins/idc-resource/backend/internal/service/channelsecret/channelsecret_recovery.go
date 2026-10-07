package channelsecret

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/internal/service/interfacerevision"
)

func (s *serviceImpl) prepareCreate(
	ctx context.Context,
	request AccessRequest,
	bindingNo string,
	actor int,
) (*bindingRow, bool, error) {
	var prepared *bindingRow
	var replayed bool
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := s.requireChannel(ctx, request.ChannelNo, true); err != nil {
			return err
		}
		existing, err := s.loadByNumber(ctx, bindingNo, true)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.ChannelNo != request.ChannelNo || existing.BindingName != request.BindingName {
				return bizerr.NewCode(CodeConflict)
			}
			switch existing.StatusCode {
			case bindingStatusActive:
				prepared, replayed = existing, true
				return nil
			case bindingStatusPreparing, bindingStatusRetryableFailed:
				prepared = existing
				if existing.StatusCode == bindingStatusRetryableFailed {
					_, err = s.database.Exec(ctx, `UPDATE plugin_idc_resource_channel_secret_binding
SET status_code = ?, updated_by = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND id = ? AND version = ?`,
						bindingStatusPreparing, actor, existing.ID, existing.Version)
					if err != nil {
						return err
					}
					prepared, err = s.loadByNumber(ctx, bindingNo, true)
				}
				return err
			default:
				return bizerr.NewCode(CodeConflict)
			}
		}
		active, err := s.loadActive(ctx, request.ChannelNo, request.BindingName, true)
		if err != nil {
			return err
		}
		if active != nil {
			return bizerr.NewCode(CodeConflict)
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel_secret_binding
    (tenant_id,binding_no,channel_no,binding_name,secret_ref,status_code,bound_at,
     version,created_by,updated_by,created_at,updated_at)
VALUES (0,?,?,?,?,?,CURRENT_TIMESTAMP,1,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
			bindingNo, request.ChannelNo, request.BindingName, "pending", bindingStatusPreparing,
			actor, actor)
		if err != nil {
			return err
		}
		prepared, err = s.loadByNumber(ctx, bindingNo, true)
		if err != nil || prepared == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		return nil
	})
	return prepared, replayed, publicError(err)
}

func (s *serviceImpl) completeCreate(
	ctx context.Context,
	request AccessRequest,
	bindingNo string,
	actor int,
	created secretcap.CreateResult,
) (*bindingRow, error) {
	var completed *bindingRow
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := s.requireChannel(ctx, request.ChannelNo, true); err != nil {
			return err
		}
		row, err := s.loadByNumber(ctx, bindingNo, true)
		if err != nil || row == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if row.StatusCode == bindingStatusActive {
			completed = row
			return nil
		}
		if row.StatusCode != bindingStatusPreparing && row.StatusCode != bindingStatusRetryableFailed {
			return bizerr.NewCode(CodeConflict)
		}
		active, err := s.loadActive(ctx, request.ChannelNo, request.BindingName, true)
		if err != nil || active != nil {
			return firstError(err, bizerr.NewCode(CodeConflict))
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_channel_secret_binding
SET secret_ref = ?, status_code = ?, bound_at = ?, updated_by = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND id = ? AND version = ? AND status_code IN (?,?)`,
			created.Handle.OpaqueRef(), bindingStatusActive, created.Metadata.UpdatedAt.UTC(), actor,
			row.ID, row.Version, bindingStatusPreparing, bindingStatusRetryableFailed)
		if err != nil || requireOneRow(result) != nil {
			return firstError(err, bizerr.NewCode(CodeConflict))
		}
		completed, err = s.loadByNumber(ctx, bindingNo, true)
		if err != nil || completed == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if _, revisionErr := interfacerevision.Append(ctx, tx, request.ChannelNo, actor); revisionErr != nil {
			return revisionErr
		}
		return nil
	})
	return completed, publicError(err)
}

func (s *serviceImpl) prepareReplace(
	ctx context.Context,
	request AccessRequest,
	active *bindingRow,
	bindingNo string,
	actor int,
) (*bindingRow, bool, error) {
	var prepared *bindingRow
	var replayed bool
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.requireChannel(ctx, request.ChannelNo, true); err != nil {
			return err
		}
		existing, err := s.loadByNumber(ctx, bindingNo, true)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.ChannelNo != request.ChannelNo || existing.BindingName != request.BindingName {
				return bizerr.NewCode(CodeConflict)
			}
			if existing.StatusCode == bindingStatusActive {
				prepared, replayed = existing, true
				return nil
			}
			if existing.StatusCode != bindingStatusPreparing && existing.StatusCode != bindingStatusRetryableFailed {
				return bizerr.NewCode(CodeConflict)
			}
			prepared = existing
			return nil
		}
		current, err := s.loadActive(ctx, request.ChannelNo, request.BindingName, true)
		if err != nil || current == nil || current.ID != active.ID || current.Version != active.Version {
			return bizerr.NewCode(CodeConflict)
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel_secret_binding
    (tenant_id,binding_no,channel_no,binding_name,secret_ref,status_code,bound_at,
     version,created_by,updated_by,created_at,updated_at)
VALUES (0,?,?,?,?,?, ?,1,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
			bindingNo, request.ChannelNo, request.BindingName, current.SecretRef,
			bindingStatusPreparing, request.ExpectedUpdatedAt.UTC(), actor, actor)
		if err != nil {
			return err
		}
		prepared, err = s.loadByNumber(ctx, bindingNo, true)
		if err != nil || prepared == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		return nil
	})
	return prepared, replayed, publicError(err)
}

func (s *serviceImpl) completeReplace(
	ctx context.Context,
	request AccessRequest,
	previous *bindingRow,
	bindingNo string,
	actor int,
	created secretcap.CreateResult,
) (*bindingRow, error) {
	var completed *bindingRow
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := s.requireChannel(ctx, request.ChannelNo, true); err != nil {
			return err
		}
		pending, err := s.loadByNumber(ctx, bindingNo, true)
		if err != nil || pending == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if pending.StatusCode == bindingStatusActive {
			completed = pending
			return nil
		}
		current, err := s.loadActive(ctx, request.ChannelNo, request.BindingName, true)
		if err != nil || current == nil || current.ID != previous.ID || current.Version != previous.Version {
			return bizerr.NewCode(CodeConflict)
		}
		now := time.Now().UTC()
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_channel_secret_binding
SET status_code = ?, superseded_at = ?, updated_by = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND id = ? AND version = ? AND status_code = ?`,
			bindingStatusSuperseded, now, actor, current.ID, current.Version, bindingStatusActive)
		if err != nil || requireOneRow(result) != nil {
			return firstError(err, bizerr.NewCode(CodeConflict))
		}
		result, err = s.database.Exec(ctx, `UPDATE plugin_idc_resource_channel_secret_binding
SET secret_ref = ?, status_code = ?, bound_at = ?, updated_by = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND id = ? AND version = ? AND status_code IN (?,?)`,
			created.Handle.OpaqueRef(), bindingStatusActive, created.Metadata.UpdatedAt.UTC(), actor, pending.ID, pending.Version,
			bindingStatusPreparing, bindingStatusRetryableFailed)
		if err != nil || requireOneRow(result) != nil {
			return firstError(err, bizerr.NewCode(CodeConflict))
		}
		completed, err = s.loadByNumber(ctx, bindingNo, true)
		if err != nil || completed == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if _, revisionErr := interfacerevision.Append(ctx, tx, request.ChannelNo, actor); revisionErr != nil {
			return revisionErr
		}
		return nil
	})
	return completed, publicError(err)
}

func (s *serviceImpl) markRetryable(ctx context.Context, bindingNo string, actor int, reason string) {
	_ = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.loadByNumber(ctx, bindingNo, true)
		if err != nil || row == nil || row.StatusCode == bindingStatusActive {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_channel_secret_binding
SET status_code = ?, updated_by = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND id = ? AND version = ? AND status_code = ?`,
			bindingStatusRetryableFailed, actor, row.ID, row.Version, bindingStatusPreparing)
		if err != nil || requireOneRow(result) != nil {
			return err
		}
		return nil
	})
}
