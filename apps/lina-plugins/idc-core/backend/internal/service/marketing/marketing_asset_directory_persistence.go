// This file loads bounded promotion asset, points, and granted-value pages from PostgreSQL.

package marketing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type promotionAssetDirectoryRow struct {
	PromotionAssetSummary
	InternalID int64 `orm:"internal_id"`
}

type pointsAccountDirectoryRow struct {
	PointsAccountSummary
	InternalID int64 `orm:"internal_id"`
}

type valueGrantDirectoryRow struct {
	ValueGrantSummary
	InternalID int64 `orm:"internal_id"`
}

func assetCursorBoundary(cursor *marketingManagementCursorBoundary) (time.Time, int64, int) {
	if cursor == nil {
		return time.Time{}, 0, 0
	}
	return time.UnixMicro(cursor.UpdatedAtMicroseconds).UTC(), cursor.ID, 1
}

func (s *assetDirectoryReaderImpl) searchAssets(ctx context.Context, input normalizedPromotionAssetSearch) (*PromotionAssetPage, error) {
	boundaryTime, boundaryID, hasCursor := assetCursorBoundary(input.cursor)
	rows := make([]promotionAssetDirectoryRow, 0, input.limit+1)
	countArgs := []any{input.typeCode, input.typeCode, input.status, input.status, input.currency, input.currency,
		input.customerNo, input.customerNo, input.campaignNo, input.campaignNo}
	pageArgs := append(append([]any(nil), countArgs...), hasCursor, boundaryTime, boundaryTime, boundaryID, input.limit+1, max(input.page-1, 0)*input.limit)
	total, err := readMarketingNumberedPage(ctx, s.database, &rows, "SELECT COUNT(*) "+promotionAssetFilterSQL, countArgs, promotionAssetPageSQL, pageArgs)
	if err != nil {
		return nil, gerror.Wrap(err, "load marketing promotion asset page")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &PromotionAssetPage{Assets: make([]PromotionAssetSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		page.Assets = append(page.Assets, row.PromotionAssetSummary)
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

func (s *assetDirectoryReaderImpl) searchPointsAccounts(ctx context.Context, input normalizedPointsAccountSearch) (*PointsAccountPage, error) {
	boundaryTime, boundaryID, hasCursor := assetCursorBoundary(input.cursor)
	rows := make([]pointsAccountDirectoryRow, 0, input.limit+1)
	countArgs := []any{input.customerNo, input.customerNo}
	pageArgs := append(append([]any(nil), countArgs...), hasCursor, boundaryTime, boundaryTime, boundaryID, input.limit+1, max(input.page-1, 0)*input.limit)
	total, err := readMarketingNumberedPage(ctx, s.database, &rows, "SELECT COUNT(*) "+pointsAccountFilterSQL, countArgs, pointsAccountPageSQL, pageArgs)
	if err != nil {
		return nil, gerror.Wrap(err, "load marketing points account page")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &PointsAccountPage{Accounts: make([]PointsAccountSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		page.Accounts = append(page.Accounts, row.PointsAccountSummary)
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

func (s *assetDirectoryReaderImpl) searchValueGrants(ctx context.Context, input normalizedValueGrantSearch) (*ValueGrantPage, error) {
	boundaryTime, boundaryID, hasCursor := assetCursorBoundary(input.cursor)
	rows := make([]valueGrantDirectoryRow, 0, input.limit+1)
	countArgs := []any{input.typeCode, input.typeCode, input.status, input.status, input.currency, input.currency,
		input.customerNo, input.customerNo, input.campaignNo, input.campaignNo}
	pageArgs := append(append([]any(nil), countArgs...), hasCursor, boundaryTime, boundaryTime, boundaryID, input.limit+1, max(input.page-1, 0)*input.limit)
	total, err := readMarketingNumberedPage(ctx, s.database, &rows, "SELECT COUNT(*) "+valueGrantFilterSQL, countArgs, valueGrantPageSQL, pageArgs)
	if err != nil {
		return nil, gerror.Wrap(err, "load marketing value grant page")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &ValueGrantPage{Grants: make([]ValueGrantSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		page.Grants = append(page.Grants, row.ValueGrantSummary)
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

func (s *assetDirectoryReaderImpl) GetAsset(ctx context.Context, assetNo string) (*PromotionAssetDetail, error) {
	if _, err := s.authorizer.AuthorizeAssetQuery(ctx); err != nil {
		return nil, err
	}
	if !identifierPattern.MatchString(assetNo) {
		return nil, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	var detail *PromotionAssetDetail
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, transactionErr := s.database.Exec(ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`); transactionErr != nil {
			return transactionErr
		}
		var readErr error
		detail, readErr = s.readAssetDetail(ctx, assetNo)
		return readErr
	})
	if err != nil {
		return nil, mapAssetDirectoryReadError(err)
	}
	if err = validatePromotionAssetDetail(detail); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return detail, nil
}

func (s *assetDirectoryReaderImpl) readAssetDetail(ctx context.Context, assetNo string) (*PromotionAssetDetail, error) {
	detail := &PromotionAssetDetail{}
	if err := s.database.GetScan(ctx, detail, promotionAssetDetailSQL, assetNo); err != nil {
		return nil, gerror.Wrap(err, "load marketing promotion asset detail")
	}
	if detail.AssetNo == "" {
		return nil, bizerr.NewCode(CodeManagementDirectoryNotFound)
	}
	detail.Reservations = make([]PromotionAssetReservationSummary, 0)
	if err := s.database.GetScan(ctx, &detail.Reservations, promotionAssetReservationSQL, assetNo, maximumAssetReservationCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing promotion asset reservations")
	}
	if len(detail.Reservations) > maximumAssetReservationCount {
		return nil, gerror.New("marketing promotion asset reservation limit exceeded")
	}
	return detail, nil
}

func (s *assetDirectoryReaderImpl) GetPointsAccount(ctx context.Context, customerNo string) (*PointsAccountDetail, error) {
	if _, err := s.authorizer.AuthorizePointsQuery(ctx); err != nil {
		return nil, err
	}
	if !identifierPattern.MatchString(customerNo) {
		return nil, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	var detail *PointsAccountDetail
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, transactionErr := s.database.Exec(ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`); transactionErr != nil {
			return transactionErr
		}
		var readErr error
		detail, readErr = s.readPointsAccountDetail(ctx, customerNo)
		return readErr
	})
	if err != nil {
		return nil, mapAssetDirectoryReadError(err)
	}
	if err = validatePointsAccountDetail(detail); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return detail, nil
}

func (s *assetDirectoryReaderImpl) readPointsAccountDetail(ctx context.Context, customerNo string) (*PointsAccountDetail, error) {
	detail := &PointsAccountDetail{}
	if err := s.database.GetScan(ctx, detail, pointsAccountDetailSQL, customerNo); err != nil {
		return nil, gerror.Wrap(err, "load marketing points account detail")
	}
	if detail.CustomerNo == "" {
		return nil, bizerr.NewCode(CodeManagementDirectoryNotFound)
	}
	detail.Lots = make([]PointsLotSummary, 0)
	if err := s.database.GetScan(ctx, &detail.Lots, pointsLotSQL, customerNo, maximumPointsLotCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing points lots")
	}
	if len(detail.Lots) > maximumPointsLotCount {
		return nil, gerror.New("marketing points lot limit exceeded")
	}
	detail.Entries = make([]PointsEntrySummary, 0)
	if err := s.database.GetScan(ctx, &detail.Entries, pointsEntrySQL, customerNo, maximumPointsEntryCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing points entries")
	}
	if len(detail.Entries) > maximumPointsEntryCount {
		return nil, gerror.New("marketing points entry limit exceeded")
	}
	return detail, nil
}

func (s *assetDirectoryReaderImpl) GetValueGrant(ctx context.Context, grantNo string) (*ValueGrantDetail, error) {
	if _, err := s.authorizer.AuthorizeGrantQuery(ctx); err != nil {
		return nil, err
	}
	if !identifierPattern.MatchString(grantNo) {
		return nil, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	var detail *ValueGrantDetail
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, transactionErr := s.database.Exec(ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`); transactionErr != nil {
			return transactionErr
		}
		var readErr error
		detail, readErr = s.readValueGrantDetail(ctx, grantNo)
		return readErr
	})
	if err != nil {
		return nil, mapAssetDirectoryReadError(err)
	}
	if err = validateValueGrantDetail(detail); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return detail, nil
}

func (s *assetDirectoryReaderImpl) readValueGrantDetail(ctx context.Context, grantNo string) (*ValueGrantDetail, error) {
	detail := &ValueGrantDetail{}
	if err := s.database.GetScan(ctx, detail, valueGrantDetailSQL, grantNo); err != nil {
		return nil, gerror.Wrap(err, "load marketing value grant detail")
	}
	if detail.GrantNo == "" {
		return nil, bizerr.NewCode(CodeManagementDirectoryNotFound)
	}
	detail.Entries = make([]ValueEntrySummary, 0)
	if err := s.database.GetScan(ctx, &detail.Entries, valueEntrySQL, grantNo, maximumValueEntryCount+1); err != nil {
		return nil, gerror.Wrap(err, "load marketing value entries")
	}
	if len(detail.Entries) > maximumValueEntryCount {
		return nil, gerror.New("marketing value entry limit exceeded")
	}
	return detail, nil
}

func mapAssetDirectoryReadError(err error) error {
	if bizerr.Is(err, CodeManagementDirectoryNotFound) {
		return err
	}
	return bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
}

func validatePromotionAssetDetail(detail *PromotionAssetDetail) error {
	if detail == nil || !validPromotionAssetSummary(detail.PromotionAssetSummary) || len(detail.Reservations) > maximumAssetReservationCount {
		return gerror.New("marketing promotion asset detail is invalid")
	}
	for _, reservation := range detail.Reservations {
		if !identifierPattern.MatchString(reservation.ReservationNo) || reservation.ReservedUses < 1 || reservation.DiscountAmountMinor < 0 ||
			!validReservationStatus(reservation.Status) || reservation.ExpiresAt.IsZero() || reservation.CreatedAt.IsZero() {
			return gerror.New("marketing promotion asset reservation is invalid")
		}
	}
	return validateAssetDirectoryPayload(detail)
}

func validatePointsAccountDetail(detail *PointsAccountDetail) error {
	if detail == nil || !validPointsAccountSummary(detail.PointsAccountSummary) || len(detail.Lots) > maximumPointsLotCount || len(detail.Entries) > maximumPointsEntryCount {
		return gerror.New("marketing points account detail is invalid")
	}
	for _, lot := range detail.Lots {
		if !validPointsLot(lot) {
			return gerror.New("marketing points lot is invalid")
		}
	}
	for _, entry := range detail.Entries {
		if !validPointsEntry(entry) {
			return gerror.New("marketing points entry is invalid")
		}
	}
	return validateAssetDirectoryPayload(detail)
}

func validateValueGrantDetail(detail *ValueGrantDetail) error {
	if detail == nil || !validValueGrantSummary(detail.ValueGrantSummary) || len(detail.Entries) > maximumValueEntryCount {
		return gerror.New("marketing value grant detail is invalid")
	}
	for _, entry := range detail.Entries {
		if !validValueEntry(entry) {
			return gerror.New("marketing value entry is invalid")
		}
	}
	return validateAssetDirectoryPayload(detail)
}

func validReservationStatus(value string) bool {
	return value == "active" || value == "consumed" || value == "released" || value == "expired"
}

func validPointsLot(lot PointsLotSummary) bool {
	return identifierPattern.MatchString(lot.LotNo) && validOptionalIdentifier(lot.CampaignNo) && validPointsLotSource(lot.SourceType) && validBusinessReference(lot.SourceReference) &&
		lot.OriginalPoints > 0 && nonNegativeMarketingOverviewValues(lot.AvailablePoints, lot.FrozenPoints, lot.ConsumedPoints, lot.ExpiredPoints, lot.ReversedPoints, lot.ReversalShortfallPoints) &&
		lot.AvailablePoints+lot.FrozenPoints+lot.ConsumedPoints+lot.ExpiredPoints+lot.ReversedPoints == lot.OriginalPoints && lot.ReversalShortfallPoints <= lot.ConsumedPoints &&
		lot.ReversedPoints+lot.ReversalShortfallPoints <= lot.OriginalPoints && validPointsLotLifecycle(lot) && !lot.CreatedAt.IsZero()
}

func validPointsEntry(entry PointsEntrySummary) bool {
	return identifierPattern.MatchString(entry.EntryNo) && validOptionalIdentifier(entry.LotNo) && validPointsEntryType(entry.Type) && validPointsEntrySource(entry.SourceType) && validBusinessReference(entry.SourceReference) &&
		(entry.AvailableDelta != 0 || entry.FrozenDelta != 0 || entry.DebtDelta != 0) && ((entry.Type == "reverse") == (entry.ReversalOfEntryNo != "")) &&
		validOptionalIdentifier(entry.ReversalOfEntryNo) && !entry.CreatedAt.IsZero()
}

func validValueEntry(entry ValueEntrySummary) bool {
	return identifierPattern.MatchString(entry.EntryNo) && validValueEntryType(entry.Type) && validValueSource(entry.SourceType) && validBusinessReference(entry.SourceReference) &&
		(entry.PendingDeltaMinor != 0 || entry.AvailableDeltaMinor != 0 || entry.FrozenDeltaMinor != 0 || entry.ConsumedDeltaMinor != 0 || entry.ExpiredDeltaMinor != 0 || entry.ReversedDeltaMinor != 0) &&
		((entry.Type == "reverse") == (entry.ReversalOfEntryNo != "")) && validOptionalIdentifier(entry.ReversalOfEntryNo) && !entry.CreatedAt.IsZero()
}

func validPointsLotSource(value string) bool {
	return value == "order" || value == "activity" || value == "manual" || value == "migration" || value == "reversal"
}

func validPointsLotLifecycle(lot PointsLotSummary) bool {
	switch lot.Status {
	case "available":
		return lot.AvailablePoints+lot.FrozenPoints > 0
	case "exhausted":
		return lot.AvailablePoints == 0 && lot.FrozenPoints == 0 && lot.ConsumedPoints > 0
	case "expired":
		return lot.AvailablePoints == 0 && lot.FrozenPoints == 0 && lot.ExpiredPoints > 0
	case "reversed":
		return lot.AvailablePoints == 0 && lot.FrozenPoints == 0 && lot.ReversedPoints > 0
	default:
		return false
	}
}

func validPointsEntryType(value string) bool {
	return value == "earn" || value == "freeze" || value == "release" || value == "spend" || value == "expire" || value == "reverse" || value == "debt" || value == "debt_repay"
}

func validPointsEntrySource(value string) bool {
	return value == "order" || value == "refund" || value == "activity" || value == "redemption" || value == "manual" || value == "migration" || value == "expiry"
}

func validValueEntryType(value string) bool {
	return value == "post" || value == "freeze" || value == "release" || value == "consume" || value == "expire" || value == "reverse" || value == "cancel"
}

const promotionAssetProjectionSQL = `a.asset_no, a.campaign_no, a.benefit_no, a.benefit_type_code AS benefit_type,
 a.asset_type_code AS type, coalesce(a.customer_no, '') AS customer_no, a.currency_code, a.face_amount_minor,
 a.rate_basis_points, a.usage_limit, a.reserved_uses, a.consumed_uses, a.status_code AS status,
 a.valid_from, a.valid_until, a.claimed_at, a.created_at, a.updated_at`

const promotionAssetFilterSQL = `FROM plugin_idc_core_marketing_promotion_asset a
WHERE a.tenant_id = 0 AND (? = '' OR a.asset_type_code = ?) AND (? = '' OR a.status_code = ?)
 AND (? = '' OR a.currency_code = ?) AND (? = '' OR a.customer_no = ?) AND (? = '' OR a.campaign_no = ?)`

const promotionAssetPageSQL = `SELECT a.id AS internal_id, ` + promotionAssetProjectionSQL + `
` + promotionAssetFilterSQL + `
   AND (? = 0 OR a.updated_at < ? OR (a.updated_at = ? AND a.id < ?))
ORDER BY a.updated_at DESC, a.id DESC LIMIT ? OFFSET ?`

const promotionAssetDetailSQL = `SELECT ` + promotionAssetProjectionSQL + `
FROM plugin_idc_core_marketing_promotion_asset a WHERE a.tenant_id = 0 AND a.asset_no = ?`

const promotionAssetReservationSQL = `SELECT reservation_no, reserved_uses, discount_amount_minor, status_code AS status, expires_at, created_at
FROM plugin_idc_core_marketing_asset_reservation
WHERE tenant_id = 0 AND asset_no = ? ORDER BY created_at DESC, id DESC LIMIT ?`

const pointsAccountFilterSQL = `FROM plugin_idc_core_marketing_points_account a
 WHERE a.tenant_id = 0 AND (? = '' OR a.customer_no = ?)`

const pointsAccountPageSQL = `WITH page AS MATERIALIZED (
 SELECT a.id AS internal_id, a.customer_no, a.available_points, a.frozen_points, a.debt_points, a.version, a.created_at, a.updated_at
 ` + pointsAccountFilterSQL + `
   AND (? = 0 OR a.updated_at < ? OR (a.updated_at = ? AND a.id < ?))
 ORDER BY a.updated_at DESC, a.id DESC LIMIT ? OFFSET ?
), lots AS (
 SELECT l.customer_no, count(*) AS lot_count FROM plugin_idc_core_marketing_points_lot l INNER JOIN page p ON p.customer_no = l.customer_no WHERE l.tenant_id = 0 GROUP BY l.customer_no
), entries AS (
 SELECT e.customer_no, count(*) AS entry_count FROM plugin_idc_core_marketing_points_entry e INNER JOIN page p ON p.customer_no = e.customer_no WHERE e.tenant_id = 0 GROUP BY e.customer_no
)
SELECT p.*, coalesce(l.lot_count, 0) AS lot_count, coalesce(e.entry_count, 0) AS entry_count
FROM page p LEFT JOIN lots l ON l.customer_no = p.customer_no LEFT JOIN entries e ON e.customer_no = p.customer_no
ORDER BY p.updated_at DESC, p.internal_id DESC`

const pointsAccountDetailSQL = `SELECT a.customer_no, a.available_points, a.frozen_points, a.debt_points, a.version, a.created_at, a.updated_at,
 (SELECT count(*) FROM plugin_idc_core_marketing_points_lot l WHERE l.tenant_id = 0 AND l.customer_no = a.customer_no) AS lot_count,
 (SELECT count(*) FROM plugin_idc_core_marketing_points_entry e WHERE e.tenant_id = 0 AND e.customer_no = a.customer_no) AS entry_count
FROM plugin_idc_core_marketing_points_account a WHERE a.tenant_id = 0 AND a.customer_no = ?`

const pointsLotSQL = `SELECT lot_no, coalesce(campaign_no, '') AS campaign_no, source_type_code AS source_type, source_reference,
 original_points, available_points, frozen_points, consumed_points, expired_points, reversed_points, reversal_shortfall_points,
 expires_at, status_code AS status, created_at
FROM plugin_idc_core_marketing_points_lot
WHERE tenant_id = 0 AND customer_no = ? ORDER BY created_at DESC, id DESC LIMIT ?`

const pointsEntrySQL = `SELECT entry_no, coalesce(lot_no, '') AS lot_no, entry_type_code AS type, available_delta, frozen_delta, debt_delta,
 source_type_code AS source_type, source_reference, coalesce(reversal_of_entry_no, '') AS reversal_of_entry_no, created_at
FROM plugin_idc_core_marketing_points_entry
WHERE tenant_id = 0 AND customer_no = ? ORDER BY created_at DESC, id DESC LIMIT ?`

const valueGrantProjectionSQL = `g.grant_no, g.grant_type_code AS type, g.campaign_no, g.benefit_no, g.customer_no, g.currency_code,
 g.source_reference, g.original_amount_minor, g.pending_amount_minor, g.available_amount_minor, g.frozen_amount_minor,
 g.consumed_amount_minor, g.expired_amount_minor, g.reversed_amount_minor, g.withdrawable, g.global_quota_reserved,
 g.customer_quota_reserved, g.expires_at, g.status_code AS status,
 coalesce(g.financial_posting_reference, '') AS financial_posting_reference, g.created_at, g.updated_at`

const valueGrantFilterSQL = `FROM plugin_idc_core_marketing_value_grant g
WHERE g.tenant_id = 0 AND (? = '' OR g.grant_type_code = ?) AND (? = '' OR g.status_code = ?)
 AND (? = '' OR g.currency_code = ?) AND (? = '' OR g.customer_no = ?) AND (? = '' OR g.campaign_no = ?)`

const valueGrantPageSQL = `SELECT g.id AS internal_id, ` + valueGrantProjectionSQL + `
` + valueGrantFilterSQL + `
   AND (? = 0 OR g.updated_at < ? OR (g.updated_at = ? AND g.id < ?))
ORDER BY g.updated_at DESC, g.id DESC LIMIT ? OFFSET ?`

const valueGrantDetailSQL = `SELECT ` + valueGrantProjectionSQL + `
FROM plugin_idc_core_marketing_value_grant g WHERE g.tenant_id = 0 AND g.grant_no = ?`

const valueEntrySQL = `SELECT entry_no, entry_type_code AS type, pending_delta_minor, available_delta_minor, frozen_delta_minor,
 consumed_delta_minor, expired_delta_minor, reversed_delta_minor, source_type_code AS source_type, source_reference,
 coalesce(reversal_of_entry_no, '') AS reversal_of_entry_no, created_at
FROM plugin_idc_core_marketing_value_entry
WHERE tenant_id = 0 AND grant_no = ? ORDER BY created_at DESC, id DESC LIMIT ?`
