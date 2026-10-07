// This file reads one authorized, bounded, secret-safe provider configuration projection.
package providerconfig

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

func (s *serviceImpl) Get(ctx context.Context) (*Configuration, error) {
	if _, err := s.authorize(ctx, PermissionQuery); err != nil {
		return nil, err
	}
	row, err := s.load(ctx, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return &Configuration{ProviderCode: providerCode, Status: statusDraft}, nil
	}
	return s.project(ctx, row)
}

func (s *serviceImpl) load(ctx context.Context, lock bool) (*entity.ProviderConfiguration, error) {
	columns := dao.ProviderConfiguration.Columns()
	model := dao.ProviderConfiguration.Ctx(ctx).
		Fields(
			columns.Id, columns.TenantId, columns.ConfigurationNo, columns.ProviderCode,
			columns.StatusCode, columns.SceneId, columns.CredentialSecretRef,
			columns.CredentialGeneration, columns.LastMutationDigest,
			columns.MutationOperationKey, columns.MutationSecretUpdatedAt,
			columns.LastValidationCode, columns.ValidatedAt, columns.Version, columns.UpdatedAt,
		).
		Where(do.ProviderConfiguration{TenantId: idccap.DefaultTenantID, ProviderCode: providerCode})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.ProviderConfiguration
	if err := model.Limit(1).Scan(&row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *serviceImpl) project(
	ctx context.Context,
	row *entity.ProviderConfiguration,
) (*Configuration, error) {
	if row == nil || row.ProviderCode != providerCode || row.ConfigurationNo != configurationNo ||
		row.TenantId != idccap.DefaultTenantID || row.Version <= 0 || row.CredentialGeneration < 0 ||
		!validStatus(Status(row.StatusCode)) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	projection := &Configuration{
		ProviderCode: providerCode, Status: Status(row.StatusCode), SceneID: row.SceneId,
		CredentialGeneration: row.CredentialGeneration, Version: row.Version,
		UpdatedAt: utcTimePointer(row.UpdatedAt),
	}
	if row.LastValidationCode == "" {
		projection.ValidationStatus = ValidationStatusUntested
	} else {
		projection.ValidationStatus = ValidationStatus(row.LastValidationCode)
		if !validValidationStatus(projection.ValidationStatus) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		projection.ValidatedAt = utcTimePointer(row.ValidatedAt)
		if projection.ValidatedAt == nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	if Status(row.StatusCode) != statusConfigured {
		return projection, nil
	}
	if row.CredentialSecretRef == nil || *row.CredentialSecretRef == "" {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	handle, err := secretcap.NewHandle(*row.CredentialSecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	metadata, err := s.manager.Metadata(ctx, handle)
	if err != nil || !metadata.Configured || metadata.UpdatedAt.IsZero() {
		return nil, bizerr.NewCode(CodeCredentialUnavailable)
	}
	projection.CredentialConfigured = true
	projection.CredentialMaskedTail = metadata.MaskedTail
	return projection, nil
}

func validValidationStatus(status ValidationStatus) bool {
	switch status {
	case ValidationStatusReachable, ValidationStatusRejected, ValidationStatusUnavailable:
		return true
	default:
		return false
	}
}

func validStatus(status Status) bool {
	switch status {
	case statusDraft, statusUpdating, statusConfigured, statusDisabled:
		return true
	default:
		return false
	}
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}
