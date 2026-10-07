// This file defines the bounded administration API for the marketing operations overview.

package v1

import "github.com/gogf/gf/v2/frame/g"

// MarketingOverviewGetReq requests the current local marketing operations snapshot.
type MarketingOverviewGetReq struct {
	g.Meta `path:"/admin/marketing/overview" method:"get" tags:"IDC Marketing Management" summary:"Get marketing operations overview" dc:"Return one bounded local snapshot of segmentation, campaign, promotion asset, points, consent, recharge bonus, and cashback operations without combining monetary currencies." permission:"idc:admin:marketing:overview"`
}

// MarketingOverviewGetRes returns bounded non-monetary and currency-separated signals.
type MarketingOverviewGetRes struct {
	GeneratedAt     int64                       `json:"generatedAt" dc:"PostgreSQL snapshot time in Unix milliseconds" eg:"1767225600000"`
	Summary         *MarketingOverviewSummary   `json:"summary" dc:"Non-monetary marketing totals" eg:"{}"`
	Currencies      []*MarketingCurrencySummary `json:"currencies" dc:"Value-grant balances grouped by exact currency" eg:"[]"`
	RecentCampaigns []*MarketingCampaignSummary `json:"recentCampaigns" dc:"At most twelve recently updated campaigns" eg:"[]"`
	RecentGrants    []*MarketingGrantSummary    `json:"recentGrants" dc:"At most twelve recently updated recharge bonus or cashback grants" eg:"[]"`
}

// MarketingOverviewSummary contains counts and points that do not represent money.
type MarketingOverviewSummary struct {
	SegmentTotal          int64 `json:"segmentTotal" dc:"Total marketing segments" eg:"12"`
	SegmentActive         int64 `json:"segmentActive" dc:"Active marketing segments" eg:"8"`
	SegmentDynamic        int64 `json:"segmentDynamic" dc:"Dynamic marketing segments" eg:"5"`
	ActiveSegmentMembers  int64 `json:"activeSegmentMembers" dc:"Current active segment memberships" eg:"320"`
	CampaignTotal         int64 `json:"campaignTotal" dc:"Total marketing campaigns" eg:"18"`
	CampaignActive        int64 `json:"campaignActive" dc:"Active marketing campaigns" eg:"4"`
	CampaignScheduled     int64 `json:"campaignScheduled" dc:"Scheduled marketing campaigns" eg:"3"`
	CampaignPaused        int64 `json:"campaignPaused" dc:"Paused marketing campaigns" eg:"1"`
	CampaignReservedUnits int64 `json:"campaignReservedUnits" dc:"Campaign quota units currently reserved" eg:"27"`
	CampaignConsumedUnits int64 `json:"campaignConsumedUnits" dc:"Campaign quota units already consumed" eg:"145"`
	AssetTotal            int64 `json:"assetTotal" dc:"Total issued coupon or voucher assets" eg:"900"`
	AssetIssued           int64 `json:"assetIssued" dc:"Coupon or voucher assets waiting for activation" eg:"120"`
	AssetActive           int64 `json:"assetActive" dc:"Active coupon or voucher assets" eg:"640"`
	AssetReservedUses     int64 `json:"assetReservedUses" dc:"Coupon or voucher uses currently reserved" eg:"22"`
	AssetConsumedUses     int64 `json:"assetConsumedUses" dc:"Coupon or voucher uses already consumed" eg:"310"`
	PointsAccountCount    int64 `json:"pointsAccountCount" dc:"Customer points account count" eg:"480"`
	PointsAvailable       int64 `json:"pointsAvailable" dc:"Available non-monetary points" eg:"250000"`
	PointsFrozen          int64 `json:"pointsFrozen" dc:"Frozen non-monetary points" eg:"8000"`
	PointsDebt            int64 `json:"pointsDebt" dc:"Outstanding non-monetary points debt" eg:"600"`
	ConsentOptedIn        int64 `json:"consentOptedIn" dc:"Explicit opted-in marketing channel consents" eg:"510"`
	ConsentOptedOut       int64 `json:"consentOptedOut" dc:"Explicit opted-out marketing channel consents" eg:"46"`
}

// MarketingCurrencySummary keeps granted value inside one exact currency.
type MarketingCurrencySummary struct {
	CurrencyCode   string `json:"currencyCode" dc:"Exact uppercase ISO-style currency code" eg:"CNY"`
	GrantCount     int64  `json:"grantCount" dc:"Recharge bonus or cashback grant count in this exact currency" eg:"24"`
	PendingMinor   int64  `json:"pendingMinor" dc:"Pending amount in the currency's smallest unit" eg:"5000"`
	AvailableMinor int64  `json:"availableMinor" dc:"Available amount in the currency's smallest unit" eg:"8800"`
	FrozenMinor    int64  `json:"frozenMinor" dc:"Frozen amount in the currency's smallest unit" eg:"1200"`
	ConsumedMinor  int64  `json:"consumedMinor" dc:"Consumed amount in the currency's smallest unit" eg:"3200"`
	ExpiredMinor   int64  `json:"expiredMinor" dc:"Expired amount in the currency's smallest unit" eg:"400"`
	ReversedMinor  int64  `json:"reversedMinor" dc:"Reversed amount in the currency's smallest unit" eg:"600"`
}

// MarketingCampaignSummary describes one recently updated campaign.
type MarketingCampaignSummary struct {
	CampaignNo          string `json:"campaignNo" dc:"Stable marketing campaign number" eg:"campaign-summer-2026"`
	NameZhCn            string `json:"nameZhCn" dc:"Simplified Chinese campaign name" eg:"Summer Server Campaign CN"`
	NameEnUs            string `json:"nameEnUs" dc:"English campaign name" eg:"Summer Server Campaign"`
	Type                string `json:"type" dc:"Campaign type: discount, coupon, voucher, points, recharge_bonus, cashback, or customer_price" eg:"cashback"`
	Status              string `json:"status" dc:"Campaign status: draft, scheduled, active, paused, ended, or archived" eg:"active"`
	TargetSegmentNo     string `json:"targetSegmentNo" dc:"Optional stable target segment number" eg:"segment-loyal"`
	TargetSnapshotNo    string `json:"targetSnapshotNo" dc:"Optional immutable target snapshot number" eg:"snapshot-summer-2026"`
	Priority            int    `json:"priority" dc:"Deterministic campaign priority; lower values are evaluated first" eg:"100"`
	GlobalQuotaLimit    *int64 `json:"globalQuotaLimit" dc:"Optional campaign-wide quota limit; null means unlimited" eg:"1000"`
	GlobalReservedCount int64  `json:"globalReservedCount" dc:"Campaign-wide quota units currently reserved" eg:"27"`
	GlobalConsumedCount int64  `json:"globalConsumedCount" dc:"Campaign-wide quota units already consumed" eg:"145"`
	StartsAt            int64  `json:"startsAt" dc:"Campaign start time in Unix milliseconds" eg:"1767225600000"`
	EndsAt              int64  `json:"endsAt" dc:"Campaign end time in Unix milliseconds" eg:"1769817600000"`
	UpdatedAt           int64  `json:"updatedAt" dc:"Last update time in Unix milliseconds" eg:"1767301200000"`
}

// MarketingGrantSummary describes one recent recharge-bonus or cashback grant.
type MarketingGrantSummary struct {
	GrantNo        string `json:"grantNo" dc:"Stable recharge bonus or cashback grant number" eg:"grant-2026-0001"`
	GrantType      string `json:"grantType" dc:"Grant type: recharge_bonus or cashback" eg:"recharge_bonus"`
	CampaignNo     string `json:"campaignNo" dc:"Owning marketing campaign number" eg:"campaign-summer-2026"`
	CustomerNo     string `json:"customerNo" dc:"Owning customer number" eg:"customer-0001"`
	CurrencyCode   string `json:"currencyCode" dc:"Exact uppercase ISO-style currency code" eg:"CNY"`
	OriginalMinor  int64  `json:"originalMinor" dc:"Original grant amount in this currency's smallest unit" eg:"10000"`
	PendingMinor   int64  `json:"pendingMinor" dc:"Pending grant amount in this currency's smallest unit" eg:"0"`
	AvailableMinor int64  `json:"availableMinor" dc:"Available grant amount in this currency's smallest unit" eg:"8800"`
	FrozenMinor    int64  `json:"frozenMinor" dc:"Frozen grant amount in this currency's smallest unit" eg:"1200"`
	ConsumedMinor  int64  `json:"consumedMinor" dc:"Consumed grant amount in this currency's smallest unit" eg:"0"`
	ExpiredMinor   int64  `json:"expiredMinor" dc:"Expired grant amount in this currency's smallest unit" eg:"0"`
	ReversedMinor  int64  `json:"reversedMinor" dc:"Reversed grant amount in this currency's smallest unit" eg:"0"`
	Withdrawable   bool   `json:"withdrawable" dc:"Whether the grant may be withdrawn; recharge bonuses are always false" eg:"false"`
	Status         string `json:"status" dc:"Grant status: pending, available, exhausted, expired, reversed, cancelled, or mixed" eg:"available"`
	ExpiresAt      *int64 `json:"expiresAt" dc:"Optional expiration time in Unix milliseconds" eg:"1769817600000"`
	CreatedAt      int64  `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1767225600000"`
	UpdatedAt      int64  `json:"updatedAt" dc:"Last update time in Unix milliseconds" eg:"1767301200000"`
}
