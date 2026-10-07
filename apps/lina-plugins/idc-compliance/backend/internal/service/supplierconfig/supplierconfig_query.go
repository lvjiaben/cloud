// This file projects supplier configuration without exposing supplier identity plaintext.

package supplierconfig

import (
	"context"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

func (s *serviceImpl) Get(ctx context.Context, provider string) (*Configuration, error) {
	provider = normalizeProvider(provider)
	if _, err := s.authorize(ctx, PermissionQuery, provider); err != nil {
		return nil, err
	}
	return s.currentConfiguration(ctx, provider)
}

func (s *serviceImpl) currentConfiguration(
	ctx context.Context,
	provider string,
) (*Configuration, error) {
	row, err := s.load(ctx, provider, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return &Configuration{
			ProviderCode: provider, CredentialStatus: CredentialUnconfigured,
			SubjectStatus: SubjectUnbound, LastOperationKind: OperationNone,
			LastOperationResult:    OperationResultNone,
			CapabilityEvidenceCode: EvidenceExternalConfirmationRequired,
		}, nil
	}
	return s.project(ctx, row)
}

func (s *serviceImpl) load(
	ctx context.Context,
	provider string,
	lock bool,
) (*entity.SupplierConfiguration, error) {
	columns := dao.SupplierConfiguration.Columns()
	model := dao.SupplierConfiguration.Ctx(ctx).
		Fields(
			columns.Id, columns.TenantId, columns.ConfigurationNo, columns.ProviderCode,
			columns.CredentialStatusCode, columns.CredentialSecretRef,
			columns.CredentialGeneration, columns.CapabilityEvidenceCode, columns.AdapterEnabled,
			columns.LastMutationDigest, columns.MutationOperationKey,
			columns.MutationSecretUpdatedAt, columns.SubjectStatusCode, columns.SubjectBindingNo,
			columns.LastOperationKindCode, columns.LastOperationResultCode,
			columns.LastOperationNo, columns.LastOperationAt,
			columns.SubjectProofDigest, columns.SubjectProofExpiresAt,
			columns.Version, columns.UpdatedAt,
		).
		Where(do.SupplierConfiguration{TenantId: idccap.DefaultTenantID, ProviderCode: provider})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.SupplierConfiguration
	if err := model.Limit(1).Scan(&row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *serviceImpl) project(
	ctx context.Context,
	row *entity.SupplierConfiguration,
) (*Configuration, error) {
	if row == nil || !validProvider(row.ProviderCode) ||
		row.ConfigurationNo != configurationNo(row.ProviderCode) ||
		row.TenantId != idccap.DefaultTenantID || row.Version <= 0 ||
		row.CredentialGeneration < 0 || !validAdapterGate(row) ||
		!validCredentialStatus(CredentialStatus(row.CredentialStatusCode)) ||
		!validSubjectStatus(SubjectStatus(row.SubjectStatusCode)) ||
		!validOperationKind(OperationKind(row.LastOperationKindCode)) ||
		!validOperationResult(OperationResult(row.LastOperationResultCode)) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	projection := &Configuration{
		ProviderCode:           row.ProviderCode,
		CredentialStatus:       CredentialStatus(row.CredentialStatusCode),
		CredentialGeneration:   row.CredentialGeneration,
		SubjectStatus:          SubjectStatus(row.SubjectStatusCode),
		SubjectBindingNo:       safeString(row.SubjectBindingNo),
		LastOperationKind:      OperationKind(row.LastOperationKindCode),
		LastOperationResult:    OperationResult(row.LastOperationResultCode),
		LastOperationNo:        safeString(row.LastOperationNo),
		LastOperationAt:        utcPointer(row.LastOperationAt),
		CapabilityEvidenceCode: row.CapabilityEvidenceCode,
		AdapterEnabled:         row.AdapterEnabled, Version: row.Version, UpdatedAt: utcPointer(row.UpdatedAt),
	}
	if CredentialStatus(row.CredentialStatusCode) != CredentialConfigured {
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

func validAdapterGate(row *entity.SupplierConfiguration) bool {
	if row == nil {
		return false
	}
	if row.ProviderCode == ProviderTencentESSBasic {
		if row.AdapterEnabled {
			return row.CapabilityEvidenceCode == EvidenceOfficialPublicContract &&
				SubjectStatus(row.SubjectStatusCode) == SubjectActive
		}
		return row.CapabilityEvidenceCode == EvidenceExternalConfirmationRequired ||
			row.CapabilityEvidenceCode == EvidenceOfficialPublicContract
	}
	return row.ProviderCode == ProviderNuonuoInvoice && !row.AdapterEnabled &&
		row.CapabilityEvidenceCode == EvidenceExternalConfirmationRequired
}

func validCredentialStatus(status CredentialStatus) bool {
	switch status {
	case CredentialUnconfigured, CredentialUpdating, CredentialConfigured, CredentialRetryableFailed:
		return true
	default:
		return false
	}
}

func validSubjectStatus(status SubjectStatus) bool {
	switch status {
	case SubjectUnbound, SubjectBinding, SubjectActive, SubjectRevoked,
		SubjectReopening, SubjectRetryableFailed:
		return true
	default:
		return false
	}
}

func validOperationKind(kind OperationKind) bool {
	switch kind {
	case OperationNone, OperationSubjectBind, OperationSubjectRevoke, OperationSubjectReopen:
		return true
	default:
		return false
	}
}

func validOperationResult(result OperationResult) bool {
	switch result {
	case OperationResultNone, OperationResultPending, OperationResultSucceeded,
		OperationResultRetryableFailed:
		return true
	default:
		return false
	}
}

func normalizeProvider(provider string) string { return strings.TrimSpace(provider) }

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}
