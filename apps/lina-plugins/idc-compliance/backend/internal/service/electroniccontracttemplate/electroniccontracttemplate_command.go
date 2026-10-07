// This file implements local draft creation, copy, metadata, and enable-state transitions.

package electroniccontracttemplate

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	auditTargetType = "electronic_contract_template"
	auditRiskHigh   = "high"
)

type mutableRow struct {
	TemplateNo                     string  `json:"template_no"`
	VersionNo                      int64   `json:"version_no"`
	Name                           string  `json:"name"`
	ScopeCode                      *string `json:"scope_code"`
	SigningOrderCode               *string `json:"signing_order_code"`
	LanguageCode                   *string `json:"language_code"`
	StatusCode                     string  `json:"status_code"`
	OriginCode                     string  `json:"origin_code"`
	ProviderStateCode              *string `json:"provider_state_code"`
	ProviderSnapshotDigest         *string `json:"provider_snapshot_digest"`
	ProviderCustomerSignatureCount int     `json:"provider_customer_signature_count"`
	ProviderCustomerSealCount      int     `json:"provider_customer_seal_count"`
	ProviderMerchantSignatureCount int     `json:"provider_merchant_signature_count"`
	ProviderMerchantSealCount      int     `json:"provider_merchant_seal_count"`
	CurrentContentRevisionNo       *string `json:"current_content_revision_no"`
	CurrentFieldSetNo              *string `json:"current_field_set_no"`
	CurrentProviderMappingSetNo    *string `json:"current_provider_mapping_set_no"`
	PreviewConfirmed               bool    `json:"preview_confirmed"`
	UsedCount                      int64   `json:"used_count"`
	LockVersion                    int64   `json:"lock_version"`
}

// Create creates an idempotent first local draft without body bytes.
func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*Item, error) {
	actor, err := s.authorize(ctx, PermissionManage, templateDirectoryTarget)
	if err != nil {
		return nil, err
	}
	input, digest, fingerprint, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}
	return s.createLocalDraft(ctx, actor, input, digest, fingerprint, "", 0)
}

// Copy creates a new local logical template and intentionally copies no body bytes.
func (s *serviceImpl) Copy(ctx context.Context, input CopyInput) (*Item, error) {
	input, err := normalizeCopyInput(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionManage, input.SourceTemplateNo)
	if err != nil {
		return nil, err
	}
	source, err := s.get(ctx, input.SourceTemplateNo, input.SourceVersionNo, false)
	if err != nil {
		return nil, err
	}
	digest := sha256Hex([]byte("contract-template-copy\x00" + input.IdempotencyKey))
	fingerprint := creationFingerprintForCopy(*source)
	return s.createLocalDraft(ctx, actor, CreateInput{
		Name: source.Name, ScopeCode: source.ScopeCode, SigningOrderCode: source.SigningOrderCode,
		LanguageCode:   source.LanguageCode,
		IdempotencyKey: input.IdempotencyKey,
	}, digest, fingerprint, source.TemplateNo, source.VersionNo)
}

func (s *serviceImpl) createLocalDraft(
	ctx context.Context,
	actor int,
	input CreateInput,
	digest string,
	fingerprint string,
	sourceTemplateNo string,
	sourceVersionNo int64,
) (*Item, error) {
	if actor <= 0 || !validDigest(digest) || !validDigest(fingerprint) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var templateNo string
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var existing *struct {
			TemplateNo  string  `json:"template_no"`
			Fingerprint *string `json:"creation_fingerprint"`
		}
		if scanErr := tx.Ctx(ctx).GetScan(&existing, `
SELECT template_no, creation_fingerprint
FROM plugin_idc_compliance_contract_template
WHERE tenant_id = ? AND creation_idempotency_digest = ?
FOR UPDATE`, idccap.DefaultTenantID, digest); scanErr != nil {
			return scanErr
		}
		if existing != nil {
			if existing.Fingerprint == nil || *existing.Fingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			templateNo = existing.TemplateNo
			return nil
		}
		var createErr error
		templateNo, createErr = newTemplateNo()
		if createErr != nil {
			return createErr
		}
		if _, createErr = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_contract_template
    (tenant_id, template_no, origin_code, creation_idempotency_digest, creation_fingerprint)
VALUES (?, ?, 'local', ?, ?)`, idccap.DefaultTenantID, templateNo, digest, fingerprint); createErr != nil {
			return createErr
		}
		if _, createErr = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_contract_template_version
    (tenant_id, template_no, version_no, name, scope_code, signing_order_code, language_code,
     status_code, lock_version)
VALUES (?, ?, 1, ?, ?, ?, ?, 'draft', 1)`,
			idccap.DefaultTenantID, templateNo, input.Name, input.ScopeCode,
			input.SigningOrderCode, input.LanguageCode); createErr != nil {
			return createErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.create",
			IdempotencyKey:   digest, OperationCode: "compliance.contract-template.create",
			TargetType: auditTargetType, TargetNo: templateNo, ResultCode: "succeeded",
			ReasonCode: "local_draft_created", RiskCode: auditRiskHigh,
			RequiredPermission: PermissionManage,
			BeforeSnapshot: map[string]any{
				"source_template_no": sourceTemplateNo, "source_version_no": sourceVersionNo,
			},
			AfterSnapshot: map[string]any{
				"origin": OriginLocal, "status": StatusDraft, "version": int64(1),
			},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.get(ctx, templateNo, 1, false)
}

// Update changes local draft metadata or completes required synchronized settings.
func (s *serviceImpl) Update(ctx context.Context, input UpdateInput) (*Item, error) {
	input, err := normalizeUpdateInput(input)
	if err != nil {
		return nil, err
	}
	if _, err = s.authorize(ctx, PermissionManage, input.TemplateNo); err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadMutableRow(ctx, tx, input.TemplateNo, input.VersionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		if row.LockVersion != input.ExpectedVersion ||
			(row.StatusCode != StatusDraft && row.StatusCode != StatusNeedsConfiguration) {
			return bizerr.NewCode(CodeConflict)
		}
		if row.OriginCode == OriginTencentSynced &&
			(row.StatusCode != StatusNeedsConfiguration || input.Name != row.Name) {
			return bizerr.NewCode(CodeConflict)
		}
		nextStatus := row.StatusCode
		if row.OriginCode == OriginTencentSynced {
			compatible, compatibilityErr := providerScopeCompatibleTX(
				ctx, tx, row.TemplateNo, row.VersionNo, input.ScopeCode,
			)
			if compatibilityErr != nil {
				return compatibilityErr
			}
			if !compatible {
				return bizerr.NewCode(CodeConflict)
			}
		}
		result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET name = ?, scope_code = ?, signing_order_code = ?, language_code = ?, status_code = ?,
    lock_version = lock_version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND lock_version = ?
  AND status_code IN ('draft', 'needs_configuration')`,
			input.Name, input.ScopeCode, input.SigningOrderCode, input.LanguageCode, nextStatus,
			idccap.DefaultTenantID, input.TemplateNo, input.VersionNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOne(result); updateErr != nil {
			return updateErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.metadata",
			IdempotencyKey:   input.TemplateNo + ":" + stringInt(input.VersionNo) + ":" + stringInt(input.ExpectedVersion),
			OperationCode:    "compliance.contract-template.metadata", TargetType: auditTargetType,
			TargetNo: input.TemplateNo, ResultCode: "succeeded", ReasonCode: "metadata_saved",
			RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
			BeforeSnapshot: templateAuditSnapshot(row),
			AfterSnapshot: map[string]any{
				"version_no": input.VersionNo, "status": nextStatus,
				"scope": input.ScopeCode, "signing_order": input.SigningOrderCode,
				"language":     input.LanguageCode,
				"lock_version": input.ExpectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.get(ctx, input.TemplateNo, input.VersionNo, false)
}

// Enable explicitly enables one version and disables any previously enabled sibling.
func (s *serviceImpl) Enable(ctx context.Context, input StateInput) (*Item, error) {
	return s.changeState(ctx, input, true)
}

// Disable explicitly prevents new contract use without deleting history.
func (s *serviceImpl) Disable(ctx context.Context, input StateInput) (*Item, error) {
	return s.changeState(ctx, input, false)
}

func (s *serviceImpl) changeState(ctx context.Context, input StateInput, enable bool) (*Item, error) {
	input, err := normalizeStateInput(input)
	if err != nil {
		return nil, err
	}
	if _, err = s.authorize(ctx, PermissionManage, input.TemplateNo); err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, lockErr := tx.Ctx(ctx).Exec(`
SELECT id
FROM plugin_idc_compliance_contract_template_version
WHERE tenant_id = ? AND template_no = ?
ORDER BY version_no
FOR UPDATE`, idccap.DefaultTenantID, input.TemplateNo); lockErr != nil {
			return lockErr
		}
		row, loadErr := loadMutableRow(ctx, tx, input.TemplateNo, input.VersionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		if row.LockVersion != input.ExpectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		before := templateAuditSnapshot(row)
		nextStatus := StatusDisabled
		if enable {
			if row.StatusCode != StatusDisabled && row.StatusCode != StatusSubmitted {
				return bizerr.NewCode(CodeConflict)
			}
			if row.ScopeCode == nil || row.SigningOrderCode == nil || row.LanguageCode == nil ||
				row.OriginCode == OriginTencentSynced &&
					(row.ProviderStateCode == nil || *row.ProviderStateCode != ProviderStateActive ||
						row.CurrentProviderMappingSetNo == nil || !providerMutableRowSigningPositionsReady(row)) ||
				row.OriginCode == OriginLocal &&
					(row.CurrentContentRevisionNo == nil || row.CurrentFieldSetNo == nil || !row.PreviewConfirmed) {
				return bizerr.NewCode(CodeConflict)
			}
			if row.OriginCode == OriginLocal {
				fields, fieldErr := loadFieldsTX(ctx, tx, row.TemplateNo, row.VersionNo, *row.CurrentFieldSetNo)
				if fieldErr != nil || !signingPositionsReady(fields, safeString(row.ScopeCode)) {
					return firstError(fieldErr, bizerr.NewCode(CodeConflict))
				}
			}
			if _, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET status_code = 'disabled', lock_version = lock_version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND status_code = 'enabled'
  AND version_no <> ?`, idccap.DefaultTenantID, input.TemplateNo, input.VersionNo); updateErr != nil {
				return updateErr
			}
			nextStatus = StatusEnabled
		} else if row.StatusCode != StatusEnabled {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET status_code = ?, lock_version = lock_version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND version_no = ?
  AND lock_version = ? AND status_code = ?`,
			nextStatus, idccap.DefaultTenantID, input.TemplateNo, input.VersionNo,
			input.ExpectedVersion, row.StatusCode)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOne(result); updateErr != nil {
			return updateErr
		}
		operation := "disable"
		if enable {
			operation = "enable"
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template." + operation,
			IdempotencyKey:   input.TemplateNo + ":" + stringInt(input.VersionNo) + ":" + stringInt(input.ExpectedVersion),
			OperationCode:    "compliance.contract-template." + operation,
			TargetType:       auditTargetType, TargetNo: input.TemplateNo,
			ResultCode: "succeeded", ReasonCode: "local_version_" + operation + "d",
			RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
			BeforeSnapshot: before,
			AfterSnapshot: map[string]any{
				"version_no": input.VersionNo, "status": nextStatus,
				"lock_version": input.ExpectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.get(ctx, input.TemplateNo, input.VersionNo, false)
}

func loadMutableRow(ctx context.Context, tx gdb.TX, templateNo string, versionNo int64) (*mutableRow, error) {
	var row *mutableRow
	err := tx.Ctx(ctx).GetScan(&row, `
SELECT version.template_no, version.version_no, version.name, version.scope_code,
	   version.signing_order_code, version.language_code, version.status_code, template.origin_code,
       template.provider_state_code, version.provider_snapshot_digest,
       version.provider_customer_signature_count, version.provider_customer_seal_count,
       version.provider_merchant_signature_count, version.provider_merchant_seal_count,
       version.current_content_revision_no, version.current_field_set_no,
       version.current_provider_mapping_set_no,
       version.preview_confirmed, version.used_count, version.lock_version
FROM plugin_idc_compliance_contract_template_version AS version
JOIN plugin_idc_compliance_contract_template AS template
  ON template.tenant_id = version.tenant_id AND template.template_no = version.template_no
WHERE version.tenant_id = ? AND version.template_no = ? AND version.version_no = ?
FOR UPDATE`, idccap.DefaultTenantID, templateNo, versionNo)
	return row, err
}

func providerMutableRowSigningPositionsReady(row *mutableRow) bool {
	if row == nil || row.ScopeCode == nil {
		return false
	}
	customer := row.ProviderCustomerSignatureCount > 0
	if *row.ScopeCode == ScopeEnterprise {
		customer = customer || row.ProviderCustomerSealCount > 0
	}
	merchant := row.ProviderMerchantSignatureCount > 0 || row.ProviderMerchantSealCount > 0
	return customer && merchant
}

func loadFieldsTX(ctx context.Context, tx gdb.TX, templateNo string, versionNo int64, fieldSetNo string) ([]FieldPlacement, error) {
	var fields []FieldPlacement
	err := tx.Ctx(ctx).GetScan(&fields, `
SELECT field_no, sequence_no, field_type_code, party_code,
       COALESCE(variable_code, '') AS variable_code, page_no,
       position_x_ppm, position_y_ppm, width_ppm, height_ppm
FROM plugin_idc_compliance_contract_template_field
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND field_set_no = ?
ORDER BY sequence_no
FOR UPDATE`, idccap.DefaultTenantID, templateNo, versionNo, fieldSetNo)
	return fields, err
}

func requireOne(result sql.Result) error {
	if result == nil {
		return bizerr.NewCode(CodeConflict)
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func templateAuditSnapshot(row *mutableRow) map[string]any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"version_no": row.VersionNo, "status": row.StatusCode,
		"scope": safeString(row.ScopeCode), "signing_order": safeString(row.SigningOrderCode),
		"language": safeString(row.LanguageCode),
		"used":     row.UsedCount > 0, "lock_version": row.LockVersion,
	}
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func publicMutationError(err error) error {
	if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeContentInvalid) ||
		bizerr.Is(err, CodeConversionUnavailable) || bizerr.Is(err, CodeNotAccessible) ||
		bizerr.Is(err, CodeConflict) {
		return err
	}
	return bizerr.NewCode(CodeUnavailable)
}

func firstError(left, right error) error {
	if left != nil {
		return left
	}
	return right
}

func stringInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
