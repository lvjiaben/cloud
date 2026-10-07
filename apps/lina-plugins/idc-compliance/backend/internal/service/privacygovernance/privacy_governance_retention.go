// Retention persistence binds each protected data type to an explicit policy.

package privacygovernance

import (
	"context"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const retentionDirectoryTarget = "privacy-retention-directory"

type retentionRow struct {
	PolicyNo      string    `json:"policy_no"`
	DataTypeCode  string    `json:"data_type_code"`
	Version       int64     `json:"version"`
	RetentionDays int       `json:"retention_days"`
	StatusCode    string    `json:"status_code"`
	CreatedBy     int       `json:"created_by_user_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func validDataType(value DataType) bool {
	switch value {
	case DataPersonalVerificationMaterial, DataEnterpriseVerificationMaterial,
		DataElectronicContractEvidence, DataTaxInvoiceEvidence,
		DataSensitiveAccessAudit, DataLocalSensitiveExport:
		return true
	default:
		return false
	}
}

func assessmentOwnsData(assessment AssessmentType, dataType DataType) bool {
	switch assessment {
	case AssessmentPersonalVerification:
		return dataType == DataPersonalVerificationMaterial
	case AssessmentEnterpriseVerification:
		return dataType == DataEnterpriseVerificationMaterial
	case AssessmentElectronicContract:
		return dataType == DataElectronicContractEvidence
	case AssessmentTaxInvoice:
		return dataType == DataTaxInvoiceEvidence
	case AssessmentLocalSensitiveExport:
		return dataType == DataLocalSensitiveExport
	case AssessmentAISafeProjection:
		return dataType == DataSensitiveAccessAudit
	default:
		return false
	}
}

func (s *serviceImpl) ListRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error) {
	if _, err := s.authorize(ctx, PermissionQuery, retentionDirectoryTarget); err != nil {
		return nil, err
	}
	var rows []retentionRow
	if err := s.database.GetScan(ctx, &rows, `SELECT policy_no, data_type_code, version, retention_days, status_code, created_by_user_id, created_at FROM plugin_idc_compliance_privacy_retention_policy WHERE tenant_id = ? AND status_code = 'active' ORDER BY data_type_code`, idccap.DefaultTenantID); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return projectRetentionRows(rows), nil
}

func (s *serviceImpl) SetRetentionPolicy(ctx context.Context, input RetentionPolicyInput) (*RetentionPolicy, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validDataType(input.DataType) || input.RetentionDays <= 0 || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionConfigure, "privacy-retention:"+string(input.DataType))
	if err != nil {
		return nil, err
	}
	digest := sha256Hex("privacy-retention-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex("privacy-retention-v1\x00" + string(input.DataType) + "\x00" + strconv.Itoa(input.RetentionDays))
	now := s.now().UTC().Truncate(time.Microsecond)
	var policyNo string
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var replays []struct {
			RequestFingerprint string `json:"request_fingerprint"`
			ResultReference    string `json:"result_reference"`
		}
		if loadErr := s.database.GetScan(ctx, &replays, `SELECT request_fingerprint, result_reference FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'set_retention' AND target_no = ? AND idempotency_digest = ?`, idccap.DefaultTenantID, input.DataType, digest); loadErr != nil {
			return loadErr
		}
		if len(replays) == 1 {
			replay := replays[0]
			if replay.RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			policyNo = replay.ResultReference
			return nil
		}
		var previousRows []retentionRow
		if loadErr := s.database.GetScan(ctx, &previousRows, `SELECT policy_no, data_type_code, version, retention_days, status_code, created_by_user_id, created_at FROM plugin_idc_compliance_privacy_retention_policy WHERE tenant_id = ? AND data_type_code = ? AND status_code = 'active' FOR UPDATE`, idccap.DefaultTenantID, input.DataType); loadErr != nil {
			return loadErr
		}
		var previous *retentionRow
		if len(previousRows) == 1 {
			previous = &previousRows[0]
		}
		if previous != nil && previous.RetentionDays == input.RetentionDays {
			policyNo = previous.PolicyNo
		} else {
			if previous != nil {
				if _, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_retention_policy SET status_code = 'superseded', superseded_at = ? WHERE tenant_id = ? AND policy_no = ? AND status_code = 'active'`, now, idccap.DefaultTenantID, previous.PolicyNo); updateErr != nil {
					return updateErr
				}
			}
			versionValue, loadErr := s.database.GetValue(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM plugin_idc_compliance_privacy_retention_policy WHERE tenant_id = ? AND data_type_code = ?`, idccap.DefaultTenantID, input.DataType)
			if loadErr != nil {
				return loadErr
			}
			version := versionValue.Int64()
			policyNo, err = newBusinessNo("PRP_")
			if err != nil {
				return err
			}
			if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_retention_policy (tenant_id, policy_no, data_type_code, version, retention_days, status_code, created_by_user_id, created_at) VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`, idccap.DefaultTenantID, policyNo, input.DataType, version, input.RetentionDays, actor, now); insertErr != nil {
				return insertErr
			}
		}
		actionNo := "PRA_" + sha256Hex("retention\x00" + string(input.DataType) + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'set_retention', ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, input.DataType, digest, fingerprint, policyNo, actor); insertErr != nil {
			return insertErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.retention", IdempotencyKey: digest,
			OperationCode: "compliance.privacy.retention-set", TargetType: "privacy_retention_policy",
			TargetNo: policyNo, ResultCode: "succeeded", ReasonCode: "privacy_retention_policy_set",
			RiskCode: "critical", RequiredPermission: PermissionConfigure,
			BeforeSnapshot: map[string]any{"data_type": input.DataType},
			AfterSnapshot:  map[string]any{"data_type": input.DataType, "retention_days": input.RetentionDays},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getRetentionPolicy(ctx, policyNo)
}

func (s *serviceImpl) RequireCollection(ctx context.Context, assessmentType AssessmentType, dataType DataType) (*CollectionDecision, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !assessmentOwnsData(assessmentType, dataType) {
		return nil, bizerr.NewCode(CodePolicyRequired)
	}
	var rows []struct {
		AssessmentNo  string    `json:"assessment_no"`
		PolicyNo      string    `json:"policy_no"`
		RetentionDays int       `json:"retention_days"`
		DatabaseNow   time.Time `json:"database_now"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT a.assessment_no, p.policy_no, p.retention_days, CURRENT_TIMESTAMP AS database_now FROM plugin_idc_compliance_privacy_assessment a JOIN plugin_idc_compliance_privacy_retention_policy p ON p.tenant_id = a.tenant_id AND p.data_type_code = ? AND p.status_code = 'active' WHERE a.tenant_id = ? AND a.assessment_type_code = ? AND a.status_code = 'published' LIMIT 1`, dataType, idccap.DefaultTenantID, assessmentType); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodePolicyRequired)
	}
	row := rows[0]
	if row.AssessmentNo == "" || row.PolicyNo == "" || row.RetentionDays <= 0 || row.DatabaseNow.IsZero() {
		return nil, bizerr.NewCode(CodePolicyRequired)
	}
	return &CollectionDecision{
		AssessmentNo: row.AssessmentNo, PolicyNo: row.PolicyNo,
		RetentionUntil: row.DatabaseNow.UTC().AddDate(0, 0, row.RetentionDays),
	}, nil
}

func (s *serviceImpl) getRetentionPolicy(ctx context.Context, policyNo string) (*RetentionPolicy, error) {
	var row *retentionRow
	if err := s.database.GetScan(ctx, &row, `SELECT policy_no, data_type_code, version, retention_days, status_code, created_by_user_id, created_at FROM plugin_idc_compliance_privacy_retention_policy WHERE tenant_id = ? AND policy_no = ?`, idccap.DefaultTenantID, policyNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	items := projectRetentionRows([]retentionRow{*row})
	return &items[0], nil
}

func projectRetentionRows(rows []retentionRow) []RetentionPolicy {
	items := make([]RetentionPolicy, 0, len(rows))
	for _, row := range rows {
		items = append(items, RetentionPolicy{
			PolicyNo: row.PolicyNo, DataType: DataType(row.DataTypeCode), Version: row.Version,
			RetentionDays: row.RetentionDays, StatusCode: row.StatusCode,
			CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.UTC(),
		})
	}
	return items
}
