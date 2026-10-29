// This file persists and recovers notification-channel credential changes so
// a process crash cannot split secretcap state from the notification database.

package notification

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const (
	channelIntentTable = "plugin_idc_notify_channel_intent"

	channelIntentConfigureCreate  = "configure_create"
	channelIntentConfigureReplace = "configure_replace"
	channelIntentClear            = "clear"

	channelIntentPrepared      = "prepared"
	channelIntentSecretApplied = "secret_applied"
	channelIntentCompleted     = "completed"
)

type channelIntentRow struct {
	OperationNo             string     `orm:"operation_no"`
	ChannelCode             string     `orm:"channel_code"`
	OperationCode           string     `orm:"operation_code"`
	StatusCode              string     `orm:"status_code"`
	ActorUserID             int        `orm:"actor_user_id"`
	BaseStatusCode          string     `orm:"base_status_code"`
	BaseVersion             int64      `orm:"base_version"`
	RequestShapeDigest      string     `orm:"request_shape_digest"`
	DesiredProviderCode     *string    `orm:"desired_provider_code"`
	DesiredNonSecretConfig  string     `orm:"desired_non_secret_configuration"`
	SecretOperationKey      string     `orm:"secret_operation_key"`
	CredentialSecretRef     *string    `orm:"credential_secret_ref"`
	SecretExpectedUpdatedAt *time.Time `orm:"secret_expected_updated_at"`
	SecretResultUpdatedAt   *time.Time `orm:"secret_result_updated_at"`
	SecretResultMaskedTail  string     `orm:"secret_result_masked_tail"`
	AvailableAt             time.Time  `orm:"available_at"`
	LeaseToken              *string    `orm:"lease_token"`
	LeaseExpiresAt          *time.Time `orm:"lease_expires_at"`
	AttemptCount            int        `orm:"attempt_count"`
	LastErrorCode           string     `orm:"last_error_code"`
	CompletedAt             *time.Time `orm:"completed_at"`
}

func (s *serviceImpl) prepareConfigureIntent(
	ctx context.Context,
	actor int,
	input ChannelConfigurationInput,
	nonSecretJSON string,
) (*channelIntentRow, error) {
	operationNo := stableNumber("nci_", input.ChannelCode, "configure", input.IdempotencyKey)
	shapeDigest := digestText("notification_channel_configure_v1", input.ChannelCode,
		input.ProviderCode, nonSecretJSON, stringInt64(input.ExpectedVersion))
	var prepared channelIntentRow
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		existing, loadErr := s.loadChannelIntentForUpdate(ctx, operationNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if !sameConfigureIntent(existing, input, shapeDigest) {
				return bizerr.NewCode(CodeConflict)
			}
			prepared = *existing
			return nil
		}

		var channel channelRow
		if loadErr = s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", input.ChannelCode).LockUpdate().Scan(&channel); loadErr != nil {
			return loadErr
		}
		if channel.ChannelCode == "" || channel.ConfigurationVersion != input.ExpectedVersion ||
			ChannelStatus(channel.StatusCode) == ChannelConfiguring {
			return bizerr.NewCode(CodeConflict)
		}

		operationCode := channelIntentConfigureCreate
		var expectedUpdatedAt *time.Time
		if channel.CredentialSecretRef != nil || channel.CredentialUpdatedAt != nil {
			if channel.CredentialSecretRef == nil || channel.CredentialUpdatedAt == nil {
				return bizerr.NewCode(CodeChannelUnavailable)
			}
			if _, handleErr := secretcap.NewHandle(*channel.CredentialSecretRef); handleErr != nil {
				return bizerr.NewCode(CodeChannelUnavailable)
			}
			operationCode = channelIntentConfigureReplace
			value := channel.CredentialUpdatedAt.UTC()
			expectedUpdatedAt = &value
		}
		providerCode := input.ProviderCode
		secretOperationKey := stableNumber("nsecop_", input.ChannelCode, "configure", input.IdempotencyKey)
		result, insertErr := s.database.Model(channelIntentTable).Ctx(ctx).Data(modeldo.ChannelIntent{
			TenantId: notificationTenantID, OperationNo: operationNo,
			ChannelCode: input.ChannelCode, OperationCode: operationCode,
			StatusCode: channelIntentPrepared, ActorUserId: actor,
			BaseStatusCode: channel.StatusCode, BaseVersion: input.ExpectedVersion,
			RequestShapeDigest: shapeDigest, DesiredProviderCode: providerCode,
			DesiredNonSecretConfiguration: nonSecretJSON,
			SecretOperationKey:            secretOperationKey,
			CredentialSecretRef:           channel.CredentialSecretRef,
			SecretExpectedUpdatedAt:       expectedUpdatedAt,
			AvailableAt:                   gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '30 seconds'"),
		}).Insert()
		if insertErr != nil || requireOne(result) != nil {
			return firstChannelIntentError(insertErr, CodeConflict)
		}
		updated, updateErr := s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", input.ChannelCode).
			Where("configuration_version", input.ExpectedVersion).
			Where("status_code", channel.StatusCode).
			Data(modeldo.Channel{StatusCode: string(ChannelConfiguring)}).Update()
		if updateErr != nil || requireOne(updated) != nil {
			return firstChannelIntentError(updateErr, CodeConflict)
		}
		prepared = channelIntentRow{
			OperationNo: operationNo, ChannelCode: input.ChannelCode, OperationCode: operationCode,
			StatusCode: channelIntentPrepared, ActorUserID: actor, BaseStatusCode: channel.StatusCode,
			BaseVersion: input.ExpectedVersion, RequestShapeDigest: shapeDigest,
			DesiredProviderCode: &providerCode, DesiredNonSecretConfig: nonSecretJSON,
			SecretOperationKey: secretOperationKey, CredentialSecretRef: channel.CredentialSecretRef,
			SecretExpectedUpdatedAt: expectedUpdatedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &prepared, nil
}

func (s *serviceImpl) prepareClearIntent(
	ctx context.Context,
	actor int,
	input ChannelLifecycleInput,
) (*channelIntentRow, error) {
	operationNo := stableNumber("nci_", input.ChannelCode, "clear", input.IdempotencyKey)
	shapeDigest := digestText("notification_channel_clear_v1", input.ChannelCode, stringInt64(input.ExpectedVersion))
	var prepared channelIntentRow
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		existing, loadErr := s.loadChannelIntentForUpdate(ctx, operationNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.ChannelCode != input.ChannelCode || existing.OperationCode != channelIntentClear ||
				existing.BaseVersion != input.ExpectedVersion || existing.RequestShapeDigest != shapeDigest {
				return bizerr.NewCode(CodeConflict)
			}
			prepared = *existing
			return nil
		}

		var channel channelRow
		if loadErr = s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", input.ChannelCode).LockUpdate().Scan(&channel); loadErr != nil {
			return loadErr
		}
		if channel.ChannelCode == "" || channel.ConfigurationVersion != input.ExpectedVersion ||
			ChannelStatus(channel.StatusCode) != ChannelDisabled ||
			channel.CredentialSecretRef == nil || channel.CredentialUpdatedAt == nil {
			return bizerr.NewCode(CodeConflict)
		}
		if _, handleErr := secretcap.NewHandle(*channel.CredentialSecretRef); handleErr != nil {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		pending, countErr := s.database.Model("plugin_idc_notify_delivery_task").Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("channel_code", input.ChannelCode).
			Where("status_code='sending' OR (external_boundary_crossed=TRUE AND status_code IN ('submitted','result_unknown','manual_review'))").Count()
		if countErr != nil {
			return countErr
		}
		if pending > 0 {
			return bizerr.NewCode(CodeConflict)
		}

		expectedUpdatedAt := channel.CredentialUpdatedAt.UTC()
		secretOperationKey := stableNumber("nsecop_", input.ChannelCode, "clear", input.IdempotencyKey)
		result, insertErr := s.database.Model(channelIntentTable).Ctx(ctx).Data(modeldo.ChannelIntent{
			TenantId: notificationTenantID, OperationNo: operationNo,
			ChannelCode: input.ChannelCode, OperationCode: channelIntentClear,
			StatusCode: channelIntentPrepared, ActorUserId: actor,
			BaseStatusCode: channel.StatusCode, BaseVersion: input.ExpectedVersion,
			RequestShapeDigest: shapeDigest, DesiredNonSecretConfiguration: "{}",
			SecretOperationKey:      secretOperationKey,
			CredentialSecretRef:     channel.CredentialSecretRef,
			SecretExpectedUpdatedAt: expectedUpdatedAt,
			AvailableAt:             gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '30 seconds'"),
		}).Insert()
		if insertErr != nil || requireOne(result) != nil {
			return firstChannelIntentError(insertErr, CodeConflict)
		}
		updated, updateErr := s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", input.ChannelCode).
			Where("configuration_version", input.ExpectedVersion).
			Where("status_code", string(ChannelDisabled)).
			Data(modeldo.Channel{StatusCode: string(ChannelConfiguring)}).Update()
		if updateErr != nil || requireOne(updated) != nil {
			return firstChannelIntentError(updateErr, CodeConflict)
		}
		prepared = channelIntentRow{
			OperationNo: operationNo, ChannelCode: input.ChannelCode, OperationCode: channelIntentClear,
			StatusCode: channelIntentPrepared, ActorUserID: actor, BaseStatusCode: channel.StatusCode,
			BaseVersion: input.ExpectedVersion, RequestShapeDigest: shapeDigest,
			DesiredNonSecretConfig: "{}", SecretOperationKey: secretOperationKey,
			CredentialSecretRef:     channel.CredentialSecretRef,
			SecretExpectedUpdatedAt: &expectedUpdatedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &prepared, nil
}

func (s *serviceImpl) executeConfigureIntent(
	ctx context.Context,
	intent *channelIntentRow,
	secretPayload []byte,
) error {
	if intent == nil || len(secretPayload) == 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.secretBinding(intent.ChannelCode)
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	var (
		handle   secretcap.Handle
		metadata secretcap.Metadata
	)
	switch intent.OperationCode {
	case channelIntentConfigureCreate:
		created, createErr := binding.Manager.Create(ctx, intent.SecretOperationKey, func(consume secretcap.Consumer) error {
			return consume(secretPayload)
		})
		if createErr != nil || created.Handle.IsZero() || !validConfiguredSecretMetadata(created.Metadata) {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		handle, metadata = created.Handle, created.Metadata
	case channelIntentConfigureReplace:
		if intent.CredentialSecretRef == nil || intent.SecretExpectedUpdatedAt == nil {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		handle, err = secretcap.NewHandle(*intent.CredentialSecretRef)
		if err != nil {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		replaced, replaceErr := binding.Manager.Replace(ctx, handle, intent.SecretExpectedUpdatedAt.UTC(),
			intent.SecretOperationKey, func(consume secretcap.Consumer) error { return consume(secretPayload) })
		if replaceErr != nil || !validConfiguredSecretMetadata(replaced.Metadata) ||
			!replaced.Metadata.UpdatedAt.UTC().After(intent.SecretExpectedUpdatedAt.UTC()) {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		metadata = replaced.Metadata
	default:
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	if err = s.persistChannelIntentSecret(ctx, intent, handle, metadata, ""); err != nil {
		return bizerr.WrapCode(err, CodeChannelUnavailable)
	}
	return s.finalizeChannelIntent(ctx, intent.OperationNo)
}

func (s *serviceImpl) executeClearIntent(ctx context.Context, intent *channelIntentRow) error {
	if intent == nil || intent.OperationCode != channelIntentClear ||
		intent.CredentialSecretRef == nil || intent.SecretExpectedUpdatedAt == nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	binding, err := s.secretBinding(intent.ChannelCode)
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	handle, err := secretcap.NewHandle(*intent.CredentialSecretRef)
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	metadata, err := binding.Manager.Revoke(ctx, handle, intent.SecretExpectedUpdatedAt.UTC())
	if err != nil || metadata.Configured ||
		!metadata.UpdatedAt.UTC().After(intent.SecretExpectedUpdatedAt.UTC()) {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	if err = s.persistChannelIntentSecret(ctx, intent, handle, metadata, ""); err != nil {
		return bizerr.WrapCode(err, CodeChannelUnavailable)
	}
	return s.finalizeChannelIntent(ctx, intent.OperationNo)
}

// ProcessChannelIntents lets a new process recover work left between the
// secretcap commit and the notification database commit. It never needs the
// original plaintext.
func (s *serviceImpl) ProcessChannelIntents(ctx context.Context, limit int) error {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumWorkerBatch {
		return bizerr.NewCode(CodeInvalid)
	}
	rows, err := s.claimChannelIntents(ctx, limit)
	if err != nil {
		return publicError(err)
	}
	for index := range rows {
		row := &rows[index]
		if recoverErr := s.recoverChannelIntent(ctx, row); recoverErr != nil {
			if releaseErr := s.releaseChannelIntent(ctx, row, "recovery_waiting"); releaseErr != nil {
				return publicError(releaseErr)
			}
		}
	}
	return nil
}

func (s *serviceImpl) claimChannelIntents(ctx context.Context, limit int) ([]channelIntentRow, error) {
	leaseToken, err := newChannelIntentLease()
	if err != nil {
		return nil, err
	}
	var rows []channelIntentRow
	err = s.database.GetScan(ctx, &rows, `
WITH due AS (
    SELECT tenant_id,operation_no
    FROM plugin_idc_notify_channel_intent
    WHERE tenant_id=? AND status_code IN ('prepared','secret_applied')
      AND available_at <= CURRENT_TIMESTAMP
      AND (lease_token IS NULL OR lease_expires_at < CURRENT_TIMESTAMP)
    ORDER BY available_at,operation_no
    FOR UPDATE SKIP LOCKED
    LIMIT ?
)
UPDATE plugin_idc_notify_channel_intent i
SET lease_token=?,lease_expires_at=CURRENT_TIMESTAMP + INTERVAL '2 minutes',
    attempt_count=attempt_count+1,updated_at=CURRENT_TIMESTAMP
FROM due
WHERE i.tenant_id=due.tenant_id AND i.operation_no=due.operation_no
RETURNING i.*`, notificationTenantID, limit, leaseToken)
	return rows, err
}

func (s *serviceImpl) recoverChannelIntent(ctx context.Context, intent *channelIntentRow) error {
	if intent == nil || intent.LeaseToken == nil || strings.TrimSpace(*intent.LeaseToken) == "" {
		return gerror.New("notification channel recovery lease is invalid")
	}
	if intent.StatusCode == channelIntentSecretApplied {
		return s.finalizeChannelIntent(ctx, intent.OperationNo)
	}
	binding, err := s.secretBinding(intent.ChannelCode)
	if err != nil {
		return err
	}
	switch intent.OperationCode {
	case channelIntentConfigureCreate:
		created, recoverErr := binding.Manager.RecoverCreate(ctx, intent.SecretOperationKey)
		if recoverErr != nil || created.Handle.IsZero() || !validConfiguredSecretMetadata(created.Metadata) {
			return gerror.New("notification channel create is not recoverable yet")
		}
		if err = s.persistChannelIntentSecret(ctx, intent, created.Handle, created.Metadata, *intent.LeaseToken); err != nil {
			return err
		}
	case channelIntentConfigureReplace:
		if intent.CredentialSecretRef == nil || intent.SecretExpectedUpdatedAt == nil {
			return gerror.New("notification channel replacement recovery state is invalid")
		}
		handle, handleErr := secretcap.NewHandle(*intent.CredentialSecretRef)
		if handleErr != nil {
			return handleErr
		}
		metadata, metadataErr := binding.Manager.Metadata(ctx, handle)
		if metadataErr != nil || !validConfiguredSecretMetadata(metadata) ||
			!metadata.UpdatedAt.UTC().After(intent.SecretExpectedUpdatedAt.UTC()) {
			return gerror.New("notification channel replacement is awaiting the original retry")
		}
		if err = s.persistChannelIntentSecret(ctx, intent, handle, metadata, *intent.LeaseToken); err != nil {
			return err
		}
	case channelIntentClear:
		if intent.CredentialSecretRef == nil || intent.SecretExpectedUpdatedAt == nil {
			return gerror.New("notification channel clear recovery state is invalid")
		}
		handle, handleErr := secretcap.NewHandle(*intent.CredentialSecretRef)
		if handleErr != nil {
			return handleErr
		}
		metadata, revokeErr := binding.Manager.Revoke(ctx, handle, intent.SecretExpectedUpdatedAt.UTC())
		if revokeErr != nil || metadata.Configured ||
			!metadata.UpdatedAt.UTC().After(intent.SecretExpectedUpdatedAt.UTC()) {
			return gerror.New("notification channel clear is not recoverable yet")
		}
		if err = s.persistChannelIntentSecret(ctx, intent, handle, metadata, *intent.LeaseToken); err != nil {
			return err
		}
	default:
		return gerror.New("notification channel recovery operation is invalid")
	}
	return s.finalizeChannelIntent(ctx, intent.OperationNo)
}

func (s *serviceImpl) persistChannelIntentSecret(
	ctx context.Context,
	intent *channelIntentRow,
	handle secretcap.Handle,
	metadata secretcap.Metadata,
	leaseToken string,
) error {
	if intent == nil || handle.IsZero() || metadata.UpdatedAt.IsZero() || len(metadata.MaskedTail) > 16 {
		return gerror.New("notification channel secret result is invalid")
	}
	model := s.database.Model(channelIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).
		Where("operation_no", intent.OperationNo).
		Where("status_code", channelIntentPrepared)
	if leaseToken == "" {
		model = model.Where("lease_token IS NULL OR lease_expires_at < CURRENT_TIMESTAMP")
	} else {
		model = model.Where("lease_token", leaseToken)
	}
	result, err := model.Data(modeldo.ChannelIntent{
		StatusCode:             channelIntentSecretApplied,
		CredentialSecretRef:    handle.OpaqueRef(),
		SecretResultUpdatedAt:  metadata.UpdatedAt.UTC(),
		SecretResultMaskedTail: metadata.MaskedTail,
		AvailableAt:            gdb.Raw("CURRENT_TIMESTAMP"),
		LeaseToken:             gdb.Raw("NULL"), LeaseExpiresAt: gdb.Raw("NULL"), LastErrorCode: "",
	}).Update()
	if err != nil {
		return err
	}
	if oneErr := requireOne(result); oneErr == nil {
		return nil
	}
	current, loadErr := s.loadChannelIntent(ctx, intent.OperationNo)
	if loadErr == nil && current != nil &&
		(current.StatusCode == channelIntentSecretApplied || current.StatusCode == channelIntentCompleted) {
		return nil
	}
	return firstChannelIntentError(loadErr, CodeConflict)
}

func (s *serviceImpl) finalizeChannelIntent(ctx context.Context, operationNo string) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		intent, err := s.loadChannelIntentForUpdate(ctx, operationNo)
		if err != nil {
			return err
		}
		if intent == nil {
			return bizerr.NewCode(CodeConflict)
		}
		if intent.StatusCode == channelIntentCompleted {
			return nil
		}
		if intent.StatusCode != channelIntentSecretApplied || intent.CredentialSecretRef == nil ||
			intent.SecretResultUpdatedAt == nil {
			return bizerr.NewCode(CodeConflict)
		}

		var channel channelRow
		if err = s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", intent.ChannelCode).LockUpdate().Scan(&channel); err != nil {
			return err
		}
		if channel.ChannelCode == "" || channel.ConfigurationVersion != intent.BaseVersion ||
			ChannelStatus(channel.StatusCode) != ChannelConfiguring {
			return bizerr.NewCode(CodeConflict)
		}

		var (
			operation string
			reason    string
			data      modeldo.Channel
		)
		switch intent.OperationCode {
		case channelIntentConfigureCreate, channelIntentConfigureReplace:
			if intent.DesiredProviderCode == nil || strings.TrimSpace(*intent.DesiredProviderCode) == "" {
				return bizerr.NewCode(CodeConflict)
			}
			operation, reason = "configure", "merchant_configuration"
			targetStatus := string(ChannelEnabled)
			if ChannelStatus(intent.BaseStatusCode) == ChannelDisabled {
				targetStatus = string(ChannelDisabled)
			}
			data = modeldo.Channel{
				ProviderCode: *intent.DesiredProviderCode, StatusCode: targetStatus,
				BalanceState: channelBalanceState(intent.ChannelCode), NonSecretConfiguration: intent.DesiredNonSecretConfig,
				CredentialSecretRef:  *intent.CredentialSecretRef,
				CredentialMaskedTail: intent.SecretResultMaskedTail,
				CredentialUpdatedAt:  intent.SecretResultUpdatedAt.UTC(), DisabledAt: gdb.Raw("NULL"),
				ConfigurationGeneration: gdb.Raw("configuration_generation + 1"),
				ConfigurationVersion:    gdb.Raw("configuration_version + 1"),
			}
		case channelIntentClear:
			operation, reason = "clear", "merchant_clear_configuration"
			data = modeldo.Channel{
				ProviderCode: gdb.Raw("NULL"), StatusCode: string(ChannelUnconfigured), BalanceState: channelBalanceState(intent.ChannelCode),
				NonSecretConfiguration: gdb.Raw("'{}'::jsonb"), CredentialSecretRef: gdb.Raw("NULL"),
				CredentialMaskedTail: "", CredentialUpdatedAt: gdb.Raw("NULL"), DisabledAt: gdb.Raw("NULL"),
				ConfigurationVersion: gdb.Raw("configuration_version + 1"),
			}
		default:
			return bizerr.NewCode(CodeConflict)
		}
		updated, updateErr := s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", intent.ChannelCode).
			Where("configuration_version", intent.BaseVersion).
			Where("status_code", string(ChannelConfiguring)).Data(data).Update()
		if updateErr != nil || requireOne(updated) != nil {
			return firstChannelIntentError(updateErr, CodeConflict)
		}
		if err = s.appendChannelAudit(ctx, intent.ActorUserID, intent.ChannelCode, operation, reason,
			intent.BaseVersion, 0, intent.OperationNo); err != nil {
			return err
		}
		completed, completeErr := s.database.Model(channelIntentTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).
			Where("operation_no", intent.OperationNo).
			Where("status_code", channelIntentSecretApplied).
			Data(modeldo.ChannelIntent{StatusCode: channelIntentCompleted,
				CompletedAt: gdb.Raw("CURRENT_TIMESTAMP"), LeaseToken: gdb.Raw("NULL"), LeaseExpiresAt: gdb.Raw("NULL")}).Update()
		if completeErr != nil || requireOne(completed) != nil {
			return firstChannelIntentError(completeErr, CodeConflict)
		}
		return nil
	})
}

func channelBalanceState(channelCode string) string {
	if channelCode == "sms" {
		return "unknown"
	}
	return "sufficient"
}

func (s *serviceImpl) releaseChannelIntent(ctx context.Context, intent *channelIntentRow, errorCode string) error {
	if intent == nil || intent.LeaseToken == nil {
		return gerror.New("notification channel recovery lease is unavailable")
	}
	result, err := s.database.Model(channelIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).
		Where("operation_no", intent.OperationNo).
		Where("lease_token", *intent.LeaseToken).
		Where("status_code IN ('prepared','secret_applied')").
		Data(modeldo.ChannelIntent{
			LeaseToken: gdb.Raw("NULL"), LeaseExpiresAt: gdb.Raw("NULL"),
			AvailableAt:   gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '30 seconds'"),
			LastErrorCode: errorCode,
		}).Update()
	if err != nil {
		return err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if rows == 0 {
		current, loadErr := s.loadChannelIntent(ctx, intent.OperationNo)
		if loadErr == nil && current != nil {
			if current.StatusCode == channelIntentCompleted ||
				(current.StatusCode == channelIntentSecretApplied && current.LeaseToken == nil) {
				return nil
			}
		}
		return firstChannelIntentError(loadErr, CodeConflict)
	}
	return nil
}

func (s *serviceImpl) loadChannelIntent(ctx context.Context, operationNo string) (*channelIntentRow, error) {
	var row channelIntentRow
	if err := s.database.Model(channelIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).
		Where("operation_no", operationNo).Scan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if row.OperationNo == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *serviceImpl) loadChannelIntentForUpdate(ctx context.Context, operationNo string) (*channelIntentRow, error) {
	var row channelIntentRow
	if err := s.database.Model(channelIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).
		Where("operation_no", operationNo).LockUpdate().Scan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if row.OperationNo == "" {
		return nil, nil
	}
	return &row, nil
}

func sameConfigureIntent(intent *channelIntentRow, input ChannelConfigurationInput, shapeDigest string) bool {
	if intent == nil || intent.ChannelCode != input.ChannelCode || intent.BaseVersion != input.ExpectedVersion ||
		intent.RequestShapeDigest != shapeDigest || intent.DesiredProviderCode == nil ||
		*intent.DesiredProviderCode != input.ProviderCode {
		return false
	}
	return intent.OperationCode == channelIntentConfigureCreate || intent.OperationCode == channelIntentConfigureReplace
}

func validConfiguredSecretMetadata(metadata secretcap.Metadata) bool {
	return metadata.Configured && !metadata.UpdatedAt.IsZero() && len(metadata.MaskedTail) <= 16
}

func newChannelIntentLease() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", gerror.Wrap(err, "generate notification channel recovery lease")
	}
	return "ncilease_" + hex.EncodeToString(value[:]), nil
}

func firstChannelIntentError(err error, code *bizerr.Code) error {
	if err != nil {
		return err
	}
	return bizerr.NewCode(code)
}

func stringInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
