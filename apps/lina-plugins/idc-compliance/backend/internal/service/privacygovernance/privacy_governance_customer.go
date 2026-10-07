// Customer privacy persistence exposes only the signed-in customer's notices and requests.

package privacygovernance

import (
	"context"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

type customerNoticeRow struct {
	AssessmentTypeCode   string `json:"assessment_type_code"`
	AssessmentNo         string `json:"assessment_no"`
	Version              int64  `json:"version"`
	Purpose              string `json:"purpose"`
	Necessity            string `json:"necessity"`
	Impact               string `json:"impact"`
	RecipientDescription string `json:"recipient_description"`
	StorageLocation      string `json:"storage_location"`
	RetentionDescription string `json:"retention_description"`
	RightsEntry          string `json:"rights_entry"`
	RetentionDays        int    `json:"retention_days"`
}

type customerRequestRow struct {
	RequestNo      string     `json:"request_no"`
	RequestType    string     `json:"request_type_code"`
	StatusCode     string     `json:"status_code"`
	HoldReasonCode string     `json:"hold_reason_code"`
	SubmittedAt    time.Time  `json:"submitted_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	Version        int64      `json:"version"`
}

func (s *serviceImpl) CurrentCustomerPrivacy(ctx context.Context) (*CustomerPrivacyOverview, error) {
	if nilDependency(s.customers) || nilDependency(s.consents) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	consents, err := s.consents.ListCurrentPrivacyConsents(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var notices []customerNoticeRow
	if err = s.database.GetScan(ctx, &notices, `SELECT a.assessment_type_code, a.assessment_no, a.version, a.purpose, a.necessity, a.impact, a.recipient_description, a.storage_location, a.retention_description, a.rights_entry, COALESCE(p.retention_days, 0) AS retention_days FROM plugin_idc_compliance_privacy_assessment a LEFT JOIN plugin_idc_compliance_privacy_retention_policy p ON p.tenant_id = a.tenant_id AND p.status_code = 'active' AND p.data_type_code = CASE a.assessment_type_code WHEN 'personal_verification' THEN 'personal_verification_material' WHEN 'enterprise_verification' THEN 'enterprise_verification_material' WHEN 'electronic_contract' THEN 'electronic_contract_evidence' WHEN 'tax_invoice' THEN 'tax_invoice_evidence' ELSE '' END WHERE a.tenant_id = ? AND a.status_code = 'published' AND a.assessment_type_code IN ('personal_verification', 'enterprise_verification', 'electronic_contract', 'tax_invoice') ORDER BY a.assessment_type_code`, idccap.DefaultTenantID); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	var requests []customerRequestRow
	if err = s.database.GetScan(ctx, &requests, `SELECT request_no, request_type_code, status_code, hold_reason_code, submitted_at, completed_at, version FROM plugin_idc_compliance_privacy_customer_request WHERE tenant_id = ? AND customer_no = ? ORDER BY submitted_at DESC, id DESC LIMIT 100`, idccap.DefaultTenantID, subject.CustomerNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result := &CustomerPrivacyOverview{
		Notices:  make([]CustomerNotice, 0, len(notices)),
		Consents: append([]identitycap.PrivacyConsentDecision(nil), consents...),
		Requests: projectCustomerRequests(requests),
	}
	for _, row := range notices {
		result.Notices = append(result.Notices, CustomerNotice{
			AssessmentType: AssessmentType(row.AssessmentTypeCode), AssessmentNo: row.AssessmentNo,
			Version: row.Version, Purpose: row.Purpose, Necessity: row.Necessity, Impact: row.Impact,
			Recipients: row.RecipientDescription, StorageLocation: row.StorageLocation,
			RetentionDescription: row.RetentionDescription, RetentionDays: row.RetentionDays,
			RightsEntry: row.RightsEntry,
		})
	}
	return result, nil
}

func (s *serviceImpl) WithdrawCurrentCustomerConsent(
	ctx context.Context,
	input identitycap.PrivacyConsentWithdrawalInput,
) (*identitycap.PrivacyConsentDecision, error) {
	if nilDependency(s.customers) || nilDependency(s.consents) || nilDependency(s.customerAudit) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var result *identitycap.PrivacyConsentDecision
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		decision, withdrawErr := s.consents.WithdrawCurrentPrivacyConsent(ctx, input)
		if withdrawErr != nil {
			return withdrawErr
		}
		result = decision
		return s.customerAudit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.consent-withdraw", IdempotencyKey: sha256Hex("privacy-consent-withdraw\x00" + input.IdempotencyKey),
			OperationCode: "compliance.privacy.consent-withdraw", TargetType: "customer_privacy_consent",
			TargetNo: decision.ConsentNo, CustomerNo: subject.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "future_processing_consent_withdrawn", RiskCode: "high",
			BeforeSnapshot: map[string]any{"consent_type": input.ConsentType, "action": "granted"},
			AfterSnapshot:  map[string]any{"consent_type": input.ConsentType, "action": "withdrawn", "historical_evidence_preserved": true},
		})
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) CreateCurrentCustomerRequest(ctx context.Context, input CustomerPrivacyRequestInput) (*CustomerRequest, error) {
	input.RequestType = strings.TrimSpace(input.RequestType)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if (input.RequestType != "export" && input.RequestType != "delete") || !idempotencyPattern.MatchString(input.IdempotencyKey) || nilDependency(s.customers) || nilDependency(s.customerAudit) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	digest := sha256Hex("privacy-customer-request-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex("privacy-customer-request-v1\x00" + subject.CustomerNo + "\x00" + input.RequestType)
	now := s.now().UTC().Truncate(time.Microsecond)
	var requestNo string
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var replays []struct {
			RequestFingerprint string `json:"request_fingerprint"`
			ResultReference    string `json:"result_reference"`
		}
		actionCode := "customer_request_" + input.RequestType
		if loadErr := s.database.GetScan(ctx, &replays, `SELECT request_fingerprint, result_reference FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = ? AND target_no = ? AND idempotency_digest = ?`, idccap.DefaultTenantID, actionCode, subject.CustomerNo, digest); loadErr != nil {
			return loadErr
		}
		if len(replays) == 1 {
			if replays[0].RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			requestNo = replays[0].ResultReference
			return nil
		}
		requestNo, err = newBusinessNo("PCR_")
		if err != nil {
			return err
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_customer_request (tenant_id, request_no, customer_no, request_type_code, status_code, submitted_at, version, created_at, updated_at) VALUES (?, ?, ?, ?, 'submitted', ?, 1, ?, ?)`, idccap.DefaultTenantID, requestNo, subject.CustomerNo, input.RequestType, now, now, now); insertErr != nil {
			return insertErr
		}
		actionNo := "PRA_" + sha256Hex(actionCode + "\x00" + subject.CustomerNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, actionCode, subject.CustomerNo, digest, fingerprint, requestNo, subject.ActorUserID); insertErr != nil {
			return insertErr
		}
		return s.customerAudit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.customer-request", IdempotencyKey: digest,
			OperationCode: "compliance.privacy.customer-request-" + input.RequestType,
			TargetType:    "customer_privacy_request", TargetNo: requestNo, CustomerNo: subject.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "customer_privacy_request_submitted", RiskCode: "high",
			BeforeSnapshot: map[string]any{"status": "missing"},
			AfterSnapshot:  map[string]any{"status": "submitted", "request_type": input.RequestType},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getCustomerRequest(ctx, subject.CustomerNo, requestNo)
}

func (s *serviceImpl) getCustomerRequest(ctx context.Context, customerNo, requestNo string) (*CustomerRequest, error) {
	var rows []customerRequestRow
	if err := s.database.GetScan(ctx, &rows, `SELECT request_no, request_type_code, status_code, hold_reason_code, submitted_at, completed_at, version FROM plugin_idc_compliance_privacy_customer_request WHERE tenant_id = ? AND customer_no = ? AND request_no = ? LIMIT 1`, idccap.DefaultTenantID, customerNo, requestNo); err != nil || len(rows) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	items := projectCustomerRequests(rows)
	return &items[0], nil
}

func projectCustomerRequests(rows []customerRequestRow) []CustomerRequest {
	items := make([]CustomerRequest, 0, len(rows))
	for _, row := range rows {
		items = append(items, CustomerRequest{
			RequestNo: row.RequestNo, RequestType: row.RequestType, StatusCode: row.StatusCode,
			HoldReasonCode: row.HoldReasonCode, SubmittedAt: row.SubmittedAt.UTC(),
			CompletedAt: row.CompletedAt, Version: row.Version,
		})
	}
	return items
}
