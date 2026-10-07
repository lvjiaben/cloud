// This file records one authorized, read-only provider connectivity validation.
package providerconfig

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const auditValidateOperation = "compliance.aliyun-face-credential.validate"

// ValidateConnectivity performs one non-initializing provider query and binds
// its bounded outcome to the exact configuration version and secret handle.
func (s *serviceImpl) ValidateConnectivity(ctx context.Context) (*Configuration, error) {
	actorUserID, err := s.authorize(ctx, PermissionValidate)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actorUserID); err != nil {
		return nil, err
	}
	beforeProbe, err := s.load(ctx, false)
	if err != nil || beforeProbe == nil || Status(beforeProbe.StatusCode) != statusConfigured ||
		beforeProbe.CredentialSecretRef == nil || *beforeProbe.CredentialSecretRef == "" {
		return nil, publicWriteError(firstError(err, bizerr.NewCode(CodeCredentialUnavailable)))
	}
	handle, err := secretcap.NewHandle(*beforeProbe.CredentialSecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeCredentialUnavailable)
	}
	validationKey, err := newMutationOperationKey()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	outcome := s.checker.Check(ctx, handle, beforeProbe.SceneId)
	if !validValidationStatus(outcome) {
		outcome = ValidationStatusUnavailable
	}
	validatedAt := s.now().UTC()
	err = dao.ProviderConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := s.load(ctx, true)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeUnavailable))
		}
		if row.Id != beforeProbe.Id || row.Version != beforeProbe.Version ||
			Status(row.StatusCode) != statusConfigured || row.CredentialSecretRef == nil ||
			*row.CredentialSecretRef != *beforeProbe.CredentialSecretRef {
			return bizerr.NewCode(CodeConflict)
		}
		before := *row
		result, updateErr := dao.ProviderConfiguration.Ctx(ctx).
			Where(do.ProviderConfiguration{Id: row.Id, Version: row.Version, StatusCode: string(statusConfigured)}).
			Data(do.ProviderConfiguration{
				LastValidationCode: string(outcome), ValidatedAt: &validatedAt,
			}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(result); updateErr != nil {
			return updateErr
		}
		after, loadErr := s.load(ctx, true)
		if loadErr != nil || after == nil {
			return firstError(loadErr, bizerr.NewCode(CodeUnavailable))
		}
		resultCode := auditResultFailed
		if outcome == ValidationStatusReachable {
			resultCode = auditResultSucceeded
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope:   auditValidateOperation,
			IdempotencyKey:     validationKey,
			OperationCode:      auditValidateOperation,
			TargetType:         auditTargetType,
			TargetNo:           configurationNo,
			ResultCode:         resultCode,
			ReasonCode:         auditReasonValidation,
			RiskCode:           auditRiskHigh,
			RequiredPermission: PermissionValidate,
			BeforeSnapshot:     configurationAuditSnapshot(&before),
			AfterSnapshot:      configurationAuditSnapshot(after),
		})
	})
	if err != nil {
		return nil, publicWriteError(err)
	}
	row, err := s.load(ctx, false)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.project(ctx, row)
}
