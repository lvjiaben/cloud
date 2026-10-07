// This file contains shared marketing directory response DTOs.

package v1

type MarketingBilingualDescription struct {
	ZhCn string `json:"zhCn" dc:"Simplified Chinese description" eg:"Loyal customer segment"`
	EnUs string `json:"enUs" dc:"English description" eg:"Loyal customer segment"`
}

type MarketingSegmentSummaryItem struct {
	SegmentNo              string `json:"segmentNo" dc:"Stable segment business number" eg:"segment-loyal"`
	SegmentCode            string `json:"segmentCode" dc:"Merchant-facing unique segment code" eg:"loyal_customers"`
	NameZhCn               string `json:"nameZhCn" dc:"Simplified Chinese segment name" eg:"Loyal Customers CN"`
	NameEnUs               string `json:"nameEnUs" dc:"English segment name" eg:"Loyal Customers"`
	Mode                   string `json:"mode" dc:"Segment mode: static or dynamic" eg:"dynamic"`
	RefreshIntervalMinutes *int   `json:"refreshIntervalMinutes" dc:"Dynamic refresh interval in minutes; null for static segments" eg:"60"`
	Status                 string `json:"status" dc:"Segment status: draft, active, paused, or archived" eg:"active"`
	Version                int64  `json:"version" dc:"Optimistic segment definition version" eg:"3"`
	ConditionCount         int64  `json:"conditionCount" dc:"Typed condition count" eg:"4"`
	ActiveMemberCount      int64  `json:"activeMemberCount" dc:"Current active membership count" eg:"320"`
	LatestSnapshotNo       string `json:"latestSnapshotNo" dc:"Latest immutable snapshot number, or empty when absent" eg:"snapshot-summer-2026"`
	LatestSnapshotStatus   string `json:"latestSnapshotStatus" dc:"Latest snapshot status: building, complete, failed, superseded, or empty" eg:"complete"`
	LatestSnapshotMembers  int64  `json:"latestSnapshotMembers" dc:"Latest snapshot member count" eg:"312"`
	LatestSnapshotAt       *int64 `json:"latestSnapshotAt" dc:"Latest snapshot start time in Unix milliseconds, or null" eg:"1767225600000"`
	CreatedAt              int64  `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1767225600000"`
	UpdatedAt              int64  `json:"updatedAt" dc:"Last update time in Unix milliseconds" eg:"1767301200000"`
}

type MarketingSegmentConditionValue struct {
	Text    string `json:"text" dc:"Text member value, or empty for an integer value" eg:"enterprise"`
	Integer *int64 `json:"integer" dc:"Integer member value, or null for a text value" eg:"30"`
}

type MarketingSegmentCondition struct {
	ConditionNo       string                            `json:"conditionNo" dc:"Stable condition business number" eg:"condition-loyal-age"`
	OrGroupNo         int                               `json:"orGroupNo" dc:"OR group number; conditions inside one group are ANDed" eg:"1"`
	SortOrder         int                               `json:"sortOrder" dc:"Deterministic condition order" eg:"10"`
	Fact              string                            `json:"fact" dc:"Typed customer fact code" eg:"registered_age_days"`
	Operator          string                            `json:"operator" dc:"Typed comparison operator" eg:"gte"`
	ValueKind         string                            `json:"valueKind" dc:"Typed value kind: none, text, integer, boolean, or currency_amount" eg:"integer"`
	ValueText         string                            `json:"valueText" dc:"Single text operand, or empty" eg:"enterprise"`
	ValueIntegerLower *int64                            `json:"valueIntegerLower" dc:"Lower integer or minor-unit operand, or null" eg:"30"`
	ValueIntegerUpper *int64                            `json:"valueIntegerUpper" dc:"Upper integer or minor-unit operand, or null" eg:"365"`
	ValueBoolean      *bool                             `json:"valueBoolean" dc:"Boolean operand, or null" eg:"true"`
	CurrencyCode      string                            `json:"currencyCode" dc:"Explicit currency for currency_amount, or empty" eg:"CNY"`
	Values            []*MarketingSegmentConditionValue `json:"values" dc:"Bounded members for in and not_in operators" eg:"[]"`
}

type MarketingSegmentSnapshot struct {
	SnapshotNo     string `json:"snapshotNo" dc:"Stable immutable snapshot number" eg:"snapshot-summer-2026"`
	SegmentVersion int64  `json:"segmentVersion" dc:"Segment definition version captured by this snapshot" eg:"3"`
	Status         string `json:"status" dc:"Snapshot status: building, complete, failed, or superseded" eg:"complete"`
	MemberCount    int64  `json:"memberCount" dc:"Captured member count" eg:"312"`
	StartedAt      int64  `json:"startedAt" dc:"Snapshot start time in Unix milliseconds" eg:"1767225600000"`
	CompletedAt    *int64 `json:"completedAt" dc:"Snapshot completion time in Unix milliseconds, or null" eg:"1767225660000"`
}

type MarketingCampaignManagementItem struct {
	CampaignNo            string `json:"campaignNo" dc:"Stable campaign business number" eg:"campaign-summer-2026"`
	CampaignCode          string `json:"campaignCode" dc:"Merchant-facing unique campaign code" eg:"summer_2026"`
	NameZhCn              string `json:"nameZhCn" dc:"Simplified Chinese campaign name" eg:"Summer Server Campaign CN"`
	NameEnUs              string `json:"nameEnUs" dc:"English campaign name" eg:"Summer Server Campaign"`
	Type                  string `json:"type" dc:"Typed campaign kind" eg:"cashback"`
	Status                string `json:"status" dc:"Campaign lifecycle status" eg:"active"`
	TargetSegmentNo       string `json:"targetSegmentNo" dc:"Target segment number, or empty" eg:"segment-loyal"`
	TargetSnapshotNo      string `json:"targetSnapshotNo" dc:"Frozen target snapshot number, or empty" eg:"snapshot-summer-2026"`
	Priority              int    `json:"priority" dc:"Deterministic evaluation priority; lower values run first" eg:"100"`
	StackPolicy           string `json:"stackPolicy" dc:"Stack policy: exclusive, stackable, or same_group_exclusive" eg:"same_group_exclusive"`
	GlobalQuotaLimit      *int64 `json:"globalQuotaLimit" dc:"Optional campaign-wide quota limit; null means unlimited" eg:"1000"`
	GlobalReservedCount   int64  `json:"globalReservedCount" dc:"Quota units currently reserved" eg:"27"`
	GlobalConsumedCount   int64  `json:"globalConsumedCount" dc:"Quota units already consumed" eg:"145"`
	PerCustomerQuotaLimit *int   `json:"perCustomerQuotaLimit" dc:"Optional per-customer quota limit" eg:"1"`
	IsNewcomerExclusive   bool   `json:"isNewcomerExclusive" dc:"Whether this campaign is restricted to one claim per verified subject" eg:"true"`
	ScopeCount            int64  `json:"scopeCount" dc:"Product-scope rule count" eg:"2"`
	BenefitCount          int64  `json:"benefitCount" dc:"Benefit rule count" eg:"1"`
	ReservationCount      int64  `json:"reservationCount" dc:"Historical reservation count" eg:"172"`
	StartsAt              int64  `json:"startsAt" dc:"Campaign start time in Unix milliseconds" eg:"1767225600000"`
	EndsAt                int64  `json:"endsAt" dc:"Campaign end time in Unix milliseconds" eg:"1769817600000"`
	CreatedAt             int64  `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1767000000000"`
	UpdatedAt             int64  `json:"updatedAt" dc:"Last update time in Unix milliseconds" eg:"1767301200000"`
}

type MarketingCampaignScope struct {
	Kind          string `json:"kind" dc:"Scope kind: all_products, product, or category" eg:"category"`
	MatchMode     string `json:"matchMode" dc:"Scope match mode: include or exclude" eg:"include"`
	ReferenceCode string `json:"referenceCode" dc:"Referenced product or category business number; empty only for all_products" eg:"product-summer-sale"`
}

type MarketingCampaignBenefit struct {
	BenefitNo            string `json:"benefitNo" dc:"Stable benefit rule number" eg:"benefit-summer-cashback"`
	Type                 string `json:"type" dc:"Benefit type" eg:"cashback"`
	CurrencyCode         string `json:"currencyCode" dc:"Explicit currency for monetary benefits, or empty for points" eg:"CNY"`
	AmountMinor          *int64 `json:"amountMinor" dc:"Fixed amount in the explicit currency's smallest unit, or null" eg:"1000"`
	RateBasisPoints      *int   `json:"rateBasisPoints" dc:"Percentage rate in basis points, or null" eg:"500"`
	MaximumDiscountMinor *int64 `json:"maximumDiscountMinor" dc:"Optional maximum discount in the explicit currency's smallest unit" eg:"5000"`
	MinimumSpendMinor    *int64 `json:"minimumSpendMinor" dc:"Optional minimum spend in the explicit currency's smallest unit" eg:"10000"`
	PointsQuantity       *int64 `json:"pointsQuantity" dc:"Non-monetary points quantity, or null" eg:"1000"`
	PurchasePhase        string `json:"purchasePhase" dc:"Purchase phase: new_purchase, renewal, or both" eg:"both"`
	CycleScope           string `json:"cycleScope" dc:"Billing-cycle scope: first, all, or limited" eg:"first"`
	CycleCount           *int   `json:"cycleCount" dc:"Cycle count for limited scope, or null" eg:"3"`
	Status               string `json:"status" dc:"Benefit status: active or inactive" eg:"active"`
	Version              int64  `json:"version" dc:"Optimistic benefit version" eg:"2"`
}
