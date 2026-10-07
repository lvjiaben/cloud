// This file loads bounded marketing-management pages and details from local PostgreSQL.

package marketing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type segmentManagementRow struct {
	SegmentManagementSummary
	InternalID int64 `orm:"internal_id"`
}

type campaignManagementRow struct {
	CampaignManagementSummary
	InternalID int64 `orm:"internal_id"`
}

func (s *managementDirectoryReaderImpl) searchSegments(ctx context.Context, input normalizedSegmentManagementSearch) (*SegmentManagementPage, error) {
	boundaryTime, boundaryID, hasCursor := time.Time{}, int64(0), 0
	if input.cursor != nil {
		boundaryTime = time.UnixMicro(input.cursor.UpdatedAtMicroseconds).UTC()
		boundaryID, hasCursor = input.cursor.ID, 1
	}
	rows := make([]segmentManagementRow, 0, input.limit+1)
	countArgs := []any{input.namePrefix, input.likePrefix, input.likePrefix, input.likePrefix,
		input.mode, input.mode, input.status, input.status}
	pageArgs := append(append([]any(nil), countArgs...), hasCursor, boundaryTime, boundaryTime, boundaryID, input.limit+1, max(input.page-1, 0)*input.limit)
	total, err := readMarketingNumberedPage(ctx, s.database, &rows, "SELECT COUNT(*) "+segmentManagementFilterSQL, countArgs, segmentManagementPageSQL, pageArgs)
	if err != nil {
		return nil, gerror.Wrap(err, "load marketing segment page")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &SegmentManagementPage{Segments: make([]SegmentManagementSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		page.Segments = append(page.Segments, row.SegmentManagementSummary)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor, err := s.cursor.encode(marketingManagementCursorBoundary{UpdatedAtMicroseconds: last.UpdatedAt.UnixMicro(), ID: last.InternalID}, input.digest)
		if err != nil {
			return nil, err
		}
		page.NextCursor = cursor
	}
	return page, nil
}

func (s *managementDirectoryReaderImpl) searchCampaigns(ctx context.Context, input normalizedCampaignManagementSearch) (*CampaignManagementPage, error) {
	boundaryTime, boundaryID, hasCursor := time.Time{}, int64(0), 0
	if input.cursor != nil {
		boundaryTime = time.UnixMicro(input.cursor.UpdatedAtMicroseconds).UTC()
		boundaryID, hasCursor = input.cursor.ID, 1
	}
	rows := make([]campaignManagementRow, 0, input.limit+1)
	countArgs := []any{input.namePrefix, input.likePrefix, input.likePrefix, input.likePrefix,
		input.typeCode, input.typeCode, input.status, input.status,
		input.targetSegmentNo, input.targetSegmentNo}
	pageArgs := append(append([]any(nil), countArgs...), hasCursor, boundaryTime, boundaryTime, boundaryID, input.limit+1, max(input.page-1, 0)*input.limit)
	total, err := readMarketingNumberedPage(ctx, s.database, &rows, "SELECT COUNT(*) "+campaignManagementFilterSQL, countArgs, campaignManagementPageSQL, pageArgs)
	if err != nil {
		return nil, gerror.Wrap(err, "load marketing campaign page")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &CampaignManagementPage{Campaigns: make([]CampaignManagementSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		page.Campaigns = append(page.Campaigns, row.CampaignManagementSummary)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor, err := s.cursor.encode(marketingManagementCursorBoundary{UpdatedAtMicroseconds: last.UpdatedAt.UnixMicro(), ID: last.InternalID}, input.digest)
		if err != nil {
			return nil, err
		}
		page.NextCursor = cursor
	}
	return page, nil
}

func (s *managementDirectoryReaderImpl) GetSegment(ctx context.Context, segmentNo string) (*SegmentManagementDetail, error) {
	if _, err := s.authorizer.AuthorizeSegmentQuery(ctx); err != nil {
		return nil, err
	}
	if !identifierPattern.MatchString(segmentNo) {
		return nil, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	var detail *SegmentManagementDetail
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, transactionErr := s.database.Exec(ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`); transactionErr != nil {
			return transactionErr
		}
		var readErr error
		detail, readErr = s.readSegmentDetail(ctx, segmentNo)
		return readErr
	})
	if err != nil {
		if bizerr.Is(err, CodeManagementDirectoryNotFound) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validateSegmentManagementDetail(detail); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return detail, nil
}

func (s *managementDirectoryReaderImpl) readSegmentDetail(ctx context.Context, segmentNo string) (*SegmentManagementDetail, error) {
	detail := &SegmentManagementDetail{}
	if err := s.database.GetScan(ctx, detail, segmentManagementDetailSQL, segmentNo); err != nil {
		return nil, gerror.Wrap(err, "load marketing segment detail")
	}
	if detail.SegmentNo == "" {
		return nil, bizerr.NewCode(CodeManagementDirectoryNotFound)
	}
	conditions := make([]segmentConditionRow, 0)
	if err := s.database.GetScan(ctx, &conditions, segmentConditionSQL, segmentNo, maximumSegmentConditionCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing segment conditions")
	}
	if len(conditions) > maximumSegmentConditionCount {
		return nil, gerror.New("marketing segment condition limit exceeded")
	}
	detail.Conditions = make([]SegmentCondition, 0, len(conditions))
	conditionIndex := make(map[string]int, len(conditions))
	for _, row := range conditions {
		conditionIndex[row.ConditionNo] = len(detail.Conditions)
		row.SegmentCondition.Values = make([]SegmentConditionValue, 0)
		detail.Conditions = append(detail.Conditions, row.SegmentCondition)
	}
	values := make([]segmentConditionValueRow, 0)
	if err := s.database.GetScan(ctx, &values, segmentConditionValueSQL, segmentNo, maximumSegmentConditionValueCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing segment condition values")
	}
	if len(values) > maximumSegmentConditionValueCount {
		return nil, gerror.New("marketing segment condition value limit exceeded")
	}
	for _, row := range values {
		index, ok := conditionIndex[row.ConditionNo]
		if !ok {
			return nil, gerror.New("marketing segment condition value is orphaned")
		}
		detail.Conditions[index].Values = append(detail.Conditions[index].Values, row.SegmentConditionValue)
	}
	detail.Snapshots = make([]SegmentSnapshotSummary, 0)
	if err := s.database.GetScan(ctx, &detail.Snapshots, segmentSnapshotSQL, segmentNo, maximumSegmentSnapshotCount); err != nil {
		return nil, gerror.Wrap(err, "load marketing segment snapshots")
	}
	return detail, nil
}

func (s *managementDirectoryReaderImpl) GetCampaign(ctx context.Context, campaignNo string) (*CampaignManagementDetail, error) {
	if _, err := s.authorizer.AuthorizeCampaignQuery(ctx); err != nil {
		return nil, err
	}
	if !identifierPattern.MatchString(campaignNo) {
		return nil, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	var detail *CampaignManagementDetail
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, transactionErr := s.database.Exec(ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`); transactionErr != nil {
			return transactionErr
		}
		var readErr error
		detail, readErr = s.readCampaignDetail(ctx, campaignNo)
		return readErr
	})
	if err != nil {
		if bizerr.Is(err, CodeManagementDirectoryNotFound) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validateCampaignManagementDetail(detail); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return detail, nil
}

func (s *managementDirectoryReaderImpl) readCampaignDetail(ctx context.Context, campaignNo string) (*CampaignManagementDetail, error) {
	detail := &CampaignManagementDetail{}
	if err := s.database.GetScan(ctx, detail, campaignManagementDetailSQL, campaignNo); err != nil {
		return nil, gerror.Wrap(err, "load marketing campaign detail")
	}
	if detail.CampaignNo == "" {
		return nil, bizerr.NewCode(CodeManagementDirectoryNotFound)
	}
	detail.Scopes = make([]CampaignScope, 0)
	if err := s.database.GetScan(ctx, &detail.Scopes, campaignScopeSQL, campaignNo, maximumCampaignScopeCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing campaign scopes")
	}
	if len(detail.Scopes) > maximumCampaignScopeCount {
		return nil, gerror.New("marketing campaign scope limit exceeded")
	}
	detail.Benefits = make([]CampaignBenefit, 0)
	if err := s.database.GetScan(ctx, &detail.Benefits, campaignBenefitSQL, campaignNo, maximumCampaignBenefitCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing campaign benefits")
	}
	if len(detail.Benefits) > maximumCampaignBenefitCount {
		return nil, gerror.New("marketing campaign benefit limit exceeded")
	}
	return detail, nil
}

type segmentConditionRow struct{ SegmentCondition }
type segmentConditionValueRow struct {
	ConditionNo string `orm:"condition_no"`
	SegmentConditionValue
}

func validateSegmentManagementDetail(detail *SegmentManagementDetail) error {
	if detail == nil || !validSegmentSummary(detail.SegmentManagementSummary) || len(detail.Conditions) > maximumSegmentConditionCount || len(detail.Snapshots) > maximumSegmentSnapshotCount {
		return gerror.New("marketing segment detail is invalid")
	}
	totalValues := 0
	for _, condition := range detail.Conditions {
		if !validSegmentCondition(condition) {
			return gerror.New("marketing segment condition is invalid")
		}
		totalValues += len(condition.Values)
	}
	if totalValues > maximumSegmentConditionValueCount {
		return gerror.New("marketing segment condition values exceeded limit")
	}
	for _, snapshot := range detail.Snapshots {
		if !validSegmentSnapshot(snapshot) {
			return gerror.New("marketing segment snapshot is invalid")
		}
	}
	return validateMarketingManagementPayload(detail)
}

func validateCampaignManagementDetail(detail *CampaignManagementDetail) error {
	if detail == nil || !validCampaignSummary(detail.CampaignManagementSummary) || detail.Version < 1 || len(detail.Scopes) > maximumCampaignScopeCount || len(detail.Benefits) > maximumCampaignBenefitCount {
		return gerror.New("marketing campaign detail is invalid")
	}
	for _, scope := range detail.Scopes {
		if !validCampaignScope(scope) {
			return gerror.New("marketing campaign scope is invalid")
		}
	}
	for _, benefit := range detail.Benefits {
		if !validCampaignBenefit(benefit) {
			return gerror.New("marketing campaign benefit is invalid")
		}
	}
	return validateMarketingManagementPayload(detail)
}

func validSegmentCondition(condition SegmentCondition) bool {
	if !identifierPattern.MatchString(condition.ConditionNo) || condition.OrGroupNo < 1 || condition.OrGroupNo > 128 || condition.SortOrder < 0 || condition.SortOrder > 1000000 ||
		!validSegmentFact(condition.Fact) || !validSegmentOperator(condition.Operator) || !validSegmentValueKind(condition.ValueKind) {
		return false
	}
	if (condition.ValueKind == "currency_amount") != currencyPattern.MatchString(condition.CurrencyCode) ||
		(condition.ValueKind != "currency_amount" && condition.CurrencyCode != "") {
		return false
	}
	for _, value := range condition.Values {
		if !validSegmentConditionValue(value) {
			return false
		}
	}
	return validSegmentConditionOperands(condition)
}

func validSegmentConditionValue(value SegmentConditionValue) bool {
	return (value.Text == "") != (value.Integer == nil) && len(value.Text) <= 128
}

func validSegmentConditionOperands(condition SegmentCondition) bool {
	switch condition.Operator {
	case "exists", "not_exists":
		return condition.ValueKind == "none" && noScalarSegmentConditionValue(condition) && len(condition.Values) == 0
	case "in", "not_in":
		return (condition.ValueKind == "text" || condition.ValueKind == "integer") && noScalarSegmentConditionValue(condition) && len(condition.Values) > 0
	case "eq", "neq":
		return validEqualitySegmentCondition(condition)
	case "gte", "lte":
		return validSingleIntegerSegmentCondition(condition)
	case "between":
		return validRangeSegmentCondition(condition)
	}
	return false
}

func noScalarSegmentConditionValue(condition SegmentCondition) bool {
	return condition.ValueText == "" && condition.ValueIntegerLower == nil && condition.ValueIntegerUpper == nil && condition.ValueBoolean == nil
}

func validEqualitySegmentCondition(condition SegmentCondition) bool {
	if len(condition.Values) != 0 {
		return false
	}
	switch condition.ValueKind {
	case "text":
		return condition.ValueText != "" && len(condition.ValueText) <= 128 && condition.ValueIntegerLower == nil && condition.ValueIntegerUpper == nil && condition.ValueBoolean == nil
	case "boolean":
		return condition.ValueText == "" && condition.ValueIntegerLower == nil && condition.ValueIntegerUpper == nil && condition.ValueBoolean != nil
	case "integer", "currency_amount":
		return validSingleIntegerSegmentCondition(condition)
	default:
		return false
	}
}

func validSingleIntegerSegmentCondition(condition SegmentCondition) bool {
	return (condition.ValueKind == "integer" || condition.ValueKind == "currency_amount") && condition.ValueText == "" && condition.ValueIntegerLower != nil && condition.ValueIntegerUpper == nil && condition.ValueBoolean == nil && len(condition.Values) == 0
}

func validRangeSegmentCondition(condition SegmentCondition) bool {
	return (condition.ValueKind == "integer" || condition.ValueKind == "currency_amount") && condition.ValueText == "" && condition.ValueIntegerLower != nil && condition.ValueIntegerUpper != nil && *condition.ValueIntegerUpper >= *condition.ValueIntegerLower && condition.ValueBoolean == nil && len(condition.Values) == 0
}

func validSegmentFact(value string) bool {
	switch value {
	case "customer_level", "customer_tag", "customer_type", "identity_status", "region_country", "registered_age_days", "paid_amount", "active_service_count", "renewal_due_days", "ticket_count", "acquisition_channel", "risk_state":
		return true
	default:
		return false
	}
}

func validSegmentOperator(value string) bool {
	switch value {
	case "eq", "neq", "in", "not_in", "gte", "lte", "between", "exists", "not_exists":
		return true
	default:
		return false
	}
}

func validSegmentValueKind(value string) bool {
	return value == "none" || value == "text" || value == "integer" || value == "boolean" || value == "currency_amount"
}

func validSegmentSnapshot(snapshot SegmentSnapshotSummary) bool {
	if !identifierPattern.MatchString(snapshot.SnapshotNo) || snapshot.SegmentVersion < 1 || !validSegmentSnapshotStatus(snapshot.Status) || snapshot.MemberCount < 0 || snapshot.StartedAt.IsZero() {
		return false
	}
	return (snapshot.Status == "building" && snapshot.CompletedAt == nil) ||
		(snapshot.Status != "building" && snapshot.CompletedAt != nil && !snapshot.CompletedAt.Before(snapshot.StartedAt))
}

func validCampaignScope(scope CampaignScope) bool {
	if scope.MatchMode != "include" && scope.MatchMode != "exclude" {
		return false
	}
	switch scope.Kind {
	case "all_products":
		return scope.MatchMode == "include" && scope.ReferenceCode == ""
	case "product", "category":
		return scope.ReferenceCode != "" && len(scope.ReferenceCode) <= 128
	default:
		return false
	}
}

func validCampaignBenefit(benefit CampaignBenefit) bool {
	if !identifierPattern.MatchString(benefit.BenefitNo) || benefit.Version < 1 ||
		(benefit.PurchasePhase != "new_purchase" && benefit.PurchasePhase != "renewal" && benefit.PurchasePhase != "both") ||
		(benefit.Status != "active" && benefit.Status != "inactive") ||
		(benefit.MinimumSpendMinor != nil && *benefit.MinimumSpendMinor < 0) || !validCampaignBenefitCycle(benefit) {
		return false
	}
	return validCampaignBenefitValue(benefit)
}

func validCampaignBenefitCycle(benefit CampaignBenefit) bool {
	if (benefit.CycleScope == "first" || benefit.CycleScope == "all") && benefit.CycleCount != nil {
		return false
	}
	if benefit.CycleScope == "limited" {
		return benefit.CycleCount != nil && *benefit.CycleCount >= 1 && *benefit.CycleCount <= 1000000
	}
	return benefit.CycleScope == "first" || benefit.CycleScope == "all"
}

func validCampaignBenefitValue(benefit CampaignBenefit) bool {
	if benefit.Type == "points" {
		return benefit.CurrencyCode == "" && benefit.PointsQuantity != nil && *benefit.PointsQuantity > 0 && benefit.AmountMinor == nil && benefit.RateBasisPoints == nil && benefit.MaximumDiscountMinor == nil && benefit.MinimumSpendMinor == nil
	}
	if !currencyPattern.MatchString(benefit.CurrencyCode) || benefit.PointsQuantity != nil {
		return false
	}
	switch benefit.Type {
	case "fixed_discount", "voucher", "recharge_bonus", "cashback", "customer_price":
		return benefit.AmountMinor != nil && *benefit.AmountMinor > 0 && benefit.RateBasisPoints == nil && benefit.MaximumDiscountMinor == nil
	case "percentage_discount":
		return benefit.AmountMinor == nil && benefit.RateBasisPoints != nil && *benefit.RateBasisPoints >= 1 && *benefit.RateBasisPoints <= 10000 &&
			(benefit.MaximumDiscountMinor == nil || *benefit.MaximumDiscountMinor > 0)
	default:
		return false
	}
}

const segmentManagementFilterSQL = `FROM plugin_idc_core_marketing_segment s
 WHERE s.tenant_id = 0
   AND (? = '' OR lower(s.segment_code) LIKE ? ESCAPE '\' OR lower(s.name_zh_cn) LIKE ? ESCAPE '\' OR lower(s.name_en_us) LIKE ? ESCAPE '\')
   AND (? = '' OR s.mode_code = ?) AND (? = '' OR s.status_code = ?)`

const segmentManagementPageSQL = `
WITH page AS MATERIALIZED (
 SELECT s.id AS internal_id, s.segment_no, s.segment_code, s.name_zh_cn, s.name_en_us,
        s.mode_code AS mode, s.refresh_interval_minutes, s.status_code AS status, s.version,
        s.created_at, s.updated_at
 ` + segmentManagementFilterSQL + `
   AND (? = 0 OR s.updated_at < ? OR (s.updated_at = ? AND s.id < ?))
 ORDER BY s.updated_at DESC, s.id DESC LIMIT ? OFFSET ?
), condition_stats AS (
 SELECT c.segment_no, count(*) AS condition_count FROM plugin_idc_core_marketing_segment_condition c INNER JOIN page p ON p.segment_no = c.segment_no WHERE c.tenant_id = 0 GROUP BY c.segment_no
), member_stats AS (
 SELECT m.segment_no, count(*) AS active_member_count FROM plugin_idc_core_marketing_segment_member m INNER JOIN page p ON p.segment_no = m.segment_no WHERE m.tenant_id = 0 AND m.status_code = 'active' GROUP BY m.segment_no
), latest_snapshot AS (
 SELECT DISTINCT ON (ss.segment_no) ss.segment_no, ss.snapshot_no, ss.status_code AS latest_snapshot_status, ss.member_count AS latest_snapshot_members, ss.started_at AS latest_snapshot_at
 FROM plugin_idc_core_marketing_segment_snapshot ss INNER JOIN page p ON p.segment_no = ss.segment_no WHERE ss.tenant_id = 0 ORDER BY ss.segment_no, ss.started_at DESC, ss.id DESC
)
SELECT p.*, coalesce(c.condition_count, 0) AS condition_count, coalesce(m.active_member_count, 0) AS active_member_count,
       coalesce(ls.snapshot_no, '') AS latest_snapshot_no, coalesce(ls.latest_snapshot_status, '') AS latest_snapshot_status,
       coalesce(ls.latest_snapshot_members, 0) AS latest_snapshot_members, ls.latest_snapshot_at
FROM page p LEFT JOIN condition_stats c ON c.segment_no = p.segment_no LEFT JOIN member_stats m ON m.segment_no = p.segment_no LEFT JOIN latest_snapshot ls ON ls.segment_no = p.segment_no
ORDER BY p.updated_at DESC, p.internal_id DESC`

const campaignManagementFilterSQL = `FROM plugin_idc_core_marketing_campaign c
 WHERE c.tenant_id = 0
   AND (? = '' OR lower(c.campaign_code) LIKE ? ESCAPE '\' OR lower(c.name_zh_cn) LIKE ? ESCAPE '\' OR lower(c.name_en_us) LIKE ? ESCAPE '\')
   AND (? = '' OR c.campaign_type_code = ?) AND (? = '' OR c.status_code = ?)
   AND (? = '' OR c.target_segment_no = ?)`

const campaignManagementPageSQL = `
WITH page AS MATERIALIZED (
 SELECT c.id AS internal_id, c.campaign_no, c.campaign_code, c.name_zh_cn, c.name_en_us,
        c.campaign_type_code AS type, c.status_code AS status, coalesce(c.target_segment_no, '') AS target_segment_no,
        coalesce(c.target_snapshot_no, '') AS target_snapshot_no, c.priority, c.stack_policy_code AS stack_policy,
        c.global_quota_limit, c.global_reserved_count, c.global_consumed_count, c.per_customer_quota_limit,
        c.is_newcomer_exclusive,
        c.starts_at, c.ends_at, c.created_at, c.updated_at
 ` + campaignManagementFilterSQL + `
   AND (? = 0 OR c.updated_at < ? OR (c.updated_at = ? AND c.id < ?))
 ORDER BY c.updated_at DESC, c.id DESC LIMIT ? OFFSET ?
), scope_stats AS (
 SELECT s.campaign_no, count(*) AS scope_count FROM plugin_idc_core_marketing_campaign_scope s INNER JOIN page p ON p.campaign_no = s.campaign_no WHERE s.tenant_id = 0 GROUP BY s.campaign_no
), benefit_stats AS (
 SELECT b.campaign_no, count(*) AS benefit_count FROM plugin_idc_core_marketing_benefit_rule b INNER JOIN page p ON p.campaign_no = b.campaign_no WHERE b.tenant_id = 0 GROUP BY b.campaign_no
), reservation_stats AS (
 SELECT r.campaign_no, count(*) AS reservation_count FROM plugin_idc_core_marketing_campaign_reservation r INNER JOIN page p ON p.campaign_no = r.campaign_no WHERE r.tenant_id = 0 GROUP BY r.campaign_no
)
SELECT p.*, coalesce(s.scope_count, 0) AS scope_count, coalesce(b.benefit_count, 0) AS benefit_count, coalesce(r.reservation_count, 0) AS reservation_count
FROM page p LEFT JOIN scope_stats s ON s.campaign_no = p.campaign_no LEFT JOIN benefit_stats b ON b.campaign_no = p.campaign_no LEFT JOIN reservation_stats r ON r.campaign_no = p.campaign_no
ORDER BY p.updated_at DESC, p.internal_id DESC`

const segmentManagementDetailSQL = `
SELECT s.segment_no, s.segment_code, s.name_zh_cn, s.name_en_us, s.description_zh_cn, s.description_en_us,
       s.mode_code AS mode, s.refresh_interval_minutes, s.status_code AS status, s.version, s.created_at, s.updated_at,
       (SELECT count(*) FROM plugin_idc_core_marketing_segment_condition c WHERE c.tenant_id = 0 AND c.segment_no = s.segment_no) AS condition_count,
       (SELECT count(*) FROM plugin_idc_core_marketing_segment_member m WHERE m.tenant_id = 0 AND m.segment_no = s.segment_no AND m.status_code = 'active') AS active_member_count,
       coalesce(latest.snapshot_no, '') AS latest_snapshot_no, coalesce(latest.status_code, '') AS latest_snapshot_status,
       coalesce(latest.member_count, 0) AS latest_snapshot_members, latest.started_at AS latest_snapshot_at
FROM plugin_idc_core_marketing_segment s
LEFT JOIN LATERAL (SELECT snapshot_no, status_code, member_count, started_at FROM plugin_idc_core_marketing_segment_snapshot WHERE tenant_id = 0 AND segment_no = s.segment_no ORDER BY started_at DESC, id DESC LIMIT 1) latest ON TRUE
WHERE s.tenant_id = 0 AND s.segment_no = ?`

const segmentConditionSQL = `SELECT condition_no, or_group_no, sort_order, fact_code AS fact, operator_code AS operator, value_kind_code AS value_kind,
 coalesce(value_text, '') AS value_text, value_integer_lower, value_integer_upper, value_boolean, coalesce(currency_code, '') AS currency_code
 FROM plugin_idc_core_marketing_segment_condition WHERE tenant_id = 0 AND segment_no = ? ORDER BY or_group_no ASC, sort_order ASC, id ASC LIMIT ?`

const segmentConditionValueSQL = `SELECT v.condition_no, coalesce(v.value_text, '') AS text, v.value_integer AS integer
 FROM plugin_idc_core_marketing_condition_value v INNER JOIN plugin_idc_core_marketing_segment_condition c ON c.condition_no = v.condition_no
 WHERE v.tenant_id = 0 AND c.tenant_id = 0 AND c.segment_no = ? ORDER BY c.or_group_no ASC, c.sort_order ASC, v.sort_order ASC, v.id ASC LIMIT ?`

const segmentSnapshotSQL = `SELECT snapshot_no, segment_version, status_code AS status, member_count, started_at, completed_at
 FROM plugin_idc_core_marketing_segment_snapshot WHERE tenant_id = 0 AND segment_no = ? ORDER BY started_at DESC, id DESC LIMIT ?`

const campaignManagementDetailSQL = `
SELECT c.campaign_no, c.campaign_code, c.name_zh_cn, c.name_en_us, c.description_zh_cn, c.description_en_us,
       c.campaign_type_code AS type, c.status_code AS status, coalesce(c.target_segment_no, '') AS target_segment_no,
       coalesce(c.target_snapshot_no, '') AS target_snapshot_no, c.priority, c.stack_policy_code AS stack_policy,
       c.stack_group_code AS stack_group, c.global_quota_limit, c.global_reserved_count, c.global_consumed_count,
       c.per_customer_quota_limit, c.is_newcomer_exclusive, c.version, c.starts_at, c.ends_at, c.created_at, c.updated_at,
       (SELECT count(*) FROM plugin_idc_core_marketing_campaign_scope s WHERE s.tenant_id = 0 AND s.campaign_no = c.campaign_no) AS scope_count,
       (SELECT count(*) FROM plugin_idc_core_marketing_benefit_rule b WHERE b.tenant_id = 0 AND b.campaign_no = c.campaign_no) AS benefit_count,
       (SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation r WHERE r.tenant_id = 0 AND r.campaign_no = c.campaign_no) AS reservation_count
FROM plugin_idc_core_marketing_campaign c WHERE c.tenant_id = 0 AND c.campaign_no = ?`

const campaignScopeSQL = `SELECT scope_kind_code AS kind, match_mode_code AS match_mode, reference_code FROM plugin_idc_core_marketing_campaign_scope WHERE tenant_id = 0 AND campaign_no = ? ORDER BY scope_kind_code ASC, match_mode_code ASC, reference_code ASC LIMIT ?`

const campaignBenefitSQL = `SELECT benefit_no, benefit_type_code AS type, coalesce(currency_code, '') AS currency_code, amount_minor,
 rate_basis_points, maximum_discount_minor, minimum_spend_minor, points_quantity, purchase_phase_code AS purchase_phase,
 cycle_scope_code AS cycle_scope, cycle_count, status_code AS status, version
 FROM plugin_idc_core_marketing_benefit_rule WHERE tenant_id = 0 AND campaign_no = ? ORDER BY id ASC LIMIT ?`
