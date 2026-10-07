// This file atomically reserves, consumes, and releases campaign and asset quota.

package marketing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type reservationRow struct {
	ReservationNo         string     `orm:"reservation_no"`
	CampaignNo            string     `orm:"campaign_no"`
	BenefitNo             string     `orm:"benefit_no"`
	CustomerNo            string     `orm:"customer_no"`
	QuoteReference        string     `orm:"quote_reference"`
	RequestSnapshotDigest string     `orm:"request_snapshot_digest"`
	CurrencyCode          string     `orm:"currency_code"`
	QuotedValueMinor      int64      `orm:"quoted_value_minor"`
	BenefitSnapshotDigest string     `orm:"benefit_snapshot_digest"`
	StatusCode            string     `orm:"status_code"`
	ExpiresAt             *time.Time `orm:"expires_at"`
	AssetNo               *string    `orm:"asset_no"`
	BenefitType           string     `orm:"benefit_type_code"`
	StackPolicy           string     `orm:"stack_policy_code"`
	StackGroup            string     `orm:"stack_group_code"`
	Priority              int        `orm:"priority"`
	GlobalQuotaLimit      *int64     `orm:"global_quota_limit"`
	PerCustomerQuotaLimit *int       `orm:"per_customer_quota_limit"`
}

type campaignQuota struct {
	global      *int64
	perCustomer *int
}

func (s *serviceImpl) Reserve(ctx context.Context, input EvaluateInput) (*Reservation, error) {
	normalized, err := normalizeEvaluateInput(input)
	if err != nil {
		return nil, err
	}
	requestDigest, err := promotionRequestDigest(normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var result *Reservation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		existing, loadErr := s.loadReservation(ctx, normalized.CustomerNo, normalized.QuoteReference, false)
		if loadErr != nil {
			return loadErr
		}
		if len(existing) > 0 {
			result, loadErr = reuseReservation(existing, requestDigest)
			return loadErr
		}
		evaluation, evaluateErr := s.evaluate(ctx, normalized)
		if evaluateErr != nil {
			return evaluateErr
		}
		if len(evaluation.Applied) == 0 {
			result = &Reservation{
				CustomerNo: normalized.CustomerNo, QuoteReference: normalized.QuoteReference,
				CurrencyCode: normalized.CurrencyCode, Status: ReservationStatusActive,
				Applied: []AppliedBenefit{},
			}
			return nil
		}
		quotas, lockErr := s.lockSelectedBenefits(ctx, evaluation.Applied)
		if lockErr != nil {
			return lockErr
		}
		// A concurrent request with the same immutable quote may have committed while
		// this transaction waited on campaign locks. Re-read before touching counters.
		existing, loadErr = s.loadReservation(ctx, normalized.CustomerNo, normalized.QuoteReference, false)
		if loadErr != nil {
			return loadErr
		}
		if len(existing) > 0 {
			result, loadErr = reuseReservation(existing, requestDigest)
			return loadErr
		}
		lockedEvaluation, evaluateErr := s.evaluate(ctx, normalized)
		if evaluateErr != nil {
			return evaluateErr
		}
		if !sameAppliedBenefits(evaluation.Applied, lockedEvaluation.Applied) {
			return bizerr.NewCode(CodeConflict)
		}
		evaluation = lockedEvaluation
		now, nowErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if nowErr != nil {
			return gerror.Wrap(nowErr, "read reservation database time")
		}
		expiresAt := now.Time().Add(s.reservationTTL)
		for _, applied := range evaluation.Applied {
			if _, reserveErr := s.reserveApplied(
				ctx, normalized, requestDigest, expiresAt, applied, quotas[applied.CampaignNo],
			); reserveErr != nil {
				return reserveErr
			}
		}
		result = &Reservation{
			CustomerNo: normalized.CustomerNo, QuoteReference: normalized.QuoteReference,
			CurrencyCode: normalized.CurrencyCode, Status: ReservationStatusActive,
			ExpiresAt: expiresAt, Applied: evaluation.Applied,
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) Consume(
	ctx context.Context,
	customerNo string,
	quoteReference string,
) (*Reservation, error) {
	return s.transition(ctx, customerNo, quoteReference, ReservationStatusConsumed)
}

func (s *serviceImpl) Release(
	ctx context.Context,
	customerNo string,
	quoteReference string,
) (*Reservation, error) {
	return s.transition(ctx, customerNo, quoteReference, ReservationStatusReleased)
}

// ReverseConsumed compensates a quote-wide consumption only when the order
// never received payment. The commerce owner controls that eligibility and
// calls this method inside the same cancellation transaction.
func (s *serviceImpl) ReverseConsumed(
	ctx context.Context,
	customerNo string,
	quoteReference string,
) (*Reservation, error) {
	customerNo = strings.TrimSpace(customerNo)
	quoteReference = strings.TrimSpace(quoteReference)
	if !identifierPattern.MatchString(customerNo) || !identifierPattern.MatchString(quoteReference) ||
		len(quoteReference) < 8 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *Reservation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		rows, loadErr := s.loadReservation(ctx, customerNo, quoteReference, true)
		if loadErr != nil {
			return loadErr
		}
		if len(rows) == 0 {
			return bizerr.NewCode(CodeConflict)
		}
		allConsumed, allReversed := true, true
		for _, row := range rows {
			allConsumed = allConsumed && row.StatusCode == ReservationStatusConsumed
			allReversed = allReversed && row.StatusCode == ReservationStatusReversed
		}
		if allReversed {
			result, loadErr = projectReservation(rows)
			if result != nil {
				result.Reused = true
			}
			return loadErr
		}
		if !allConsumed {
			return bizerr.NewCode(CodeConflict)
		}
		nowValue, nowErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if nowErr != nil {
			return gerror.Wrap(nowErr, "read promotion reversal database time")
		}
		if reverseErr := s.reverseConsumedRows(ctx, rows, nowValue.Time()); reverseErr != nil {
			return reverseErr
		}
		rows, loadErr = s.loadReservation(ctx, customerNo, quoteReference, false)
		if loadErr != nil {
			return loadErr
		}
		result, loadErr = projectReservation(rows)
		return loadErr
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) transition(
	ctx context.Context,
	customerNo string,
	quoteReference string,
	target string,
) (*Reservation, error) {
	customerNo = strings.TrimSpace(customerNo)
	quoteReference = strings.TrimSpace(quoteReference)
	if !identifierPattern.MatchString(customerNo) || !identifierPattern.MatchString(quoteReference) ||
		len(quoteReference) < 8 || (target != ReservationStatusConsumed && target != ReservationStatusReleased) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *Reservation
	expiredDuringConsume := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		rows, loadErr := s.loadReservation(ctx, customerNo, quoteReference, true)
		if loadErr != nil {
			return loadErr
		}
		if len(rows) == 0 {
			return bizerr.NewCode(CodeConflict)
		}
		allTarget := true
		allActive := true
		for _, row := range rows {
			allTarget = allTarget && row.StatusCode == target
			allActive = allActive && row.StatusCode == ReservationStatusActive
		}
		if allTarget {
			result, loadErr = projectReservation(rows)
			if result != nil {
				result.Reused = true
			}
			return loadErr
		}
		if !allActive {
			return bizerr.NewCode(CodeConflict)
		}
		nowValue, nowErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if nowErr != nil {
			return gerror.Wrap(nowErr, "read promotion transition database time")
		}
		now := nowValue.Time()
		effectiveTarget := target
		for _, row := range rows {
			if row.ExpiresAt == nil || !row.ExpiresAt.After(now) {
				effectiveTarget = ReservationStatusExpired
				break
			}
		}
		if transitionErr := s.transitionRows(ctx, rows, effectiveTarget, now); transitionErr != nil {
			return transitionErr
		}
		rows, loadErr = s.loadReservation(ctx, customerNo, quoteReference, false)
		if loadErr != nil {
			return loadErr
		}
		result, loadErr = projectReservation(rows)
		if loadErr != nil {
			return loadErr
		}
		if target == ReservationStatusConsumed && effectiveTarget == ReservationStatusExpired {
			expiredDuringConsume = true
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if expiredDuringConsume {
		return result, bizerr.NewCode(CodeReservationExpired)
	}
	return result, nil
}

func (s *serviceImpl) loadReservation(
	ctx context.Context,
	customerNo string,
	quoteReference string,
	lock bool,
) ([]reservationRow, error) {
	query := reservationProjectionSQL
	if lock {
		query += ` FOR UPDATE OF r`
	}
	var rows []reservationRow
	if err := s.database.GetScan(ctx, &rows, query, customerNo, quoteReference); err != nil {
		return nil, gerror.Wrap(err, "load promotion reservation")
	}
	return rows, nil
}

func reuseReservation(rows []reservationRow, requestDigest string) (*Reservation, error) {
	for _, row := range rows {
		if row.RequestSnapshotDigest != requestDigest {
			return nil, bizerr.NewCode(CodeConflict)
		}
	}
	result, err := projectReservation(rows)
	if result != nil {
		result.Reused = true
	}
	return result, err
}

func projectReservation(rows []reservationRow) (*Reservation, error) {
	if len(rows) == 0 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	first := rows[0]
	if first.ExpiresAt == nil {
		return nil, gerror.New("persisted promotion reservation has no expiry")
	}
	result := &Reservation{
		CustomerNo: first.CustomerNo, QuoteReference: first.QuoteReference,
		CurrencyCode: first.CurrencyCode, Status: first.StatusCode, ExpiresAt: *first.ExpiresAt,
		Applied: make([]AppliedBenefit, 0, len(rows)),
	}
	for _, row := range rows {
		if row.CustomerNo != result.CustomerNo || row.QuoteReference != result.QuoteReference ||
			row.CurrencyCode != result.CurrencyCode || row.StatusCode != result.Status ||
			row.ExpiresAt == nil || !row.ExpiresAt.Equal(result.ExpiresAt) {
			return nil, gerror.New("persisted quote reservation set is inconsistent")
		}
		assetNo := ""
		if row.AssetNo != nil {
			assetNo = *row.AssetNo
		}
		result.Applied = append(result.Applied, AppliedBenefit{
			CampaignNo: row.CampaignNo, BenefitNo: row.BenefitNo, AssetNo: assetNo,
			BenefitType: row.BenefitType, StackPolicy: row.StackPolicy,
			StackGroup: row.StackGroup, Priority: row.Priority,
			DiscountAmountMinor:   row.QuotedValueMinor,
			BenefitSnapshotDigest: row.BenefitSnapshotDigest,
		})
	}
	sort.Slice(result.Applied, func(left, right int) bool {
		if result.Applied[left].Priority != result.Applied[right].Priority {
			return result.Applied[left].Priority < result.Applied[right].Priority
		}
		return result.Applied[left].CampaignNo < result.Applied[right].CampaignNo
	})
	return result, nil
}

func sameAppliedBenefits(left []AppliedBenefit, right []AppliedBenefit) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (s *serviceImpl) lockSelectedBenefits(
	ctx context.Context,
	applied []AppliedBenefit,
) (map[string]campaignQuota, error) {
	placeholders := make([]string, len(applied))
	arguments := make([]any, 0, len(applied)*2)
	for index, item := range applied {
		placeholders[index] = `(?, ?)`
		arguments = append(arguments, item.CampaignNo, item.BenefitNo)
	}
	query := `
SELECT c.campaign_no, c.global_quota_limit, c.per_customer_quota_limit
FROM plugin_idc_core_marketing_campaign c
JOIN plugin_idc_core_marketing_benefit_rule b ON b.campaign_no = c.campaign_no
WHERE (c.campaign_no, b.benefit_no) IN (` + strings.Join(placeholders, ",") + `)
ORDER BY c.campaign_no ASC, b.benefit_no ASC
FOR UPDATE OF c, b`
	var locked []struct {
		CampaignNo            string `orm:"campaign_no"`
		GlobalQuotaLimit      *int64 `orm:"global_quota_limit"`
		PerCustomerQuotaLimit *int   `orm:"per_customer_quota_limit"`
	}
	if err := s.database.GetScan(ctx, &locked, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "lock selected promotion benefits")
	}
	if len(locked) != len(applied) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	quotas := make(map[string]campaignQuota, len(locked))
	for _, row := range locked {
		quotas[row.CampaignNo] = campaignQuota{
			global: row.GlobalQuotaLimit, perCustomer: row.PerCustomerQuotaLimit,
		}
	}
	return quotas, nil
}

func (s *serviceImpl) reserveApplied(
	ctx context.Context,
	input EvaluateInput,
	requestDigest string,
	expiresAt time.Time,
	applied AppliedBenefit,
	quota campaignQuota,
) (string, error) {
	if err := incrementCampaignQuota(
		ctx, s.database, applied.CampaignNo, input.CustomerNo, quota,
	); err != nil {
		return "", err
	}
	if applied.AssetNo != "" {
		result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET reserved_uses = reserved_uses + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND asset_no = ? AND customer_no = ? AND currency_code = ?
  AND campaign_no = ? AND benefit_no = ? AND status_code = 'active'
  AND valid_from <= CURRENT_TIMESTAMP AND valid_until > CURRENT_TIMESTAMP
  AND reserved_uses + consumed_uses < usage_limit`,
			applied.AssetNo, input.CustomerNo, input.CurrencyCode, applied.CampaignNo, applied.BenefitNo,
		)
		if err != nil {
			return "", gerror.Wrap(err, "reserve promotion asset quota")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return "", gerror.Wrap(affectedErr, "read promotion asset reservation result")
			}
			return "", bizerr.NewCode(CodeQuotaUnavailable)
		}
	}
	reservationNo, err := newReservationNumber(s.random)
	if err != nil {
		return "", err
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_campaign_reservation (
    tenant_id, reservation_no, campaign_no, benefit_no, customer_no, quote_reference,
    request_snapshot_digest, currency_code, reserved_units, quoted_value_minor,
    benefit_snapshot_digest, status_code, expires_at
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, 'active', ?)`,
		reservationNo, applied.CampaignNo, applied.BenefitNo, input.CustomerNo,
		input.QuoteReference, requestDigest, input.CurrencyCode, applied.DiscountAmountMinor,
		applied.BenefitSnapshotDigest, expiresAt,
	)
	if err != nil {
		return "", gerror.Wrap(err, "insert campaign quota reservation")
	}
	if applied.AssetNo != "" {
		_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_asset_reservation (
    tenant_id, reservation_no, asset_no, customer_no, quote_reference, currency_code,
    reserved_uses, discount_amount_minor, status_code, expires_at
) VALUES (0, ?, ?, ?, ?, ?, 1, ?, 'active', ?)`,
			reservationNo, applied.AssetNo, input.CustomerNo, input.QuoteReference,
			input.CurrencyCode, applied.DiscountAmountMinor, expiresAt,
		)
		if err != nil {
			return "", gerror.Wrap(err, "insert promotion asset reservation")
		}
	}
	return reservationNo, nil
}

func incrementCampaignQuota(
	ctx context.Context,
	database gdb.DB,
	campaignNo string,
	customerNo string,
	quota campaignQuota,
) error {
	if quota.global != nil {
		result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_campaign
SET global_reserved_count = global_reserved_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ?
  AND global_reserved_count + global_consumed_count < global_quota_limit`, campaignNo)
		if err != nil {
			return gerror.Wrap(err, "reserve global campaign quota")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return gerror.Wrap(affectedErr, "read global campaign quota result")
			}
			return bizerr.NewCode(CodeQuotaUnavailable)
		}
	}
	if quota.perCustomer != nil {
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_customer_quota (
    tenant_id, campaign_no, customer_no, quota_limit
) VALUES (0, ?, ?, ?) ON CONFLICT (campaign_no, customer_no) DO NOTHING`,
			campaignNo, customerNo, *quota.perCustomer,
		); err != nil {
			return gerror.Wrap(err, "initialize customer campaign quota")
		}
		result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_customer_quota
SET reserved_count = reserved_count + 1, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ? AND customer_no = ?
  AND quota_limit = ? AND reserved_count + consumed_count < quota_limit`,
			campaignNo, customerNo, *quota.perCustomer,
		)
		if err != nil {
			return gerror.Wrap(err, "reserve customer campaign quota")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return gerror.Wrap(affectedErr, "read customer campaign quota result")
			}
			return bizerr.NewCode(CodeQuotaUnavailable)
		}
	}
	return nil
}

func (s *serviceImpl) transitionRows(
	ctx context.Context,
	rows []reservationRow,
	target string,
	now time.Time,
) error {
	for _, row := range rows {
		consume := target == ReservationStatusConsumed
		if err := transitionCampaignCounters(
			ctx,
			s.database,
			row.CampaignNo,
			row.CustomerNo,
			row.GlobalQuotaLimit != nil,
			row.PerCustomerQuotaLimit != nil,
			consume,
		); err != nil {
			return err
		}
		if row.AssetNo != nil {
			if err := transitionAssetCounters(ctx, s.database, *row.AssetNo, consume); err != nil {
				return err
			}
		}
		timestampColumn := "released_at"
		if consume {
			timestampColumn = "consumed_at"
		}
		result, err := s.database.Exec(ctx, fmt.Sprintf(`
UPDATE plugin_idc_core_marketing_campaign_reservation
SET status_code = ?, %s = ?, updated_at = ?
WHERE tenant_id = 0 AND reservation_no = ? AND status_code = 'active'`, timestampColumn),
			target, now, now, row.ReservationNo,
		)
		if err != nil {
			return gerror.Wrap(err, "transition campaign reservation")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return gerror.Wrap(affectedErr, "read campaign reservation transition result")
			}
			return bizerr.NewCode(CodeConflict)
		}
		if row.AssetNo != nil {
			assetReservationResult, assetReservationErr := s.database.Exec(ctx, fmt.Sprintf(`
UPDATE plugin_idc_core_marketing_asset_reservation
SET status_code = ?, %s = ?, updated_at = ?
WHERE tenant_id = 0 AND reservation_no = ? AND status_code = 'active'`, timestampColumn),
				target, now, now, row.ReservationNo,
			)
			if assetReservationErr != nil {
				return gerror.Wrap(assetReservationErr, "transition promotion asset reservation")
			}
			if err = requireOneAffected(assetReservationResult, "transition promotion asset reservation"); err != nil {
				return err
			}
		}
	}
	return nil
}

func transitionCampaignCounters(
	ctx context.Context,
	database gdb.DB,
	campaignNo string,
	customerNo string,
	hasGlobalQuota bool,
	hasCustomerQuota bool,
	consume bool,
) error {
	var campaignResult sql.Result
	var err error
	if consume {
		campaignResult, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_campaign
SET global_reserved_count = CASE WHEN global_quota_limit IS NULL THEN 0 ELSE global_reserved_count - 1 END,
    global_consumed_count = CASE WHEN global_quota_limit IS NULL THEN 0 ELSE global_consumed_count + 1 END,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ?
  AND ((? = TRUE AND global_quota_limit IS NOT NULL AND global_reserved_count > 0)
       OR (? = FALSE AND global_quota_limit IS NULL))`,
			campaignNo, hasGlobalQuota, hasGlobalQuota,
		)
		if err != nil {
			return gerror.Wrap(err, "consume global campaign quota")
		}
		if err = requireOneAffected(campaignResult, "consume global campaign quota"); err != nil {
			return err
		}
		if !hasCustomerQuota {
			return nil
		}
		customerResult, customerErr := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_customer_quota
SET reserved_count = reserved_count - 1, consumed_count = consumed_count + 1,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ? AND customer_no = ? AND reserved_count > 0`,
			campaignNo, customerNo,
		)
		if customerErr != nil {
			return gerror.Wrap(customerErr, "consume customer campaign quota")
		}
		return requireOneAffected(customerResult, "consume customer campaign quota")
	}
	campaignResult, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_campaign
SET global_reserved_count = CASE WHEN global_quota_limit IS NULL THEN 0 ELSE global_reserved_count - 1 END,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ?
  AND ((? = TRUE AND global_quota_limit IS NOT NULL AND global_reserved_count > 0)
       OR (? = FALSE AND global_quota_limit IS NULL))`,
		campaignNo, hasGlobalQuota, hasGlobalQuota,
	)
	if err != nil {
		return gerror.Wrap(err, "release global campaign quota")
	}
	if err = requireOneAffected(campaignResult, "release global campaign quota"); err != nil {
		return err
	}
	if !hasCustomerQuota {
		return nil
	}
	customerResult, customerErr := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_customer_quota
SET reserved_count = reserved_count - 1, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ? AND customer_no = ? AND reserved_count > 0`,
		campaignNo, customerNo,
	)
	if customerErr != nil {
		return gerror.Wrap(customerErr, "release customer campaign quota")
	}
	return requireOneAffected(customerResult, "release customer campaign quota")
}

func transitionAssetCounters(ctx context.Context, database gdb.DB, assetNo string, consume bool) error {
	if consume {
		result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET reserved_uses = reserved_uses - 1, consumed_uses = consumed_uses + 1,
    status_code = CASE
      WHEN reserved_uses - 1 = 0 AND consumed_uses + 1 = usage_limit THEN 'exhausted'
      ELSE 'active' END,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND asset_no = ? AND status_code = 'active' AND reserved_uses > 0`, assetNo)
		if err != nil {
			return gerror.Wrap(err, "consume promotion asset quota")
		}
		return requireOneAffected(result, "consume promotion asset quota")
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET reserved_uses = reserved_uses - 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND asset_no = ? AND status_code = 'active' AND reserved_uses > 0`, assetNo)
	if err != nil {
		return gerror.Wrap(err, "release promotion asset quota")
	}
	return requireOneAffected(result, "release promotion asset quota")
}

func (s *serviceImpl) reverseConsumedRows(
	ctx context.Context,
	rows []reservationRow,
	now time.Time,
) error {
	for _, row := range rows {
		if err := reverseCampaignCounters(
			ctx, s.database, row.CampaignNo, row.CustomerNo,
			row.GlobalQuotaLimit != nil, row.PerCustomerQuotaLimit != nil,
		); err != nil {
			return err
		}
		if row.AssetNo != nil {
			if err := reverseAssetCounter(ctx, s.database, *row.AssetNo); err != nil {
				return err
			}
		}
		result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_campaign_reservation
SET status_code = 'reversed', reversed_at = ?, updated_at = ?
WHERE tenant_id = 0 AND reservation_no = ? AND status_code = 'consumed'`,
			now, now, row.ReservationNo,
		)
		if err != nil {
			return gerror.Wrap(err, "reverse campaign reservation")
		}
		if err = requireOneAffected(result, "reverse campaign reservation"); err != nil {
			return err
		}
		if row.AssetNo == nil {
			continue
		}
		assetResult, assetErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_asset_reservation
SET status_code = 'reversed', reversed_at = ?, updated_at = ?
WHERE tenant_id = 0 AND reservation_no = ? AND status_code = 'consumed'`,
			now, now, row.ReservationNo,
		)
		if assetErr != nil {
			return gerror.Wrap(assetErr, "reverse promotion asset reservation")
		}
		if err = requireOneAffected(assetResult, "reverse promotion asset reservation"); err != nil {
			return err
		}
	}
	return nil
}

func reverseCampaignCounters(
	ctx context.Context,
	database gdb.DB,
	campaignNo string,
	customerNo string,
	hasGlobalQuota bool,
	hasCustomerQuota bool,
) error {
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_campaign
SET global_consumed_count = CASE
      WHEN global_quota_limit IS NULL THEN 0 ELSE global_consumed_count - 1 END,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ?
  AND ((? = TRUE AND global_quota_limit IS NOT NULL AND global_consumed_count > 0)
       OR (? = FALSE AND global_quota_limit IS NULL))`,
		campaignNo, hasGlobalQuota, hasGlobalQuota,
	)
	if err != nil {
		return gerror.Wrap(err, "reverse global campaign quota")
	}
	if err = requireOneAffected(result, "reverse global campaign quota"); err != nil {
		return err
	}
	if !hasCustomerQuota {
		return nil
	}
	result, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_customer_quota
SET consumed_count = consumed_count - 1, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND campaign_no = ? AND customer_no = ? AND consumed_count > 0`,
		campaignNo, customerNo,
	)
	if err != nil {
		return gerror.Wrap(err, "reverse customer campaign quota")
	}
	return requireOneAffected(result, "reverse customer campaign quota")
}

func reverseAssetCounter(ctx context.Context, database gdb.DB, assetNo string) error {
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET consumed_uses = consumed_uses - 1,
    status_code = CASE
      WHEN status_code = 'exhausted' AND valid_until > CURRENT_TIMESTAMP THEN 'active'
      ELSE status_code END,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND asset_no = ?
  AND status_code IN ('active', 'exhausted', 'expired', 'revoked')
  AND consumed_uses > 0`, assetNo)
	if err != nil {
		return gerror.Wrap(err, "reverse promotion asset quota")
	}
	return requireOneAffected(result, "reverse promotion asset quota")
}

func requireOneAffected(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read "+operation+" result")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func promotionRequestDigest(input EvaluateInput) (string, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", gerror.Wrap(err, "encode promotion reservation request")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func newReservationNumber(random io.Reader) (string, error) {
	raw := make([]byte, 16)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", gerror.Wrap(err, "generate promotion reservation number")
	}
	return "mres_" + hex.EncodeToString(raw), nil
}

const reservationProjectionSQL = `
SELECT r.reservation_no, r.campaign_no, r.benefit_no, r.customer_no,
       r.quote_reference, r.request_snapshot_digest, r.currency_code,
       r.quoted_value_minor, r.benefit_snapshot_digest, r.status_code, r.expires_at,
       ar.asset_no, b.benefit_type_code, c.stack_policy_code, c.stack_group_code, c.priority
       , c.global_quota_limit, c.per_customer_quota_limit
FROM plugin_idc_core_marketing_campaign_reservation r
JOIN plugin_idc_core_marketing_campaign c ON c.campaign_no = r.campaign_no
JOIN plugin_idc_core_marketing_benefit_rule b
  ON b.campaign_no = r.campaign_no AND b.benefit_no = r.benefit_no
LEFT JOIN plugin_idc_core_marketing_asset_reservation ar
  ON ar.reservation_no = r.reservation_no
WHERE r.tenant_id = 0 AND r.customer_no = ? AND r.quote_reference = ?
ORDER BY r.campaign_no ASC, r.id ASC`
