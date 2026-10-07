// This file provides the bounded, staff-only marketing operations overview.

package marketing

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	permissionMarketingOverview         = "idc:admin:marketing:overview"
	maximumMarketingOverviewCurrencies  = 128
	maximumMarketingOverviewRecentItems = 12
	maximumMarketingOverviewBytes       = 512 * 1024
)

// ManagementOverviewAuthorizer fails closed before any marketing data is read.
type ManagementOverviewAuthorizer interface {
	// AuthorizeMarketingOverview validates a direct active staff actor and exact read permission before persistence access.
	AuthorizeMarketingOverview(ctx context.Context) (actorUserID int, err error)
}

// ManagementOverviewReader returns the local marketing operations projection.
type ManagementOverviewReader interface {
	// GetOverview returns one bounded, currency-separated local snapshot or a stable authorization/unavailable error.
	GetOverview(ctx context.Context) (*ManagementOverview, error)
}

// ManagementOverview is one bounded and currency-safe operations snapshot.
type ManagementOverview struct {
	GeneratedAt     time.Time
	Summary         MarketingOverviewSummary
	Currencies      []MarketingCurrencySummary
	RecentCampaigns []MarketingCampaignSummary
	RecentGrants    []MarketingGrantSummary
}

// MarketingOverviewSummary contains only counts and non-monetary totals.
type MarketingOverviewSummary struct {
	SegmentTotal          int64
	SegmentActive         int64
	SegmentDynamic        int64
	ActiveSegmentMembers  int64
	CampaignTotal         int64
	CampaignActive        int64
	CampaignScheduled     int64
	CampaignPaused        int64
	CampaignReservedUnits int64
	CampaignConsumedUnits int64
	AssetTotal            int64
	AssetIssued           int64
	AssetActive           int64
	AssetReservedUses     int64
	AssetConsumedUses     int64
	PointsAccountCount    int64
	PointsAvailable       int64
	PointsFrozen          int64
	PointsDebt            int64
	ConsentOptedIn        int64
	ConsentOptedOut       int64
}

// MarketingCurrencySummary keeps every monetary amount inside one explicit currency.
type MarketingCurrencySummary struct {
	CurrencyCode   string
	GrantCount     int64
	PendingMinor   int64
	AvailableMinor int64
	FrozenMinor    int64
	ConsumedMinor  int64
	ExpiredMinor   int64
	ReversedMinor  int64
}

// MarketingCampaignSummary is the bounded recent campaign projection.
type MarketingCampaignSummary struct {
	CampaignNo          string
	NameZhCN            string
	NameEnUS            string
	Type                string
	Status              string
	TargetSegmentNo     string
	TargetSnapshotNo    string
	Priority            int
	GlobalQuotaLimit    *int64
	GlobalReservedCount int64
	GlobalConsumedCount int64
	StartsAt            time.Time
	EndsAt              time.Time
	UpdatedAt           time.Time
}

// MarketingGrantSummary is the bounded recent recharge-bonus/cashback projection.
type MarketingGrantSummary struct {
	GrantNo        string
	GrantType      string
	CampaignNo     string
	CustomerNo     string
	CurrencyCode   string
	OriginalMinor  int64
	PendingMinor   int64
	AvailableMinor int64
	FrozenMinor    int64
	ConsumedMinor  int64
	ExpiredMinor   int64
	ReversedMinor  int64
	Withdrawable   bool
	Status         string
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type managementOverviewReaderImpl struct {
	database   gdb.DB
	authorizer ManagementOverviewAuthorizer
}

var _ ManagementOverviewReader = (*managementOverviewReaderImpl)(nil)

// NewManagementOverviewReader constructs the staff-only local read model.
func NewManagementOverviewReader(
	database gdb.DB,
	authorizer ManagementOverviewAuthorizer,
) (ManagementOverviewReader, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(authorizer) {
		return nil, gerror.New("marketing management overview requires database and authorization")
	}
	return &managementOverviewReaderImpl{database: database, authorizer: authorizer}, nil
}

// GetOverview authorizes before persistence access and returns one consistent local snapshot.
func (s *managementOverviewReaderImpl) GetOverview(ctx context.Context) (*ManagementOverview, error) {
	if _, err := s.authorizer.AuthorizeMarketingOverview(ctx); err != nil {
		return nil, err
	}
	var overview *ManagementOverview
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, transactionErr := s.database.Exec(
			ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`,
		); transactionErr != nil {
			return gerror.Wrap(transactionErr, "start consistent marketing overview read")
		}
		var readErr error
		overview, readErr = s.readOverview(ctx)
		return readErr
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementOverviewUnavailable)
	}
	if err = validateManagementOverview(overview); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementOverviewUnavailable)
	}
	return overview, nil
}

// readOverview performs the fixed-query projection inside the caller's read-only transaction.
func (s *managementOverviewReaderImpl) readOverview(ctx context.Context) (*ManagementOverview, error) {
	overview := &ManagementOverview{}
	if err := s.database.GetScan(ctx, &overview.Summary, `
SELECT
  (SELECT count(*) FROM plugin_idc_core_marketing_segment WHERE tenant_id = 0) AS segment_total,
  (SELECT count(*) FROM plugin_idc_core_marketing_segment WHERE tenant_id = 0 AND status_code = 'active') AS segment_active,
  (SELECT count(*) FROM plugin_idc_core_marketing_segment WHERE tenant_id = 0 AND mode_code = 'dynamic') AS segment_dynamic,
  (SELECT count(*) FROM plugin_idc_core_marketing_segment_member WHERE tenant_id = 0 AND status_code = 'active') AS active_segment_members,
  (SELECT count(*) FROM plugin_idc_core_marketing_campaign WHERE tenant_id = 0) AS campaign_total,
  (SELECT count(*) FROM plugin_idc_core_marketing_campaign WHERE tenant_id = 0 AND status_code = 'active') AS campaign_active,
  (SELECT count(*) FROM plugin_idc_core_marketing_campaign WHERE tenant_id = 0 AND status_code = 'scheduled') AS campaign_scheduled,
  (SELECT count(*) FROM plugin_idc_core_marketing_campaign WHERE tenant_id = 0 AND status_code = 'paused') AS campaign_paused,
  (SELECT coalesce(sum(global_reserved_count), 0) FROM plugin_idc_core_marketing_campaign WHERE tenant_id = 0) AS campaign_reserved_units,
  (SELECT coalesce(sum(global_consumed_count), 0) FROM plugin_idc_core_marketing_campaign WHERE tenant_id = 0) AS campaign_consumed_units,
  (SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id = 0) AS asset_total,
  (SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id = 0 AND status_code = 'issued') AS asset_issued,
  (SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id = 0 AND status_code = 'active') AS asset_active,
  (SELECT coalesce(sum(reserved_uses), 0) FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id = 0) AS asset_reserved_uses,
  (SELECT coalesce(sum(consumed_uses), 0) FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id = 0) AS asset_consumed_uses,
  (SELECT count(*) FROM plugin_idc_core_marketing_points_account WHERE tenant_id = 0) AS points_account_count,
  (SELECT coalesce(sum(available_points), 0) FROM plugin_idc_core_marketing_points_account WHERE tenant_id = 0) AS points_available,
  (SELECT coalesce(sum(frozen_points), 0) FROM plugin_idc_core_marketing_points_account WHERE tenant_id = 0) AS points_frozen,
  (SELECT coalesce(sum(debt_points), 0) FROM plugin_idc_core_marketing_points_account WHERE tenant_id = 0) AS points_debt,
  (SELECT count(*) FROM plugin_idc_core_marketing_channel_consent WHERE tenant_id = 0 AND status_code = 'opted_in') AS consent_opted_in,
  (SELECT count(*) FROM plugin_idc_core_marketing_channel_consent WHERE tenant_id = 0 AND status_code = 'opted_out') AS consent_opted_out`); err != nil {
		return nil, gerror.Wrap(err, "read marketing overview summary")
	}
	databaseTime, err := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, gerror.Wrap(err, "read marketing overview database time")
	}
	overview.GeneratedAt = databaseTime.Time()
	if err = s.database.GetScan(ctx, &overview.Currencies, `
SELECT currency_code, count(*) AS grant_count,
       coalesce(sum(pending_amount_minor), 0) AS pending_minor,
       coalesce(sum(available_amount_minor), 0) AS available_minor,
       coalesce(sum(frozen_amount_minor), 0) AS frozen_minor,
       coalesce(sum(consumed_amount_minor), 0) AS consumed_minor,
       coalesce(sum(expired_amount_minor), 0) AS expired_minor,
       coalesce(sum(reversed_amount_minor), 0) AS reversed_minor
FROM plugin_idc_core_marketing_value_grant
WHERE tenant_id = 0
GROUP BY currency_code
ORDER BY currency_code ASC
LIMIT ?`, maximumMarketingOverviewCurrencies+1); err != nil {
		return nil, gerror.Wrap(err, "read marketing overview currency summaries")
	}
	if err = s.database.GetScan(ctx, &overview.RecentCampaigns, `
SELECT campaign_no, name_zh_cn, name_en_us, campaign_type_code AS type,
       status_code AS status, coalesce(target_segment_no, '') AS target_segment_no,
       coalesce(target_snapshot_no, '') AS target_snapshot_no, priority,
       global_quota_limit, global_reserved_count, global_consumed_count,
       starts_at, ends_at, updated_at
FROM plugin_idc_core_marketing_campaign
WHERE tenant_id = 0
ORDER BY updated_at DESC, id DESC
LIMIT ?`, maximumMarketingOverviewRecentItems); err != nil {
		return nil, gerror.Wrap(err, "read recent marketing campaigns")
	}
	if err = s.database.GetScan(ctx, &overview.RecentGrants, `
SELECT grant_no, grant_type_code AS grant_type, campaign_no, customer_no,
       currency_code, original_amount_minor AS original_minor,
       pending_amount_minor AS pending_minor, available_amount_minor AS available_minor,
       frozen_amount_minor AS frozen_minor, consumed_amount_minor AS consumed_minor,
       expired_amount_minor AS expired_minor, reversed_amount_minor AS reversed_minor,
       withdrawable, status_code AS status, expires_at, created_at, updated_at
FROM plugin_idc_core_marketing_value_grant
WHERE tenant_id = 0
ORDER BY updated_at DESC, id DESC
LIMIT ?`, maximumMarketingOverviewRecentItems); err != nil {
		return nil, gerror.Wrap(err, "read recent marketing value grants")
	}
	return overview, nil
}

// validateManagementOverview fails closed on malformed, negative, or oversized projections.
func validateManagementOverview(overview *ManagementOverview) error {
	if overview == nil || overview.GeneratedAt.IsZero() ||
		len(overview.Currencies) > maximumMarketingOverviewCurrencies ||
		len(overview.RecentCampaigns) > maximumMarketingOverviewRecentItems ||
		len(overview.RecentGrants) > maximumMarketingOverviewRecentItems {
		return gerror.New("marketing overview exceeded projection bounds")
	}
	if !nonNegativeMarketingOverviewValues(
		overview.Summary.SegmentTotal,
		overview.Summary.SegmentActive,
		overview.Summary.SegmentDynamic,
		overview.Summary.ActiveSegmentMembers,
		overview.Summary.CampaignTotal,
		overview.Summary.CampaignActive,
		overview.Summary.CampaignScheduled,
		overview.Summary.CampaignPaused,
		overview.Summary.CampaignReservedUnits,
		overview.Summary.CampaignConsumedUnits,
		overview.Summary.AssetTotal,
		overview.Summary.AssetIssued,
		overview.Summary.AssetActive,
		overview.Summary.AssetReservedUses,
		overview.Summary.AssetConsumedUses,
		overview.Summary.PointsAccountCount,
		overview.Summary.PointsAvailable,
		overview.Summary.PointsFrozen,
		overview.Summary.PointsDebt,
		overview.Summary.ConsentOptedIn,
		overview.Summary.ConsentOptedOut,
	) {
		return gerror.New("marketing overview contains a negative summary value")
	}
	for _, currency := range overview.Currencies {
		if !currencyPattern.MatchString(currency.CurrencyCode) ||
			!nonNegativeMarketingOverviewValues(
				currency.GrantCount,
				currency.PendingMinor,
				currency.AvailableMinor,
				currency.FrozenMinor,
				currency.ConsumedMinor,
				currency.ExpiredMinor,
				currency.ReversedMinor,
			) {
			return gerror.New("marketing overview contains an invalid currency")
		}
	}
	for _, campaign := range overview.RecentCampaigns {
		if campaign.CampaignNo == "" || campaign.StartsAt.IsZero() || campaign.EndsAt.IsZero() ||
			campaign.UpdatedAt.IsZero() || campaign.EndsAt.Before(campaign.StartsAt) ||
			!nonNegativeMarketingOverviewValues(campaign.GlobalReservedCount, campaign.GlobalConsumedCount) ||
			(campaign.GlobalQuotaLimit != nil && *campaign.GlobalQuotaLimit < 0) {
			return gerror.New("marketing overview contains an invalid campaign")
		}
	}
	for _, grant := range overview.RecentGrants {
		if grant.GrantNo == "" || grant.CurrencyCode == "" ||
			!currencyPattern.MatchString(grant.CurrencyCode) || grant.CreatedAt.IsZero() ||
			grant.UpdatedAt.IsZero() || !nonNegativeMarketingOverviewValues(
			grant.OriginalMinor,
			grant.PendingMinor,
			grant.AvailableMinor,
			grant.FrozenMinor,
			grant.ConsumedMinor,
			grant.ExpiredMinor,
			grant.ReversedMinor,
		) {
			return gerror.New("marketing overview contains an invalid value grant")
		}
	}
	payload, err := json.Marshal(overview)
	if err != nil {
		return gerror.Wrap(err, "encode marketing overview")
	}
	if len(payload) > maximumMarketingOverviewBytes {
		return gerror.New("marketing overview exceeded response byte limit")
	}
	return nil
}

// nonNegativeMarketingOverviewValues verifies database counters and amounts remain non-negative.
func nonNegativeMarketingOverviewValues(values ...int64) bool {
	for _, value := range values {
		if value < 0 {
			return false
		}
	}
	return true
}

// isNilMarketingManagementDependency rejects interface values containing typed nil pointers.
func isNilMarketingManagementDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
