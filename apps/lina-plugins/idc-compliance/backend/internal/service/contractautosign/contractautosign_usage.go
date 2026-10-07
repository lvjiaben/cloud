package contractautosign

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/dialect"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *serviceImpl) Evaluate(ctx context.Context, input DecisionInput) (*Decision, error) {
	input.SourceCode = strings.TrimSpace(input.SourceCode)
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if (input.SourceCode != SourceRequired && input.SourceCode != SourceOptional) ||
		!templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		!currencyPattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	status, err := s.gateway.TencentOnboardingStatus(ctx)
	if err != nil || status.State != "active" || !autoSignAvailable(status) {
		return nil, bizerr.NewCode(CodeCapabilityUnavailable)
	}
	var row *struct {
		AuthorizationNo string `json:"authorization_no"`
		Version         int64  `json:"version"`
	}
	sourceColumn := "auth_row.allow_required_order"
	if input.SourceCode == SourceOptional {
		sourceColumn = "auth_row.allow_manual_optional"
	}
	query := `
SELECT auth_row.authorization_no, auth_row.version
FROM plugin_idc_compliance_auto_sign_auth AS auth_row
JOIN plugin_idc_compliance_auto_sign_auth_template AS relation
  ON relation.tenant_id = auth_row.tenant_id
 AND relation.authorization_no = auth_row.authorization_no
JOIN plugin_idc_compliance_contract_template_version AS template_version
  ON template_version.tenant_id = relation.tenant_id
 AND template_version.template_no = relation.template_no
 AND template_version.version_no = relation.version_no
WHERE auth_row.tenant_id = ? AND auth_row.status_code = 'active'
  AND auth_row.expires_at > CURRENT_TIMESTAMP
  AND auth_row.success_count < auth_row.success_limit
  AND auth_row.currency_code = ? AND auth_row.max_amount_minor >= ?
  AND ` + sourceColumn + ` = TRUE
  AND relation.template_no = ? AND relation.version_no = ?
  AND template_version.status_code = 'enabled'
LIMIT 1`
	if err = s.database.GetScan(ctx, &row, query, idccap.DefaultTenantID, input.CurrencyCode,
		input.AmountMinor, input.TemplateNo, input.VersionNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil || !templateNoPattern.MatchString(row.AuthorizationNo) || row.Version <= 0 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return &Decision{AuthorizationNo: row.AuthorizationNo, Version: row.Version}, nil
}

func (s *serviceImpl) RecordAccepted(ctx context.Context, input AcceptedInput) error {
	input.AuthorizationNo = strings.TrimSpace(input.AuthorizationNo)
	input.ContractNo = strings.TrimSpace(input.ContractNo)
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ProviderAcceptedAt = input.ProviderAcceptedAt.UTC()
	if !templateNoPattern.MatchString(input.AuthorizationNo) || !templateNoPattern.MatchString(input.ContractNo) ||
		!currencyPattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 ||
		input.ProviderAcceptedAt.IsZero() || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return bizerr.NewCode(CodeInvalid)
	}
	digest := sha256Hex("accepted\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex(strings.Join([]string{input.AuthorizationNo, input.ContractNo,
		input.CurrencyCode, stringInt64(input.AmountMinor), input.ProviderAcceptedAt.Format(time.RFC3339Nano)}, "\x00"))
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var replay *struct {
			Fingerprint string `json:"command_fingerprint"`
		}
		if scanErr := tx.Ctx(ctx).GetScan(&replay, `SELECT command_fingerprint FROM plugin_idc_compliance_contract_auto_sign_usage WHERE tenant_id = ? AND command_idempotency_digest = ? FOR UPDATE`, idccap.DefaultTenantID, digest); scanErr != nil {
			return scanErr
		}
		if replay != nil {
			if replay.Fingerprint == fingerprint {
				return nil
			}
			return bizerr.NewCode(CodeConflict)
		}
		var authorization *struct {
			CurrencyCode   string     `json:"currency_code"`
			MaxAmountMinor int64      `json:"max_amount_minor"`
			SuccessLimit   int64      `json:"success_limit"`
			SuccessCount   int64      `json:"success_count"`
			StatusCode     string     `json:"status_code"`
			ExpiresAt      *time.Time `json:"expires_at"`
			Version        int64      `json:"version"`
		}
		if scanErr := tx.Ctx(ctx).GetScan(&authorization, `SELECT currency_code, max_amount_minor, success_limit, success_count, status_code, expires_at, version FROM plugin_idc_compliance_auto_sign_auth WHERE tenant_id = ? AND authorization_no = ? FOR UPDATE`, idccap.DefaultTenantID, input.AuthorizationNo); scanErr != nil {
			return scanErr
		}
		databaseNow, timeErr := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if timeErr != nil {
			return timeErr
		}
		now := databaseNow.Time().UTC()
		if authorization == nil || authorization.StatusCode != StatusActive || authorization.ExpiresAt == nil ||
			!authorization.ExpiresAt.After(now) || authorization.CurrencyCode != input.CurrencyCode ||
			input.AmountMinor > authorization.MaxAmountMinor || authorization.SuccessCount >= authorization.SuccessLimit {
			return bizerr.NewCode(CodeConflict)
		}
		usageNo, createErr := newUsageNo()
		if createErr != nil {
			return createErr
		}
		if _, createErr = tx.Model(do.ContractAutoSignUsage{}).Ctx(ctx).Data(do.ContractAutoSignUsage{
			TenantId: idccap.DefaultTenantID, UsageNo: usageNo, AuthorizationNo: input.AuthorizationNo,
			ContractNo: input.ContractNo, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor,
			CommandIdempotencyDigest: digest, CommandFingerprint: fingerprint,
			ProviderAcceptedAt: &input.ProviderAcceptedAt,
		}).Insert(); createErr != nil {
			return createErr
		}
		result, createErr := tx.Model(do.AutoSignAuth{}).Ctx(ctx).
			Where("tenant_id", idccap.DefaultTenantID).Where("authorization_no", input.AuthorizationNo).
			Where("version", authorization.Version).Where("success_count", authorization.SuccessCount).
			Data(do.AutoSignAuth{SuccessCount: authorization.SuccessCount + 1, Version: authorization.Version + 1}).Update()
		if createErr != nil {
			return createErr
		}
		affected, createErr := result.RowsAffected()
		if createErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || dialect.IsUniqueConstraintViolation(err) {
			return bizerr.NewCode(CodeConflict)
		}
		return bizerr.NewCode(CodeUnavailable)
	}
	return nil
}

func newUsageNo() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "casu_" + hex.EncodeToString(buffer), nil
}

func stringInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
