// This file owns currency-safe marketing performance facts and staff-reviewed abuse cases.

package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	permissionMarketingPerformance = "idc:admin:marketing:performance:list"
	permissionMarketingAbuseList   = "idc:admin:marketing:abuse:list"
	permissionMarketingAbuseReview = "idc:admin:marketing:abuse:review"
	permissionMarketingCostList    = "idc:admin:marketing:cost:missing:list"
	permissionMarketingCostCorrect = "idc:admin:marketing:cost:correct"
	permissionMarketingNewcomerSet = "idc:admin:marketing:campaign:newcomer:update"
	maximumMarketingReportRange    = 366 * 24 * time.Hour
	maximumMarketingAbusePageSize  = 100
)

var (
	CodeInsightsInvalid = bizerr.MustDefine(
		"IDC_MARKETING_INSIGHTS_INVALID", "The marketing insights request is invalid", gcode.CodeInvalidParameter,
	)
	CodeInsightsNotAccessible = bizerr.MustDefine(
		"IDC_MARKETING_INSIGHTS_NOT_ACCESSIBLE", "The marketing insights data is not accessible", gcode.CodeNotAuthorized,
	)
	CodeInsightsConflict = bizerr.MustDefine(
		"IDC_MARKETING_INSIGHTS_CONFLICT", "The marketing abuse case has changed", gcode.CodeInvalidOperation,
	)
	CodeInsightsUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_INSIGHTS_UNAVAILABLE", "The marketing insights service is temporarily unavailable", gcode.CodeInternalError,
	)
)

// MarketingInsightsAuthorizer keeps report, queue and review permissions separate.
type MarketingInsightsAuthorizer interface {
	AuthorizeMarketingPerformance(context.Context) (int, error)
	AuthorizeMarketingAbuseList(context.Context) (int, error)
	AuthorizeMarketingAbuseReview(context.Context) (int, error)
	AuthorizeMarketingCostList(context.Context) (int, error)
	AuthorizeMarketingCostCorrect(context.Context) (int, error)
	AuthorizeMarketingNewcomerSet(context.Context) (int, error)
}

// MarketingInsightsService exposes bounded admin reads and the audited review action.
type MarketingInsightsService interface {
	Performance(context.Context, MarketingPerformanceInput) (*MarketingPerformanceReport, error)
	AbuseCases(context.Context, MarketingAbuseSearchInput) (*MarketingAbusePage, error)
	ReviewAbuse(context.Context, MarketingAbuseReviewInput) (*MarketingAbuseCase, error)
	MissingCosts(context.Context, MarketingMissingCostSearchInput) (*MarketingMissingCostPage, error)
	CorrectCost(context.Context, MarketingCostCorrectionInput) (*MarketingMissingCostItem, error)
	SetCampaignNewcomer(context.Context, MarketingCampaignNewcomerInput) (*MarketingCampaignNewcomerSetting, error)
}

type MarketingPerformanceInput struct {
	StartedAt time.Time
	EndedAt   time.Time
	Page      int
	Limit     int
}

type MarketingPerformanceReport struct {
	StartedAt time.Time
	EndedAt   time.Time
	Rows      []MarketingPerformanceRow
	Total     int
}

type MarketingPerformanceRow struct {
	CampaignNo   string
	NameZhCN     string
	NameEnUS     string
	CurrencyCode string
	Exposure     int64
	Claim        int64
	Use          int64
	Order        int64
	ReceiptMinor int64
	RefundMinor  int64
	CostMinor    int64
	NetMinor     int64
	CostComplete bool
}

type MarketingAbuseSearchInput struct {
	Status string
	Limit  int
	Page   int
}

type MarketingAbusePage struct {
	Cases []MarketingAbuseCase
	Total int
}

type MarketingAbuseCase struct {
	CaseNo              string
	CampaignNo          string
	CustomerNo          string
	SignalCode          string
	StatusCode          string
	RelatedAccountCount int
	EvidenceSummaryCode string
	WindowStartedAt     time.Time
	WindowEndedAt       time.Time
	ObservedCount       int
	BlockedUntil        *time.Time
	ReviewReason        string
	ReviewedBy          int
	ReviewedAt          *time.Time
	Version             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MarketingAbuseReviewInput struct {
	CaseNo          string
	Decision        string
	Reason          string
	ExpectedVersion int64
}

type MarketingMissingCostSearchInput struct{ Limit, Page int }

type MarketingMissingCostPage struct {
	Items []MarketingMissingCostItem
	Total int
}

type MarketingMissingCostItem struct {
	OrderItemNo      string
	OrderNo          string
	VersionNo        string
	CurrencyCode     string
	PeriodUnitCode   string
	PeriodCount      int
	Quantity         int
	PurchaseCycles   int
	ProductNameZhCN  string
	ProductNameEnUS  string
	StatusCode       string
	UnitCostMinor    *int64
	TotalCostMinor   *int64
	CorrectionReason string
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MarketingCostCorrectionInput struct {
	RequestNo       string
	OrderItemNo     string
	UnitCostMinor   int64
	Reason          string
	ExpectedVersion int64
}

type MarketingCampaignNewcomerInput struct {
	RequestNo       string
	CampaignNo      string
	Enabled         bool
	ExpectedVersion int64
}

type MarketingCampaignNewcomerSetting struct {
	CampaignNo string
	Enabled    bool
	Version    int64
	UpdatedAt  time.Time
}

type marketingInsightsServiceImpl struct {
	database   gdb.DB
	authorizer MarketingInsightsAuthorizer
	audit      idccap.BusinessAuditWriter
}

var _ MarketingInsightsService = (*marketingInsightsServiceImpl)(nil)

func NewMarketingInsightsService(database gdb.DB, authorizer MarketingInsightsAuthorizer, audit idccap.BusinessAuditWriter) (MarketingInsightsService, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(authorizer) || isNilMarketingManagementDependency(audit) {
		return nil, gerror.New("marketing insights requires database, authorization, and audit")
	}
	return &marketingInsightsServiceImpl{database: database, authorizer: authorizer, audit: audit}, nil
}

func (s *marketingInsightsServiceImpl) Performance(ctx context.Context, input MarketingPerformanceInput) (*MarketingPerformanceReport, error) {
	if _, err := s.authorizer.AuthorizeMarketingPerformance(ctx); err != nil {
		return nil, err
	}
	if input.StartedAt.IsZero() || input.EndedAt.IsZero() || input.EndedAt.Before(input.StartedAt) || input.EndedAt.Sub(input.StartedAt) > maximumMarketingReportRange {
		return nil, bizerr.NewCode(CodeInsightsInvalid)
	}
	limit, offset, err := marketingInsightsPage(input.Page, input.Limit)
	if err != nil {
		return nil, err
	}
	rows := make([]MarketingPerformanceRow, 0)
	const groupsSQL = `
FROM plugin_idc_core_marketing_effect_fact f
JOIN plugin_idc_core_marketing_campaign c ON c.campaign_no = f.campaign_no
WHERE f.tenant_id = 0 AND f.occurred_at >= ? AND f.occurred_at <= ?
GROUP BY c.id, c.campaign_no, f.currency_code`
	total, err := readMarketingNumberedPage(ctx, s.database, &rows,
		`SELECT COUNT(*) FROM (SELECT c.id, f.currency_code `+groupsSQL+`) grouped`,
		[]any{input.StartedAt.UTC(), input.EndedAt.UTC()}, `
WITH selected_groups AS MATERIALIZED (
 SELECT c.id, c.campaign_no, f.currency_code `+groupsSQL+`
 ORDER BY c.id DESC, f.currency_code ASC NULLS FIRST LIMIT ? OFFSET ?
)
SELECT c.campaign_no, c.name_zh_cn, c.name_en_us,
       COALESCE(f.currency_code, '') AS currency_code,
       count(*) FILTER (WHERE f.event_type_code = 'exposure') AS exposure,
       count(*) FILTER (WHERE f.event_type_code = 'claim') AS claim,
       count(*) FILTER (WHERE f.event_type_code = 'use') AS use,
       count(*) FILTER (WHERE f.event_type_code = 'order') AS "order",
		COALESCE(sum(CASE WHEN f.event_type_code = 'receipt' THEN f.amount_minor WHEN f.event_type_code = 'receipt_reversal' THEN -f.amount_minor ELSE 0 END), 0) AS receipt_minor,
		COALESCE(sum(CASE WHEN f.event_type_code = 'refund' THEN f.amount_minor WHEN f.event_type_code = 'refund_reversal' THEN -f.amount_minor ELSE 0 END), 0) AS refund_minor,
		COALESCE(sum(CASE WHEN f.event_type_code = 'cost' THEN f.amount_minor WHEN f.event_type_code = 'cost_reversal' THEN -f.amount_minor ELSE 0 END), 0) AS cost_minor,
		COALESCE(sum(CASE WHEN f.event_type_code = 'receipt' THEN f.amount_minor WHEN f.event_type_code = 'receipt_reversal' THEN -f.amount_minor ELSE 0 END), 0)
		  - COALESCE(sum(CASE WHEN f.event_type_code = 'refund' THEN f.amount_minor WHEN f.event_type_code = 'refund_reversal' THEN -f.amount_minor ELSE 0 END), 0)
		  - COALESCE(sum(CASE WHEN f.event_type_code = 'cost' THEN f.amount_minor WHEN f.event_type_code = 'cost_reversal' THEN -f.amount_minor ELSE 0 END), 0) AS net_minor,
		NOT EXISTS (
		  SELECT 1 FROM plugin_idc_core_marketing_campaign_reservation reservation
		  JOIN plugin_idc_core_commerce_order order_row ON order_row.quote_no = reservation.quote_reference
		  JOIN plugin_idc_core_commerce_order_cost_snapshot cost ON cost.order_no = order_row.order_no
		  WHERE reservation.campaign_no = c.campaign_no AND order_row.currency_code IS NOT DISTINCT FROM f.currency_code
		    AND order_row.created_at >= ? AND order_row.created_at <= ? AND cost.status_code = 'missing'
		) AS cost_complete
FROM plugin_idc_core_marketing_effect_fact f
JOIN plugin_idc_core_marketing_campaign c ON c.campaign_no = f.campaign_no
JOIN selected_groups selected ON selected.campaign_no = f.campaign_no
 AND selected.currency_code IS NOT DISTINCT FROM f.currency_code
WHERE f.tenant_id = 0 AND f.occurred_at >= ? AND f.occurred_at <= ?
GROUP BY c.id, c.campaign_no, c.name_zh_cn, c.name_en_us, f.currency_code
ORDER BY c.id DESC, f.currency_code ASC NULLS FIRST`,
		[]any{input.StartedAt.UTC(), input.EndedAt.UTC(), limit, offset,
			input.StartedAt.UTC(), input.EndedAt.UTC(), input.StartedAt.UTC(), input.EndedAt.UTC()})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return &MarketingPerformanceReport{StartedAt: input.StartedAt.UTC(), EndedAt: input.EndedAt.UTC(), Rows: rows, Total: total}, nil
}

func (s *marketingInsightsServiceImpl) AbuseCases(ctx context.Context, input MarketingAbuseSearchInput) (*MarketingAbusePage, error) {
	if _, err := s.authorizer.AuthorizeMarketingAbuseList(ctx); err != nil {
		return nil, err
	}
	status := strings.TrimSpace(input.Status)
	if status != "" && status != "pending_review" && status != "confirmed_abuse" && status != "false_positive" {
		return nil, bizerr.NewCode(CodeInsightsInvalid)
	}
	limit, offset, err := marketingInsightsPage(input.Page, input.Limit)
	if err != nil {
		return nil, err
	}
	rows := make([]MarketingAbuseCase, 0, limit)
	query := `SELECT case_no, campaign_no, COALESCE(customer_no, '') AS customer_no, signal_code,
 status_code, related_account_count, evidence_summary_code, window_started_at, window_ended_at,
 observed_count, blocked_until, COALESCE(review_reason, '') AS review_reason,
 COALESCE(reviewed_by, 0) AS reviewed_by, reviewed_at, version, created_at, updated_at
 FROM plugin_idc_core_marketing_abuse_case WHERE tenant_id = 0`
	args := []any{}
	if status != "" {
		query += ` AND status_code = ?`
		args = append(args, status)
	}
	countSQL := `SELECT COUNT(*) FROM (` + query + `) visible_cases`
	countArgs := append([]any(nil), args...)
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	total, err := readMarketingNumberedPage(ctx, s.database, &rows, countSQL, countArgs, query, args)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return &MarketingAbusePage{Cases: rows, Total: total}, nil
}

func (s *marketingInsightsServiceImpl) ReviewAbuse(ctx context.Context, input MarketingAbuseReviewInput) (*MarketingAbuseCase, error) {
	actor, err := s.authorizer.AuthorizeMarketingAbuseReview(ctx)
	if err != nil {
		return nil, err
	}
	input.CaseNo, input.Decision, input.Reason = strings.TrimSpace(input.CaseNo), strings.TrimSpace(input.Decision), strings.TrimSpace(input.Reason)
	if input.CaseNo == "" || (input.Decision != "confirmed_abuse" && input.Decision != "false_positive") || input.ExpectedVersion <= 0 || utf8.RuneCountInString(input.Reason) < 2 || utf8.RuneCountInString(input.Reason) > 500 {
		return nil, bizerr.NewCode(CodeInsightsInvalid)
	}
	var result MarketingAbuseCase
	conflict := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var before MarketingAbuseCase
		if readErr := s.database.GetScan(txCtx, &before, `SELECT case_no, campaign_no, COALESCE(customer_no, '') AS customer_no,
 signal_code, status_code, related_account_count, evidence_summary_code, window_started_at, window_ended_at,
 observed_count, blocked_until, COALESCE(review_reason, '') AS review_reason, COALESCE(reviewed_by, 0) AS reviewed_by,
 reviewed_at, version, created_at, updated_at FROM plugin_idc_core_marketing_abuse_case
 WHERE tenant_id = 0 AND case_no = ? FOR UPDATE`, input.CaseNo); readErr != nil {
			return readErr
		}
		if before.CaseNo == "" || before.StatusCode != "pending_review" || before.Version != input.ExpectedVersion {
			conflict = true
			return bizerr.NewCode(CodeInsightsConflict)
		}
		if _, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_marketing_abuse_case
 SET status_code = ?, review_reason = ?, reviewed_by = ?, reviewed_at = CURRENT_TIMESTAMP,
     version = version + 1, updated_at = CURRENT_TIMESTAMP
 WHERE tenant_id = 0 AND case_no = ? AND status_code = 'pending_review' AND version = ?`, input.Decision, input.Reason, actor, input.CaseNo, input.ExpectedVersion); updateErr != nil {
			return updateErr
		}
		if input.Decision == "confirmed_abuse" && before.CustomerNo != "" && before.CampaignNo != "" {
			if _, restrictionErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_marketing_campaign_restriction
 (tenant_id, campaign_no, customer_no, case_no, status_code) VALUES (0, ?, ?, ?, 'active')
 ON CONFLICT (case_no) DO NOTHING`, before.CampaignNo, before.CustomerNo, before.CaseNo); restrictionErr != nil {
				return restrictionErr
			}
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "marketing.abuse.review", IdempotencyKey: input.CaseNo + ":" + input.Decision,
			OperationCode: "marketing.abuse.review", TargetType: "marketing_abuse_case", TargetNo: input.CaseNo,
			CustomerNo: before.CustomerNo, ResultCode: "succeeded", ReasonCode: "staff_review", RiskCode: "high",
			RequiredPermission: permissionMarketingAbuseReview, CorrelationNo: before.CampaignNo,
			BeforeSnapshot: map[string]any{"status": before.StatusCode, "version": before.Version},
			AfterSnapshot:  map[string]any{"status": input.Decision, "reason": input.Reason},
		}); auditErr != nil {
			return auditErr
		}
		return s.database.GetScan(txCtx, &result, `SELECT case_no, campaign_no, COALESCE(customer_no, '') AS customer_no,
 signal_code, status_code, related_account_count, evidence_summary_code, window_started_at, window_ended_at,
 observed_count, blocked_until, COALESCE(review_reason, '') AS review_reason, COALESCE(reviewed_by, 0) AS reviewed_by,
 reviewed_at, version, created_at, updated_at FROM plugin_idc_core_marketing_abuse_case WHERE case_no = ?`, input.CaseNo)
	})
	if err != nil {
		if conflict {
			return nil, bizerr.NewCode(CodeInsightsConflict)
		}
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return &result, nil
}

func (s *marketingInsightsServiceImpl) MissingCosts(ctx context.Context, input MarketingMissingCostSearchInput) (*MarketingMissingCostPage, error) {
	if _, err := s.authorizer.AuthorizeMarketingCostList(ctx); err != nil {
		return nil, err
	}
	limit, offset, err := marketingInsightsPage(input.Page, input.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]MarketingMissingCostItem, 0, limit)
	const query = `
SELECT cost.order_item_no, cost.order_no, cost.version_no, cost.currency_code,
       cost.period_unit_code, cost.period_count, cost.quantity, cost.purchase_cycles,
       item.product_name_zh_cn, item.product_name_en_us, cost.status_code,
       cost.unit_cost_minor, cost.total_cost_minor, COALESCE(cost.correction_reason, '') AS correction_reason,
       cost.version, cost.created_at, cost.updated_at
FROM plugin_idc_core_commerce_order_cost_snapshot cost
JOIN plugin_idc_core_commerce_order_item item
  ON item.order_no = cost.order_no AND item.order_item_no = cost.order_item_no
WHERE cost.tenant_id = 0 AND cost.status_code = 'missing'
  AND EXISTS (
    SELECT 1 FROM plugin_idc_core_commerce_order order_row
    JOIN plugin_idc_core_marketing_campaign_reservation reservation
      ON reservation.quote_reference = order_row.quote_no AND reservation.status_code = 'consumed'
    WHERE order_row.order_no = cost.order_no
  )`
	total, err := readMarketingNumberedPage(ctx, s.database, &items,
		`SELECT COUNT(*) FROM (`+query+`) missing_costs`, nil,
		query+` ORDER BY cost.created_at ASC, cost.id ASC LIMIT ? OFFSET ?`, []any{limit, offset})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return &MarketingMissingCostPage{Items: items, Total: total}, nil
}

func marketingInsightsPage(page, limit int) (int, int, error) {
	if limit == 0 {
		limit = 25
	}
	if !validMarketingNumberedPage(page, limit, "") {
		return 0, 0, bizerr.NewCode(CodeInsightsInvalid)
	}
	if page < 1 {
		page = 1
	}
	return limit, (page - 1) * limit, nil
}

type marketingCostCorrectionRow struct {
	MarketingMissingCostItem
	CorrectionRequestNo     string `orm:"correction_request_no"`
	CorrectionRequestDigest string `orm:"correction_request_digest"`
}

func (s *marketingInsightsServiceImpl) CorrectCost(ctx context.Context, input MarketingCostCorrectionInput) (*MarketingMissingCostItem, error) {
	actor, err := s.authorizer.AuthorizeMarketingCostCorrect(ctx)
	if err != nil {
		return nil, err
	}
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.OrderItemNo = strings.TrimSpace(input.OrderItemNo)
	input.Reason = strings.TrimSpace(input.Reason)
	if len(input.RequestNo) < 8 || len(input.RequestNo) > 64 || input.OrderItemNo == "" ||
		input.UnitCostMinor < 0 || input.ExpectedVersion < 1 || utf8.RuneCountInString(input.Reason) < 2 ||
		utf8.RuneCountInString(input.Reason) > 500 {
		return nil, bizerr.NewCode(CodeInsightsInvalid)
	}
	requestDigest := marketingFactDigest(input.RequestNo, input.OrderItemNo, input.Reason,
		strconv.FormatInt(input.UnitCostMinor, 10), strconv.FormatInt(input.ExpectedVersion, 10))
	var result MarketingMissingCostItem
	conflict := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var before marketingCostCorrectionRow
		if readErr := s.database.GetScan(txCtx, &before, `
SELECT cost.order_item_no, cost.order_no, cost.version_no, cost.currency_code,
       cost.period_unit_code, cost.period_count, cost.quantity, cost.purchase_cycles,
       item.product_name_zh_cn, item.product_name_en_us, cost.status_code,
       cost.unit_cost_minor, cost.total_cost_minor, COALESCE(cost.correction_reason, '') AS correction_reason,
       COALESCE(cost.correction_request_no, '') AS correction_request_no,
       COALESCE(cost.correction_request_digest, '') AS correction_request_digest,
       cost.version, cost.created_at, cost.updated_at
FROM plugin_idc_core_commerce_order_cost_snapshot cost
JOIN plugin_idc_core_commerce_order_item item
  ON item.order_no = cost.order_no AND item.order_item_no = cost.order_item_no
WHERE cost.tenant_id = 0 AND cost.order_item_no = ? FOR UPDATE OF cost`, input.OrderItemNo); readErr != nil {
			return readErr
		}
		if before.OrderItemNo == "" {
			conflict = true
			return bizerr.NewCode(CodeInsightsConflict)
		}
		if before.StatusCode == "corrected" && before.CorrectionRequestNo == input.RequestNo && before.CorrectionRequestDigest == requestDigest {
			result = before.MarketingMissingCostItem
			return nil
		}
		if before.StatusCode != "missing" || before.Version != input.ExpectedVersion || before.Quantity < 1 || before.PurchaseCycles < 1 {
			conflict = true
			return bizerr.NewCode(CodeInsightsConflict)
		}
		multiplier := int64(before.Quantity) * int64(before.PurchaseCycles)
		if multiplier < 1 || (input.UnitCostMinor > 0 && input.UnitCostMinor > math.MaxInt64/multiplier) {
			return bizerr.NewCode(CodeInsightsInvalid)
		}
		totalCost := input.UnitCostMinor * multiplier
		if _, updateErr := s.database.Exec(txCtx, `
UPDATE plugin_idc_core_commerce_order_cost_snapshot
SET status_code='corrected', unit_cost_minor=?, total_cost_minor=?, source_code='historical_correction',
    source_observed_at=CURRENT_TIMESTAMP, correction_request_no=?, correction_request_digest=?,
    correction_reason=?, corrected_by=?, corrected_at=CURRENT_TIMESTAMP,
    version=version+1, updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND order_item_no=? AND status_code='missing' AND version=?`,
			input.UnitCostMinor, totalCost, input.RequestNo, requestDigest, input.Reason, actor,
			input.OrderItemNo, input.ExpectedVersion); updateErr != nil {
			return updateErr
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "marketing.cost.correct", IdempotencyKey: input.RequestNo,
			OperationCode: "marketing.cost.correct", TargetType: "commerce_order_item", TargetNo: input.OrderItemNo,
			ResultCode: "succeeded", ReasonCode: "historical_cost_correction", RiskCode: "high",
			RequiredPermission: permissionMarketingCostCorrect, CorrelationNo: before.OrderNo,
			BeforeSnapshot: map[string]any{"status": before.StatusCode, "version": before.Version},
			AfterSnapshot: map[string]any{"status": "corrected", "unit_cost_minor": input.UnitCostMinor,
				"total_cost_minor": totalCost, "reason": input.Reason},
		}); auditErr != nil {
			return auditErr
		}
		return s.database.GetScan(txCtx, &result, `
SELECT cost.order_item_no, cost.order_no, cost.version_no, cost.currency_code,
       cost.period_unit_code, cost.period_count, cost.quantity, cost.purchase_cycles,
       item.product_name_zh_cn, item.product_name_en_us, cost.status_code,
       cost.unit_cost_minor, cost.total_cost_minor, COALESCE(cost.correction_reason, '') AS correction_reason,
       cost.version, cost.created_at, cost.updated_at
FROM plugin_idc_core_commerce_order_cost_snapshot cost
JOIN plugin_idc_core_commerce_order_item item
  ON item.order_no = cost.order_no AND item.order_item_no = cost.order_item_no
WHERE cost.tenant_id = 0 AND cost.order_item_no = ?`, input.OrderItemNo)
	})
	if err != nil {
		if conflict {
			return nil, bizerr.NewCode(CodeInsightsConflict)
		}
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return &result, nil
}

type marketingCampaignNewcomerRequestRow struct {
	CampaignNo       string    `orm:"campaign_no"`
	Enabled          bool      `orm:"enabled"`
	ResultingVersion int64     `orm:"resulting_version"`
	ResultingUpdated time.Time `orm:"resulting_updated_at"`
	RequestDigest    string    `orm:"request_digest"`
}

func (s *marketingInsightsServiceImpl) SetCampaignNewcomer(ctx context.Context, input MarketingCampaignNewcomerInput) (*MarketingCampaignNewcomerSetting, error) {
	actor, err := s.authorizer.AuthorizeMarketingNewcomerSet(ctx)
	if err != nil {
		return nil, err
	}
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.CampaignNo = strings.TrimSpace(input.CampaignNo)
	if len(input.RequestNo) < 8 || len(input.RequestNo) > 64 || !identifierPattern.MatchString(input.CampaignNo) || input.ExpectedVersion < 1 {
		return nil, bizerr.NewCode(CodeInsightsInvalid)
	}
	requestDigest := marketingFactDigest(input.RequestNo, input.CampaignNo, strconv.FormatBool(input.Enabled), strconv.FormatInt(input.ExpectedVersion, 10))
	var result MarketingCampaignNewcomerSetting
	conflict := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var replay *marketingCampaignNewcomerRequestRow
		if readErr := s.database.GetScan(txCtx, &replay, `SELECT campaign_no, enabled, resulting_version, resulting_updated_at, request_digest
FROM plugin_idc_core_marketing_campaign_newcomer_request WHERE tenant_id=0 AND request_no=? FOR UPDATE`, input.RequestNo); readErr != nil {
			return readErr
		}
		if replay != nil {
			if replay.CampaignNo != input.CampaignNo || replay.Enabled != input.Enabled || replay.RequestDigest != requestDigest {
				conflict = true
				return bizerr.NewCode(CodeInsightsConflict)
			}
			result = MarketingCampaignNewcomerSetting{
				CampaignNo: replay.CampaignNo, Enabled: replay.Enabled,
				Version: replay.ResultingVersion, UpdatedAt: replay.ResultingUpdated,
			}
			return nil
		}
		var before struct {
			CampaignNo string    `orm:"campaign_no"`
			Enabled    bool      `orm:"enabled"`
			Status     string    `orm:"status_code"`
			Version    int64     `orm:"version"`
			UpdatedAt  time.Time `orm:"updated_at"`
		}
		if readErr := s.database.GetScan(txCtx, &before, `SELECT campaign_no, is_newcomer_exclusive AS enabled,
 status_code, version, updated_at FROM plugin_idc_core_marketing_campaign
WHERE tenant_id=0 AND campaign_no=? FOR UPDATE`, input.CampaignNo); readErr != nil {
			return readErr
		}
		if before.CampaignNo == "" || before.Version != input.ExpectedVersion ||
			(before.Status != "draft" && before.Status != "scheduled" && before.Status != "active" && before.Status != "paused") {
			conflict = true
			return bizerr.NewCode(CodeInsightsConflict)
		}
		if _, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_marketing_campaign
SET is_newcomer_exclusive=?, version=version+1, updated_by=?, updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND campaign_no=? AND version=?`, input.Enabled, actor, input.CampaignNo, input.ExpectedVersion); updateErr != nil {
			return updateErr
		}
		if _, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_marketing_campaign_newcomer_request
 (tenant_id, request_no, campaign_no, enabled, expected_version, resulting_version, resulting_updated_at, request_digest, updated_by)
VALUES (0, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?)`, input.RequestNo, input.CampaignNo, input.Enabled,
			input.ExpectedVersion, input.ExpectedVersion+1, requestDigest, actor); insertErr != nil {
			return insertErr
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "marketing.campaign.newcomer", IdempotencyKey: input.RequestNo,
			OperationCode: "marketing.campaign.newcomer", TargetType: "marketing_campaign", TargetNo: input.CampaignNo,
			ResultCode: "succeeded", ReasonCode: "campaign_configuration", RiskCode: "high",
			RequiredPermission: permissionMarketingNewcomerSet,
			BeforeSnapshot:     map[string]any{"newcomer_exclusive": before.Enabled, "version": before.Version},
			AfterSnapshot:      map[string]any{"newcomer_exclusive": input.Enabled, "version": input.ExpectedVersion + 1},
		}); auditErr != nil {
			return auditErr
		}
		return s.database.GetScan(txCtx, &result, `SELECT campaign_no, is_newcomer_exclusive AS enabled, version, updated_at
FROM plugin_idc_core_marketing_campaign WHERE tenant_id=0 AND campaign_no=?`, input.CampaignNo)
	})
	if err != nil {
		if conflict {
			return nil, bizerr.NewCode(CodeInsightsConflict)
		}
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return &result, nil
}

// marketingFactDigest returns a stable local identifier without retaining source payloads.
func marketingFactDigest(parts ...string) string {
	payload, err := json.Marshal(parts)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
