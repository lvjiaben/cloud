package contractautosign

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/dialect"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*Item, error) {
	actor, err := s.authorize(ctx, "contract-auto-sign")
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	input, digest, fingerprint, err := normalizeCreate(input, now)
	if err != nil {
		return nil, err
	}
	var replay *authorizationRow
	if err = s.database.GetScan(ctx, &replay, `
SELECT authorization_no, creation_fingerprint
FROM plugin_idc_compliance_auto_sign_auth
WHERE tenant_id = ? AND creation_idempotency_digest = ?`, idccap.DefaultTenantID, digest); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if replay != nil {
		if replay.CreationFingerprint != fingerprint {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return s.get(ctx, replay.AuthorizationNo)
	}
	status, err := s.gateway.TencentOnboardingStatus(ctx)
	if err != nil || status.State != "active" || !autoSignAvailable(status) {
		return nil, bizerr.NewCode(CodeCapabilityUnavailable)
	}
	var authorizationNo string
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, updateErr := tx.Exec(`
UPDATE plugin_idc_compliance_auto_sign_auth
SET status_code = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND status_code = ? AND expires_at <= CURRENT_TIMESTAMP`,
			StatusExpired, idccap.DefaultTenantID, StatusActive); updateErr != nil {
			return updateErr
		}
		var existing *authorizationRow
		if scanErr := tx.Ctx(ctx).GetScan(&existing, `
SELECT authorization_no, currency_code, allow_required_order, allow_manual_optional, max_amount_minor,
       success_limit, success_count, status_code, expires_at, created_by_user_id, revoked_by_user_id,
       revoked_at, version, created_at, creation_fingerprint
FROM plugin_idc_compliance_auto_sign_auth
WHERE tenant_id = ? AND creation_idempotency_digest = ? FOR UPDATE`, idccap.DefaultTenantID, digest); scanErr != nil {
			return scanErr
		}
		if existing != nil {
			if existing.CreationFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			authorizationNo = existing.AuthorizationNo
			return nil
		}
		count, countErr := tx.Model(do.AutoSignAuth{}).Ctx(ctx).
			Where("tenant_id", idccap.DefaultTenantID).Where("currency_code", input.CurrencyCode).
			Where("status_code", StatusActive).Count()
		if countErr != nil || count > 0 {
			return bizerr.NewCode(CodeConflict)
		}
		valid, validateErr := exactTemplatesEligible(ctx, tx, input.Templates)
		if validateErr != nil {
			return validateErr
		}
		if !valid {
			return bizerr.NewCode(CodeConflict)
		}
		authorizationNo, validateErr = newAuthorizationNo()
		if validateErr != nil {
			return validateErr
		}
		if _, validateErr = tx.Model(do.AutoSignAuth{}).Ctx(ctx).Data(do.AutoSignAuth{
			TenantId: idccap.DefaultTenantID, AuthorizationNo: authorizationNo, ProviderCode: ProviderTencent,
			CurrencyCode: input.CurrencyCode, AllowRequiredOrder: input.AllowRequiredOrder,
			AllowManualOptional: input.AllowManualOptional, MaxAmountMinor: input.MaxAmountMinor,
			SuccessLimit: input.SuccessLimit, SuccessCount: int64(0), StatusCode: StatusActive,
			ExpiresAt: &input.ExpiresAt, CreatedByUserId: actor, CreationIdempotencyDigest: digest,
			CreationFingerprint: fingerprint, Version: int64(1),
		}).Insert(); validateErr != nil {
			return validateErr
		}
		relations := make([]do.AutoSignAuthTemplate, 0, len(input.Templates))
		for _, ref := range input.Templates {
			relations = append(relations, do.AutoSignAuthTemplate{TenantId: idccap.DefaultTenantID,
				AuthorizationNo: authorizationNo, TemplateNo: ref.TemplateNo, VersionNo: ref.VersionNo})
		}
		if _, validateErr = tx.Model(do.AutoSignAuthTemplate{}).Ctx(ctx).Data(relations).Insert(); validateErr != nil {
			return validateErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"authorization_no": ""},
			IdempotencyScope: "compliance.contract-auto-sign.create", IdempotencyKey: digest,
			OperationCode: "compliance.contract-auto-sign.create", TargetType: "contract_auto_sign_authorization",
			TargetNo: authorizationNo, ResultCode: "succeeded", ReasonCode: "enterprise_auto_sign_enabled",
			RiskCode: "high", RequiredPermission: PermissionManage,
			AfterSnapshot: map[string]any{"currency": input.CurrencyCode, "template_count": len(input.Templates),
				"success_limit": input.SuccessLimit, "expires_at": input.ExpiresAt.UnixMilli()},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || dialect.IsUniqueConstraintViolation(err) {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.get(ctx, authorizationNo)
}

func (s *serviceImpl) Revoke(ctx context.Context, input RevokeInput) (*Item, error) {
	actor, err := s.authorize(ctx, input.AuthorizationNo)
	if err != nil {
		return nil, err
	}
	input, digest, fingerprint, err := normalizeRevoke(input)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var row *struct {
			StatusCode                  string  `json:"status_code"`
			Version                     int64   `json:"version"`
			RevocationIdempotencyDigest *string `json:"revocation_idempotency_digest"`
			RevocationFingerprint       *string `json:"revocation_fingerprint"`
		}
		if scanErr := tx.Ctx(ctx).GetScan(&row, `SELECT status_code, version, revocation_idempotency_digest, revocation_fingerprint FROM plugin_idc_compliance_auto_sign_auth WHERE tenant_id = ? AND authorization_no = ? FOR UPDATE`, idccap.DefaultTenantID, input.AuthorizationNo); scanErr != nil {
			return scanErr
		}
		if row == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if row.StatusCode == StatusRevoked {
			if row.RevocationIdempotencyDigest != nil && row.RevocationFingerprint != nil &&
				*row.RevocationIdempotencyDigest == digest && *row.RevocationFingerprint == fingerprint {
				return nil
			}
			return bizerr.NewCode(CodeConflict)
		}
		if row.StatusCode != StatusActive || row.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr := tx.Model(do.AutoSignAuth{}).Ctx(ctx).
			Where("tenant_id", idccap.DefaultTenantID).Where("authorization_no", input.AuthorizationNo).
			Where("version", input.ExpectedVersion).Where("status_code", StatusActive).
			Data(do.AutoSignAuth{StatusCode: StatusRevoked, RevokedByUserId: &actor,
				RevokedAt: &now, RevocationIdempotencyDigest: &digest, RevocationFingerprint: &fingerprint,
				Version: input.ExpectedVersion + 1}).Update()
		if updateErr != nil {
			return updateErr
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-auto-sign.revoke", IdempotencyKey: digest,
			OperationCode: "compliance.contract-auto-sign.revoke", TargetType: "contract_auto_sign_authorization",
			TargetNo: input.AuthorizationNo, ResultCode: "succeeded", ReasonCode: "enterprise_auto_sign_revoked",
			RiskCode: "high", RequiredPermission: PermissionManage,
			BeforeSnapshot: map[string]any{"status": StatusActive}, AfterSnapshot: map[string]any{"status": StatusRevoked},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.get(ctx, input.AuthorizationNo)
}

func autoSignAvailable(status localgatewayclient.TencentOnboardingResult) bool {
	for _, capability := range status.Capabilities {
		if capability.Code == "auto_sign.enterprise" {
			return capability.State == "available"
		}
	}
	return false
}

func exactTemplatesEligible(ctx context.Context, tx gdb.TX, refs []TemplateRef) (bool, error) {
	placeholders := make([]string, 0, len(refs))
	args := make([]any, 0, len(refs)*2+1)
	args = append(args, idccap.DefaultTenantID)
	for _, ref := range refs {
		placeholders = append(placeholders, "(?, ?)")
		args = append(args, ref.TemplateNo, ref.VersionNo)
	}
	value, err := tx.Ctx(ctx).GetValue(`
SELECT COUNT(*)
FROM plugin_idc_compliance_contract_template_version AS version
WHERE version.tenant_id = ? AND (version.template_no, version.version_no) IN (`+strings.Join(placeholders, ",")+`)
  AND version.status_code = 'enabled' AND version.scope_code IN ('enterprise', 'all_customers')
  AND (version.provider_merchant_seal_count > 0 OR EXISTS (
    SELECT 1 FROM plugin_idc_compliance_contract_template_field_set AS field_set
    JOIN plugin_idc_compliance_contract_template_field AS field
      ON field.tenant_id = field_set.tenant_id AND field.field_set_no = field_set.field_set_no
    WHERE field_set.tenant_id = version.tenant_id AND field_set.field_set_no = version.current_field_set_no
      AND field.party_code = 'merchant' AND field.field_type_code = 'enterprise_seal'
  ))`, args...)
	return err == nil && value.Int() == len(refs), err
}

func (s *serviceImpl) get(ctx context.Context, authorizationNo string) (*Item, error) {
	var row *authorizationRow
	if err := s.database.GetScan(ctx, &row, `
SELECT authorization_no, currency_code, allow_required_order, allow_manual_optional, max_amount_minor,
       success_limit, success_count, CASE WHEN status_code = 'active' AND expires_at <= CURRENT_TIMESTAMP THEN 'expired' ELSE status_code END AS status_code,
       expires_at, created_by_user_id, revoked_by_user_id, revoked_at, version, created_at
FROM plugin_idc_compliance_auto_sign_auth WHERE tenant_id = ? AND authorization_no = ?`, idccap.DefaultTenantID, authorizationNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	item, valid := projectAuthorization(*row)
	if !valid {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	var refs []TemplateRef
	if err := s.database.GetScan(ctx, &refs, `SELECT relation.template_no, relation.version_no, version.name FROM plugin_idc_compliance_auto_sign_auth_template AS relation JOIN plugin_idc_compliance_contract_template_version AS version ON version.tenant_id = relation.tenant_id AND version.template_no = relation.template_no AND version.version_no = relation.version_no WHERE relation.tenant_id = ? AND relation.authorization_no = ? ORDER BY relation.id`, idccap.DefaultTenantID, authorizationNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	item.Templates = refs
	return &item, nil
}
