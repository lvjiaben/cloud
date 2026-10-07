package billing

import (
	"context"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
)

type renewalServiceDirectoryRow struct {
	ID                         int64      `orm:"id"`
	EntitlementNo              string     `orm:"entitlement_no"`
	ProductNameZhCN            string     `orm:"product_name_zh_cn"`
	ProductNameEnUS            string     `orm:"product_name_en_us"`
	CurrencyCode               string     `orm:"currency_code"`
	EntitlementStatusCode      string     `orm:"entitlement_status_code"`
	RenewalModeCode            string     `orm:"renewal_mode_code"`
	PeriodUnitCode             string     `orm:"period_unit_code"`
	PeriodCount                int        `orm:"period_count"`
	EntitlementExpiresAt       time.Time  `orm:"entitlement_expires_at"`
	EntitlementVersion         int64      `orm:"entitlement_version"`
	PolicyNo                   *string    `orm:"policy_no"`
	PolicyVersion              *int64     `orm:"policy_version"`
	AutoRenewEnabled           *bool      `orm:"auto_renew_enabled"`
	RenewalCycles              *int       `orm:"renewal_cycles"`
	LeadSeconds                *int64     `orm:"lead_seconds"`
	MaximumAttempts            *int       `orm:"maximum_attempts"`
	RetryIntervalSeconds       *int64     `orm:"retry_interval_seconds"`
	FailureNotificationEnabled *bool      `orm:"failure_notification_enabled"`
	LatestAttemptOutcomeCode   string     `orm:"latest_attempt_outcome_code"`
	LatestAttemptFailureCode   string     `orm:"latest_attempt_failure_code"`
	LatestAttemptAt            *time.Time `orm:"latest_attempt_at"`
	NextAttemptAt              *time.Time `orm:"next_attempt_at"`
	UpdatedAt                  time.Time  `orm:"updated_at"`
}

func (s *financeDirectoryImpl) ListAccountRenewalServices(
	ctx context.Context,
	filter RenewalServiceDirectoryFilter,
) (*RenewalServiceDirectoryPage, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	filter.CustomerNo = customerNo
	filter.StatusCode, err = normalizeDirectoryStatus(filter.StatusCode)
	if err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(
		filter.Limit, defaultAccountServiceLimit, maximumAccountServiceLimit,
	)
	if err != nil || len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatusCode string
		Limit                        int
	}{"account_renewal_service", customerNo, filter.StatusCode, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	query := `
SELECT entitlement.id, entitlement.entitlement_no,
	   intent.product_name_zh_cn, intent.product_name_en_us,
	   entitlement.currency_code,
	   entitlement.status_code AS entitlement_status_code,
	   entitlement.renewal_mode_code, entitlement.period_unit_code, entitlement.period_count,
	   entitlement.expires_at AS entitlement_expires_at,
	   entitlement.version AS entitlement_version,
	   policy.policy_no, policy.policy_version, policy.auto_renew_enabled,
       policy.renewal_cycles, policy.lead_seconds, policy.maximum_attempts,
       policy.retry_interval_seconds, policy.failure_notification_enabled,
       COALESCE(attempt.outcome_code, '') AS latest_attempt_outcome_code,
       COALESCE(attempt.failure_code, '') AS latest_attempt_failure_code,
       attempt.created_at AS latest_attempt_at, attempt.next_attempt_at,
	   entitlement.updated_at
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id = entitlement.tenant_id
 AND intent.intent_no = entitlement.source_intent_no
LEFT JOIN plugin_idc_core_service_renewal_policy policy
  ON policy.tenant_id = entitlement.tenant_id
 AND policy.policy_no = entitlement.current_renewal_policy_no
 AND policy.policy_version = entitlement.current_renewal_policy_version
 AND policy.entitlement_no = entitlement.entitlement_no
LEFT JOIN LATERAL (
    SELECT outcome_code, failure_code, created_at, next_attempt_at
    FROM plugin_idc_core_service_renewal_attempt
    WHERE tenant_id = 0 AND entitlement_no = entitlement.entitlement_no
    ORDER BY attempt_sequence DESC, id DESC
    LIMIT 1
) attempt ON TRUE
WHERE entitlement.tenant_id = 0 AND entitlement.customer_no = ?
  AND entitlement.renewal_mode_code IN ('provider_confirmed', 'local_entitlement')
  AND entitlement.charge_type_code <> 'one_time'
  AND entitlement.period_unit_code <> 'once'
  AND entitlement.expires_at IS NOT NULL
  AND entitlement.status_code IN ('active', 'expired')`
	arguments := []any{customerNo}
	if filter.StatusCode != "" {
		query += ` AND entitlement.status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((entitlement.updated_at < ?) OR (entitlement.updated_at = ? AND entitlement.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY entitlement.updated_at DESC, entitlement.id DESC LIMIT ?`
	arguments = append(arguments, filter.Limit+1)
	rows := make([]renewalServiceDirectoryRow, 0, filter.Limit+1)
	if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &RenewalServiceDirectoryPage{
		Services: make([]RenewalServiceDirectoryItem, 0, len(rows)),
	}
	for _, row := range rows {
		if row.ID < 1 || row.EntitlementVersion < 1 || row.EntitlementExpiresAt.IsZero() || row.UpdatedAt.IsZero() {
			return nil, bizerr.NewCode(CodeDirectoryIntegrity)
		}
		item := RenewalServiceDirectoryItem{
			EntitlementNo: row.EntitlementNo, ProductNameZhCN: row.ProductNameZhCN,
			ProductNameEnUS: row.ProductNameEnUS,
			CurrencyCode:    row.CurrencyCode, EntitlementStatusCode: row.EntitlementStatusCode,
			RenewalModeCode: row.RenewalModeCode,
			PeriodUnitCode:  row.PeriodUnitCode, PeriodCount: row.PeriodCount,
			EntitlementExpiresAt: row.EntitlementExpiresAt.UTC(), EntitlementVersion: row.EntitlementVersion,
			LatestAttemptOutcomeCode: row.LatestAttemptOutcomeCode,
			LatestAttemptFailureCode: row.LatestAttemptFailureCode,
			LatestAttemptAt:          row.LatestAttemptAt, NextAttemptAt: row.NextAttemptAt,
			CanConfigureAutomaticRenewal: renewableServiceCanConfigure(row),
		}
		if row.PolicyNo != nil && row.PolicyVersion != nil && row.AutoRenewEnabled != nil &&
			row.RenewalCycles != nil && row.LeadSeconds != nil && row.MaximumAttempts != nil &&
			row.RetryIntervalSeconds != nil && row.FailureNotificationEnabled != nil {
			item.PolicyNo = *row.PolicyNo
			item.PolicyVersion = *row.PolicyVersion
			item.AutoRenewEnabled = *row.AutoRenewEnabled
			item.RenewalCycles = *row.RenewalCycles
			item.LeadSeconds = *row.LeadSeconds
			item.MaximumAttempts = *row.MaximumAttempts
			item.RetryIntervalSeconds = *row.RetryIntervalSeconds
			item.FailureNotificationEnabled = *row.FailureNotificationEnabled
		}
		page.Services = append(page.Services, item)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
			SortTimeMicroseconds: last.UpdatedAt.UnixMicro(), ID: last.ID,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
		}
	}
	return page, nil
}

func renewableServiceCanConfigure(row renewalServiceDirectoryRow) bool {
	if row.PeriodUnitCode == "once" {
		return false
	}
	if row.RenewalModeCode != "local_entitlement" && row.RenewalModeCode != "provider_confirmed" {
		return false
	}
	return row.EntitlementStatusCode == "active" || row.EntitlementStatusCode == "expired"
}
