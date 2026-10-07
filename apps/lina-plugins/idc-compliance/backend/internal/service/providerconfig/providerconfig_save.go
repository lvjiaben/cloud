// This file performs audited, fail-closed, idempotently resumable credential replacement.
package providerconfig

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	auditPrepareOperation  = "compliance.aliyun-face-credential.update.prepare"
	auditCompleteOperation = "compliance.aliyun-face-credential.update.complete"
)

type mutationStage struct {
	row                   *entity.ProviderConfiguration
	handle                secretcap.Handle
	operationKey          string
	secretExpectedUpdated time.Time
	replayed              bool
}

type secretMutation struct {
	handle secretcap.Handle
	reused bool
}

// Save performs a fail-closed two-phase replacement. The local row is first
// moved to updating and audited, so a crash or audit failure can never leave a
// newly changed credential enabled without a resumable mutation marker.
func (s *serviceImpl) Save(ctx context.Context, input SaveInput) (*Configuration, error) {
	defer clearCredentialInput(&input)
	actorUserID, err := s.authorize(ctx, PermissionWrite)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actorUserID); err != nil {
		return nil, err
	}
	owned := SaveInput{
		SceneID: input.SceneID, ExpectedVersion: input.ExpectedVersion,
		IdempotencyKey:  input.IdempotencyKey,
		AccessKeyID:     append([]byte(nil), input.AccessKeyID...),
		AccessKeySecret: append([]byte(nil), input.AccessKeySecret...),
	}
	defer clearCredentialInput(&owned)
	normalized, digest, err := normalizeSave(owned)
	if err != nil || aliyunface.ValidateCredential(owned.AccessKeyID, owned.AccessKeySecret) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	operationKey, err := newMutationOperationKey()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	stage, err := s.prepareMutation(ctx, normalized, digest, operationKey)
	if err != nil {
		return nil, err
	}
	if stage.replayed {
		return s.project(ctx, stage.row)
	}
	producer, err := aliyunface.NewCredentialProducer(owned.AccessKeyID, owned.AccessKeySecret)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	mutation, err := s.mutateSecret(ctx, stage, producer)
	if err != nil {
		return nil, err
	}
	if err = s.completeMutation(ctx, stage, digest, mutation.handle); err != nil {
		return nil, err
	}
	row, err := s.load(ctx, false)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.project(ctx, row)
}

func (s *serviceImpl) prepareMutation(
	ctx context.Context,
	input SaveInput,
	digest string,
	operationKey string,
) (mutationStage, error) {
	var stage mutationStage
	err := dao.ProviderConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, true)
		if err != nil {
			return err
		}
		if row == nil {
			if input.ExpectedVersion != 0 || !lowerHex64Pattern.MatchString(operationKey) {
				return bizerr.NewCode(CodeConflict)
			}
			if _, err = dao.ProviderConfiguration.Ctx(ctx).
				Data(do.ProviderConfiguration{
					TenantId: idccap.DefaultTenantID, ConfigurationNo: configurationNo,
					ProviderCode: providerCode, StatusCode: string(statusUpdating), SceneId: input.SceneID,
					CredentialGeneration: 0, LastMutationDigest: &digest,
					MutationOperationKey: &operationKey, Version: 1,
				}).Insert(); err != nil {
				return err
			}
			row, err = s.load(ctx, true)
			if err != nil || row == nil {
				return firstError(err, bizerr.NewCode(CodeUnavailable))
			}
			if err = s.appendPrepareAudit(ctx, nil, row); err != nil {
				return err
			}
			return s.resumeStage(row, &stage)
		}
		if digestPointerEquals(row.LastMutationDigest, digest) {
			if row.SceneId != input.SceneID {
				return bizerr.NewCode(CodeConflict)
			}
			switch Status(row.StatusCode) {
			case statusConfigured:
				stage = mutationStage{row: row, replayed: true}
				return nil
			case statusUpdating:
				return s.resumeStage(row, &stage)
			default:
				return bizerr.NewCode(CodeConflict)
			}
		}
		if Status(row.StatusCode) == statusUpdating || row.Version != input.ExpectedVersion ||
			!lowerHex64Pattern.MatchString(operationKey) {
			return bizerr.NewCode(CodeConflict)
		}
		before := *row
		var fence *time.Time
		if row.CredentialSecretRef != nil && *row.CredentialSecretRef != "" {
			handle, handleErr := secretcap.NewHandle(*row.CredentialSecretRef)
			if handleErr != nil {
				return bizerr.NewCode(CodeUnavailable)
			}
			metadata, metadataErr := s.manager.Metadata(ctx, handle)
			if metadataErr != nil || !metadata.Configured || metadata.UpdatedAt.IsZero() {
				return bizerr.NewCode(CodeCredentialUnavailable)
			}
			updated := metadata.UpdatedAt.UTC()
			fence = &updated
		}
		columns := dao.ProviderConfiguration.Columns()
		result, updateErr := dao.ProviderConfiguration.Ctx(ctx).
			Where(do.ProviderConfiguration{Id: row.Id, Version: row.Version}).
			Fields(
				columns.StatusCode, columns.SceneId, columns.LastMutationDigest,
				columns.MutationOperationKey, columns.MutationSecretUpdatedAt,
				columns.LastValidationCode, columns.ValidatedAt,
			).
			Data(do.ProviderConfiguration{
				StatusCode: string(statusUpdating), SceneId: input.SceneID,
				LastMutationDigest: &digest, MutationOperationKey: &operationKey,
				MutationSecretUpdatedAt: fence, LastValidationCode: "", ValidatedAt: nil,
			}).Update()
		if updateErr != nil {
			return updateErr
		}
		if err = requireOneRow(result); err != nil {
			return err
		}
		row, err = s.load(ctx, true)
		if err != nil || row == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if err = s.appendPrepareAudit(ctx, &before, row); err != nil {
			return err
		}
		return s.resumeStage(row, &stage)
	})
	if err != nil {
		return mutationStage{}, publicWriteError(err)
	}
	return stage, nil
}

func (s *serviceImpl) resumeStage(row *entity.ProviderConfiguration, stage *mutationStage) error {
	if row == nil || row.MutationOperationKey == nil ||
		!lowerHex64Pattern.MatchString(*row.MutationOperationKey) {
		return bizerr.NewCode(CodeUnavailable)
	}
	stage.row = row
	stage.operationKey = *row.MutationOperationKey
	if row.CredentialSecretRef == nil || *row.CredentialSecretRef == "" {
		if row.CredentialGeneration != 0 || row.MutationSecretUpdatedAt != nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		return nil
	}
	handle, err := secretcap.NewHandle(*row.CredentialSecretRef)
	if err != nil || row.MutationSecretUpdatedAt == nil || row.MutationSecretUpdatedAt.IsZero() {
		return bizerr.NewCode(CodeUnavailable)
	}
	stage.handle = handle
	stage.secretExpectedUpdated = row.MutationSecretUpdatedAt.UTC()
	return nil
}

func (s *serviceImpl) mutateSecret(
	ctx context.Context,
	stage mutationStage,
	producer secretcap.PlaintextProducer,
) (secretMutation, error) {
	if !lowerHex64Pattern.MatchString(stage.operationKey) {
		return secretMutation{}, bizerr.NewCode(CodeUnavailable)
	}
	secretKey := "aliyun-face-credential:" + stage.operationKey
	if stage.handle.IsZero() {
		created, err := s.manager.Create(ctx, secretKey, producer)
		if err != nil || created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
			return secretMutation{}, bizerr.NewCode(CodeCredentialUnavailable)
		}
		return secretMutation{handle: created.Handle, reused: created.Reused}, nil
	}
	mutation, err := s.manager.Replace(
		ctx, stage.handle, stage.secretExpectedUpdated, secretKey, producer,
	)
	if err != nil || !mutation.Metadata.Configured || mutation.Metadata.UpdatedAt.IsZero() {
		return secretMutation{}, bizerr.NewCode(CodeCredentialUnavailable)
	}
	return secretMutation{handle: stage.handle, reused: mutation.Reused}, nil
}

func (s *serviceImpl) completeMutation(
	ctx context.Context,
	stage mutationStage,
	digest string,
	handle secretcap.Handle,
) error {
	if handle.IsZero() || !lowerHex64Pattern.MatchString(stage.operationKey) {
		return bizerr.NewCode(CodeCredentialUnavailable)
	}
	err := dao.ProviderConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, true)
		if err != nil || row == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if digestPointerEquals(row.LastMutationDigest, digest) && Status(row.StatusCode) == statusConfigured {
			if row.CredentialSecretRef == nil || *row.CredentialSecretRef == "" {
				return bizerr.NewCode(CodeConflict)
			}
			persisted, handleErr := secretcap.NewHandle(*row.CredentialSecretRef)
			if handleErr != nil || !persisted.Equal(handle) {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if Status(row.StatusCode) != statusUpdating || !digestPointerEquals(row.LastMutationDigest, digest) ||
			row.Version != stage.row.Version {
			return bizerr.NewCode(CodeConflict)
		}
		if row.CredentialSecretRef != nil && *row.CredentialSecretRef != "" {
			persisted, handleErr := secretcap.NewHandle(*row.CredentialSecretRef)
			if handleErr != nil || !persisted.Equal(handle) {
				return bizerr.NewCode(CodeConflict)
			}
		}
		secretReference := handle.OpaqueRef()
		result, updateErr := dao.ProviderConfiguration.Ctx(ctx).
			Where(do.ProviderConfiguration{
				Id: row.Id, Version: row.Version, StatusCode: string(statusUpdating),
			}).
			Data(do.ProviderConfiguration{
				StatusCode: string(statusConfigured), CredentialSecretRef: &secretReference,
				CredentialGeneration: gdb.Raw(dao.ProviderConfiguration.Columns().CredentialGeneration + " + 1"),
				Version:              gdb.Raw(dao.ProviderConfiguration.Columns().Version + " + 1"),
			}).Update()
		if updateErr != nil {
			return updateErr
		}
		if err = requireOneRow(result); err != nil {
			return err
		}
		after, err := s.load(ctx, true)
		if err != nil || after == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: auditCompleteOperation, IdempotencyKey: stage.operationKey,
			OperationCode: auditCompleteOperation, TargetType: auditTargetType,
			TargetNo: configurationNo, ResultCode: auditResultSucceeded, ReasonCode: auditReasonCredentialUpdate,
			RiskCode: auditRiskHigh, RequiredPermission: PermissionWrite,
			BeforeSnapshot: configurationAuditSnapshot(row), AfterSnapshot: configurationAuditSnapshot(after),
		})
	})
	return publicWriteError(err)
}

func (s *serviceImpl) appendPrepareAudit(
	ctx context.Context,
	before *entity.ProviderConfiguration,
	after *entity.ProviderConfiguration,
) error {
	if after == nil || after.MutationOperationKey == nil ||
		!lowerHex64Pattern.MatchString(*after.MutationOperationKey) {
		return bizerr.NewCode(CodeUnavailable)
	}
	return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: auditPrepareOperation, IdempotencyKey: *after.MutationOperationKey,
		OperationCode: auditPrepareOperation, TargetType: auditTargetType,
		TargetNo: configurationNo, ResultCode: auditResultAccepted, ReasonCode: auditReasonCredentialUpdate,
		RiskCode: auditRiskHigh, RequiredPermission: PermissionWrite,
		BeforeSnapshot: configurationAuditSnapshot(before), AfterSnapshot: configurationAuditSnapshot(after),
	})
}

func configurationAuditSnapshot(row *entity.ProviderConfiguration) map[string]any {
	if row == nil {
		return map[string]any{"state": statusDraft, "credential_configured": false, "version": int64(0)}
	}
	return map[string]any{
		"state": row.StatusCode, "provider_code": providerCode, "scene_id": row.SceneId,
		"credential_configured": row.CredentialSecretRef != nil && *row.CredentialSecretRef != "",
		"credential_generation": row.CredentialGeneration,
		"validation_status":     safeValidationAuditStatus(row.LastValidationCode),
		"version":               row.Version,
	}
}

func safeValidationAuditStatus(value string) string {
	if value == "" {
		return string(ValidationStatusUntested)
	}
	status := ValidationStatus(value)
	if !validValidationStatus(status) {
		return string(ValidationStatusUnavailable)
	}
	return value
}

func publicWriteError(err error) error {
	if err == nil {
		return nil
	}
	if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) ||
		bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeCredentialUnavailable) ||
		bizerr.Is(err, CodeRecentAuthenticationRequired) {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
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

func digestPointerEquals(value *string, expected string) bool {
	return value != nil && *value == expected
}

func firstError(primary error, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
