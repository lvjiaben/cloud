// This file persists merchant-owned supplier credentials through the governed secret service.

package supplierconfig

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	auditPrepareOperation  = "compliance.local-supplier-credential.update.prepare"
	auditCompleteOperation = "compliance.local-supplier-credential.update.complete"
	auditTargetType        = "local_supplier_configuration"
)

type mutationStage struct {
	row                   *entity.SupplierConfiguration
	handle                secretcap.Handle
	operationKey          string
	secretExpectedUpdated time.Time
	replayed              bool
}

func (s *serviceImpl) SaveCredential(
	ctx context.Context,
	input CredentialSaveInput,
) (*Configuration, error) {
	defer clearCredentialSaveInput(&input)
	provider := normalizeProvider(input.ProviderCode)
	actor, err := s.authorize(ctx, PermissionCredentialWrite, provider)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	owned := CredentialSaveInput{
		ProviderCode:       provider,
		CredentialEnvelope: append([]byte(nil), input.CredentialEnvelope...),
		ExpectedVersion:    input.ExpectedVersion, IdempotencyKey: input.IdempotencyKey,
	}
	defer clearCredentialSaveInput(&owned)
	normalized, digest, err := normalizeCredentialSave(owned)
	if err != nil {
		return nil, err
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
	producer, clearProducer := credentialProducer(owned.CredentialEnvelope)
	defer clearProducer()
	handle, err := s.mutateSecret(ctx, normalized.ProviderCode, stage, producer)
	if err != nil {
		if markErr := s.markCredentialFailed(ctx, normalized.ProviderCode, digest, stage); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return nil, err
	}
	if err = s.completeMutation(ctx, normalized.ProviderCode, digest, stage, handle); err != nil {
		if markErr := s.markCredentialFailed(ctx, normalized.ProviderCode, digest, stage); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return nil, err
	}
	row, err := s.load(ctx, normalized.ProviderCode, false)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(firstError(err, bizerr.NewCode(CodeUnavailable)), CodeUnavailable)
	}
	return s.project(ctx, row)
}

func (s *serviceImpl) markCredentialFailed(
	ctx context.Context,
	provider string,
	digest string,
	stage mutationStage,
) error {
	return publicError(dao.SupplierConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, provider, true)
		if err != nil || row == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if CredentialStatus(row.CredentialStatusCode) != CredentialUpdating ||
			!digestPointerEquals(row.LastMutationDigest, digest) || row.Version != stage.row.Version {
			return bizerr.NewCode(CodeConflict)
		}
		result, err := dao.SupplierConfiguration.Ctx(ctx).
			Where(do.SupplierConfiguration{
				Id: row.Id, Version: row.Version,
				CredentialStatusCode: string(CredentialUpdating),
			}).
			Data(do.SupplierConfiguration{
				CredentialStatusCode: string(CredentialRetryableFailed),
				Version:              gdb.Raw(dao.SupplierConfiguration.Columns().Version + " + 1"),
			}).Update()
		if err != nil {
			return err
		}
		if err = requireOneRow(result); err != nil {
			return err
		}
		after, err := s.load(ctx, provider, true)
		if err != nil || after == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: auditCompleteOperation,
			IdempotencyKey:   stage.operationKey + ":failed",
			OperationCode:    auditCompleteOperation, TargetType: auditTargetType,
			TargetNo: configurationNo(provider), ResultCode: string(CredentialRetryableFailed),
			ReasonCode: "credential_mutation_failed", RiskCode: "high",
			RequiredPermission: PermissionCredentialWrite,
			BeforeSnapshot:     auditSnapshot(row), AfterSnapshot: auditSnapshot(after),
		})
	}))
}

func (s *serviceImpl) prepareMutation(
	ctx context.Context,
	input CredentialSaveInput,
	digest string,
	operationKey string,
) (mutationStage, error) {
	stage := mutationStage{}
	err := dao.SupplierConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, input.ProviderCode, true)
		if err != nil {
			return err
		}
		if row != nil && digestPointerEquals(row.LastMutationDigest, digest) {
			if CredentialStatus(row.CredentialStatusCode) == CredentialConfigured {
				stage.row, stage.replayed = row, true
				return nil
			}
			if CredentialStatus(row.CredentialStatusCode) == CredentialUpdating {
				return resumeStage(row, &stage)
			}
		}
		if row != nil && (CredentialStatus(row.CredentialStatusCode) == CredentialUpdating || row.Version != input.ExpectedVersion) {
			return bizerr.NewCode(CodeConflict)
		}
		if row == nil && input.ExpectedVersion != 0 {
			return bizerr.NewCode(CodeConflict)
		}
		before := row
		var fence *time.Time
		if row != nil && row.CredentialSecretRef != nil && *row.CredentialSecretRef != "" {
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
		if row == nil {
			_, err = dao.SupplierConfiguration.Ctx(ctx).Data(do.SupplierConfiguration{
				TenantId: idccap.DefaultTenantID, ConfigurationNo: configurationNo(input.ProviderCode),
				ProviderCode: input.ProviderCode, CredentialStatusCode: string(CredentialUpdating),
				SubjectStatusCode:       string(SubjectUnbound),
				LastOperationKindCode:   string(OperationNone),
				LastOperationResultCode: string(OperationResultNone),
				CapabilityEvidenceCode:  EvidenceExternalConfirmationRequired, AdapterEnabled: false,
				LastMutationDigest: &digest, MutationOperationKey: &operationKey, Version: int64(1),
			}).Insert()
		} else {
			result, updateErr := dao.SupplierConfiguration.Ctx(ctx).
				Where(do.SupplierConfiguration{Id: row.Id, Version: row.Version}).
				Data(do.SupplierConfiguration{
					CredentialStatusCode:   string(CredentialUpdating),
					CapabilityEvidenceCode: EvidenceExternalConfirmationRequired, AdapterEnabled: false,
					LastMutationDigest: &digest, MutationOperationKey: &operationKey,
					MutationSecretUpdatedAt: fence,
				}).Update()
			if updateErr != nil {
				return updateErr
			}
			err = requireOneRow(result)
		}
		if err != nil {
			return err
		}
		row, err = s.load(ctx, input.ProviderCode, true)
		if err != nil || row == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if err = s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: auditPrepareOperation, IdempotencyKey: operationKey,
			OperationCode: auditPrepareOperation, TargetType: auditTargetType,
			TargetNo: configurationNo(input.ProviderCode), ResultCode: "accepted",
			ReasonCode: "merchant_owned_supplier_credential_update", RiskCode: "high",
			RequiredPermission: PermissionCredentialWrite,
			BeforeSnapshot:     auditSnapshot(before), AfterSnapshot: auditSnapshot(row),
		}); err != nil {
			return err
		}
		return resumeStage(row, &stage)
	})
	if err != nil {
		return mutationStage{}, publicError(err)
	}
	return stage, nil
}

func resumeStage(row *entity.SupplierConfiguration, stage *mutationStage) error {
	if row == nil || row.MutationOperationKey == nil ||
		!lowerHex64Pattern.MatchString(*row.MutationOperationKey) || row.AdapterEnabled ||
		row.CapabilityEvidenceCode != EvidenceExternalConfirmationRequired {
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
	provider string,
	stage mutationStage,
	producer secretcap.PlaintextProducer,
) (secretcap.Handle, error) {
	if !validProvider(provider) || !lowerHex64Pattern.MatchString(stage.operationKey) {
		return secretcap.Handle{}, bizerr.NewCode(CodeUnavailable)
	}
	idempotencyKey := "local-supplier-credential:" + provider + ":" + stage.operationKey
	if stage.handle.IsZero() {
		created, err := s.manager.Create(ctx, idempotencyKey, producer)
		if err != nil || created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
			return secretcap.Handle{}, bizerr.NewCode(CodeCredentialUnavailable)
		}
		return created.Handle, nil
	}
	mutation, err := s.manager.Replace(
		ctx, stage.handle, stage.secretExpectedUpdated, idempotencyKey, producer,
	)
	if err != nil || !mutation.Metadata.Configured || mutation.Metadata.UpdatedAt.IsZero() {
		return secretcap.Handle{}, bizerr.NewCode(CodeCredentialUnavailable)
	}
	return stage.handle, nil
}

func (s *serviceImpl) completeMutation(
	ctx context.Context,
	provider string,
	digest string,
	stage mutationStage,
	handle secretcap.Handle,
) error {
	if handle.IsZero() {
		return bizerr.NewCode(CodeCredentialUnavailable)
	}
	err := dao.SupplierConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, provider, true)
		if err != nil || row == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		if digestPointerEquals(row.LastMutationDigest, digest) &&
			CredentialStatus(row.CredentialStatusCode) == CredentialConfigured {
			return nil
		}
		if CredentialStatus(row.CredentialStatusCode) != CredentialUpdating ||
			!digestPointerEquals(row.LastMutationDigest, digest) ||
			row.Version != stage.row.Version || row.AdapterEnabled {
			return bizerr.NewCode(CodeConflict)
		}
		secretReference := handle.OpaqueRef()
		result, updateErr := dao.SupplierConfiguration.Ctx(ctx).
			Where(do.SupplierConfiguration{
				Id: row.Id, Version: row.Version,
				CredentialStatusCode: string(CredentialUpdating),
			}).
			Data(do.SupplierConfiguration{
				CredentialStatusCode: string(CredentialConfigured), CredentialSecretRef: &secretReference,
				CredentialGeneration: gdb.Raw(
					dao.SupplierConfiguration.Columns().CredentialGeneration + " + 1",
				),
				CapabilityEvidenceCode: EvidenceExternalConfirmationRequired, AdapterEnabled: false,
				Version: gdb.Raw(dao.SupplierConfiguration.Columns().Version + " + 1"),
			}).Update()
		if updateErr != nil {
			return updateErr
		}
		if err = requireOneRow(result); err != nil {
			return err
		}
		after, err := s.load(ctx, provider, true)
		if err != nil || after == nil {
			return firstError(err, bizerr.NewCode(CodeUnavailable))
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: auditCompleteOperation, IdempotencyKey: stage.operationKey,
			OperationCode: auditCompleteOperation, TargetType: auditTargetType,
			TargetNo: configurationNo(provider), ResultCode: "succeeded",
			ReasonCode: "merchant_owned_supplier_credential_update", RiskCode: "high",
			RequiredPermission: PermissionCredentialWrite,
			BeforeSnapshot:     auditSnapshot(row), AfterSnapshot: auditSnapshot(after),
		})
	})
	return publicError(err)
}

func auditSnapshot(row *entity.SupplierConfiguration) map[string]any {
	if row == nil {
		return map[string]any{
			"credential_state": CredentialUnconfigured, "credential_configured": false,
			"subject_state":       SubjectUnbound,
			"adapter_enabled":     false,
			"capability_evidence": EvidenceExternalConfirmationRequired,
			"version":             int64(0),
		}
	}
	return map[string]any{
		"credential_state": row.CredentialStatusCode, "provider_code": row.ProviderCode,
		"subject_state":         row.SubjectStatusCode,
		"subject_binding_no":    safeString(row.SubjectBindingNo),
		"last_operation_kind":   row.LastOperationKindCode,
		"last_operation_result": row.LastOperationResultCode,
		"last_operation_no":     safeString(row.LastOperationNo),
		"credential_configured": row.CredentialSecretRef != nil && *row.CredentialSecretRef != "",
		"credential_generation": row.CredentialGeneration,
		"adapter_enabled":       row.AdapterEnabled,
		"capability_evidence":   row.CapabilityEvidenceCode, "version": row.Version,
	}
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func publicError(err error) error {
	if err == nil {
		return nil
	}
	for _, code := range []*bizerr.Code{
		CodeInvalid, CodeNotAccessible, CodeRecentAuthenticationRequired, CodeConflict,
		CodeCredentialUnavailable, CodeExternalEvidenceRequired, CodeGatewayUnavailable,
	} {
		if bizerr.Is(err, code) {
			return err
		}
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

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
