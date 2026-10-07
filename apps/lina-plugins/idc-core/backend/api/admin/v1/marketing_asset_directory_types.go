// This file contains promotion asset, points, and granted-value management DTOs.

package v1

type MarketingPromotionAssetItem struct {
	AssetNo         string `json:"assetNo" dc:"Stable coupon or voucher business number" eg:"asset-summer-001"`
	CampaignNo      string `json:"campaignNo" dc:"Owning campaign business number" eg:"campaign-summer-2026"`
	BenefitNo       string `json:"benefitNo" dc:"Owning benefit rule business number" eg:"benefit-summer-voucher"`
	BenefitType     string `json:"benefitType" dc:"Benefit rule type" eg:"voucher"`
	Type            string `json:"type" dc:"Promotion asset type: coupon or voucher" eg:"voucher"`
	CustomerNo      string `json:"customerNo" dc:"Owning customer business number, or empty before claim" eg:"customer-001"`
	CurrencyCode    string `json:"currencyCode" dc:"Explicit uppercase three-letter currency" eg:"CNY"`
	FaceAmountMinor *int64 `json:"faceAmountMinor" dc:"Fixed face amount in the currency's smallest unit, or null" eg:"1000"`
	RateBasisPoints *int   `json:"rateBasisPoints" dc:"Percentage discount in basis points, or null" eg:"500"`
	UsageLimit      int    `json:"usageLimit" dc:"Maximum permitted uses" eg:"1"`
	ReservedUses    int    `json:"reservedUses" dc:"Uses currently reserved by quotes" eg:"0"`
	ConsumedUses    int    `json:"consumedUses" dc:"Uses already consumed" eg:"1"`
	Status          string `json:"status" dc:"Asset lifecycle status" eg:"active"`
	ValidFrom       int64  `json:"validFrom" dc:"Validity start in Unix milliseconds" eg:"1767225600000"`
	ValidUntil      int64  `json:"validUntil" dc:"Validity end in Unix milliseconds" eg:"1769817600000"`
	ClaimedAt       *int64 `json:"claimedAt" dc:"Claim time in Unix milliseconds, or null" eg:"1767301200000"`
	CreatedAt       int64  `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1767000000000"`
	UpdatedAt       int64  `json:"updatedAt" dc:"Last update time in Unix milliseconds" eg:"1767301200000"`
}

type MarketingAssetReservationItem struct {
	ReservationNo       string `json:"reservationNo" dc:"Stable asset reservation business number" eg:"asset-reservation-001"`
	ReservedUses        int    `json:"reservedUses" dc:"Uses reserved by this quote" eg:"1"`
	DiscountAmountMinor int64  `json:"discountAmountMinor" dc:"Quoted discount in the asset currency's smallest unit" eg:"1000"`
	Status              string `json:"status" dc:"Reservation lifecycle status" eg:"consumed"`
	ExpiresAt           int64  `json:"expiresAt" dc:"Reservation expiry in Unix milliseconds" eg:"1767303000000"`
	CreatedAt           int64  `json:"createdAt" dc:"Reservation creation time in Unix milliseconds" eg:"1767301200000"`
}

type MarketingPointsAccountItem struct {
	CustomerNo      string `json:"customerNo" dc:"Owning customer business number" eg:"customer-001"`
	AvailablePoints int64  `json:"availablePoints" dc:"Available non-monetary points" eg:"1200"`
	FrozenPoints    int64  `json:"frozenPoints" dc:"Frozen non-monetary points" eg:"100"`
	DebtPoints      int64  `json:"debtPoints" dc:"Outstanding non-monetary points debt" eg:"0"`
	Version         int64  `json:"version" dc:"Optimistic account version" eg:"7"`
	LotCount        int64  `json:"lotCount" dc:"Historical points lot count" eg:"4"`
	EntryCount      int64  `json:"entryCount" dc:"Historical points entry count" eg:"12"`
	CreatedAt       int64  `json:"createdAt" dc:"Account creation time in Unix milliseconds" eg:"1767000000000"`
	UpdatedAt       int64  `json:"updatedAt" dc:"Last account update in Unix milliseconds" eg:"1767301200000"`
}

type MarketingPointsLotItem struct {
	LotNo                   string `json:"lotNo" dc:"Stable points lot business number" eg:"points-lot-001"`
	CampaignNo              string `json:"campaignNo" dc:"Source campaign number, or empty" eg:"campaign-points-2026"`
	SourceType              string `json:"sourceType" dc:"Typed lot source" eg:"order"`
	SourceReference         string `json:"sourceReference" dc:"Idempotent source business reference" eg:"order-001"`
	OriginalPoints          int64  `json:"originalPoints" dc:"Original non-monetary points quantity" eg:"1000"`
	AvailablePoints         int64  `json:"availablePoints" dc:"Available points in this lot" eg:"700"`
	FrozenPoints            int64  `json:"frozenPoints" dc:"Frozen points in this lot" eg:"100"`
	ConsumedPoints          int64  `json:"consumedPoints" dc:"Consumed points in this lot" eg:"200"`
	ExpiredPoints           int64  `json:"expiredPoints" dc:"Expired points in this lot" eg:"0"`
	ReversedPoints          int64  `json:"reversedPoints" dc:"Reversed points in this lot" eg:"0"`
	ReversalShortfallPoints int64  `json:"reversalShortfallPoints" dc:"Consumed reversal shortfall represented as account debt" eg:"0"`
	ExpiresAt               *int64 `json:"expiresAt" dc:"Lot expiry in Unix milliseconds, or null" eg:"1798761600000"`
	Status                  string `json:"status" dc:"Points lot lifecycle status" eg:"available"`
	CreatedAt               int64  `json:"createdAt" dc:"Lot creation time in Unix milliseconds" eg:"1767000000000"`
}

type MarketingPointsEntryItem struct {
	EntryNo           string `json:"entryNo" dc:"Stable points entry business number" eg:"points-entry-001"`
	LotNo             string `json:"lotNo" dc:"Affected points lot number, or empty" eg:"points-lot-001"`
	Type              string `json:"type" dc:"Typed points movement" eg:"earn"`
	AvailableDelta    int64  `json:"availableDelta" dc:"Signed available-points change" eg:"1000"`
	FrozenDelta       int64  `json:"frozenDelta" dc:"Signed frozen-points change" eg:"0"`
	DebtDelta         int64  `json:"debtDelta" dc:"Signed points-debt change" eg:"0"`
	SourceType        string `json:"sourceType" dc:"Typed movement source" eg:"order"`
	SourceReference   string `json:"sourceReference" dc:"Idempotent source business reference" eg:"order-001"`
	ReversalOfEntryNo string `json:"reversalOfEntryNo" dc:"Original entry number for a reversal, or empty" eg:""`
	CreatedAt         int64  `json:"createdAt" dc:"Entry creation time in Unix milliseconds" eg:"1767000000000"`
}

type MarketingValueGrantItem struct {
	GrantNo                   string `json:"grantNo" dc:"Stable granted-value business number" eg:"grant-001"`
	Type                      string `json:"type" dc:"Grant type: recharge_bonus or cashback" eg:"recharge_bonus"`
	CampaignNo                string `json:"campaignNo" dc:"Owning campaign business number" eg:"campaign-recharge-2026"`
	BenefitNo                 string `json:"benefitNo" dc:"Owning benefit rule business number" eg:"benefit-recharge-2026"`
	CustomerNo                string `json:"customerNo" dc:"Owning customer business number" eg:"customer-001"`
	CurrencyCode              string `json:"currencyCode" dc:"Explicit uppercase three-letter currency" eg:"CNY"`
	SourceReference           string `json:"sourceReference" dc:"Idempotent source business reference" eg:"payment-001"`
	OriginalAmountMinor       int64  `json:"originalAmountMinor" dc:"Original amount in the explicit currency's smallest unit" eg:"1000"`
	PendingAmountMinor        int64  `json:"pendingAmountMinor" dc:"Amount pending financial posting" eg:"0"`
	AvailableAmountMinor      int64  `json:"availableAmountMinor" dc:"Available amount" eg:"700"`
	FrozenAmountMinor         int64  `json:"frozenAmountMinor" dc:"Frozen amount" eg:"100"`
	ConsumedAmountMinor       int64  `json:"consumedAmountMinor" dc:"Consumed amount" eg:"200"`
	ExpiredAmountMinor        int64  `json:"expiredAmountMinor" dc:"Expired amount" eg:"0"`
	ReversedAmountMinor       int64  `json:"reversedAmountMinor" dc:"Reversed amount" eg:"0"`
	Withdrawable              bool   `json:"withdrawable" dc:"Whether this granted value may be withdrawn" eg:"false"`
	GlobalQuotaReserved       bool   `json:"globalQuotaReserved" dc:"Whether campaign quota remains reserved" eg:"false"`
	CustomerQuotaReserved     bool   `json:"customerQuotaReserved" dc:"Whether customer quota remains reserved" eg:"false"`
	ExpiresAt                 *int64 `json:"expiresAt" dc:"Grant expiry in Unix milliseconds, or null" eg:"1798761600000"`
	Status                    string `json:"status" dc:"Grant lifecycle status" eg:"available"`
	FinancialPostingReference string `json:"financialPostingReference" dc:"Immutable financial posting business reference, or empty before posting" eg:"ledger-entry-001"`
	CreatedAt                 int64  `json:"createdAt" dc:"Grant creation time in Unix milliseconds" eg:"1767000000000"`
	UpdatedAt                 int64  `json:"updatedAt" dc:"Last grant update in Unix milliseconds" eg:"1767301200000"`
}

type MarketingValueEntryItem struct {
	EntryNo             string `json:"entryNo" dc:"Stable granted-value entry business number" eg:"value-entry-001"`
	Type                string `json:"type" dc:"Typed granted-value movement" eg:"consume"`
	PendingDeltaMinor   int64  `json:"pendingDeltaMinor" dc:"Signed pending amount change" eg:"0"`
	AvailableDeltaMinor int64  `json:"availableDeltaMinor" dc:"Signed available amount change" eg:"0"`
	FrozenDeltaMinor    int64  `json:"frozenDeltaMinor" dc:"Signed frozen amount change" eg:"-200"`
	ConsumedDeltaMinor  int64  `json:"consumedDeltaMinor" dc:"Signed consumed amount change" eg:"200"`
	ExpiredDeltaMinor   int64  `json:"expiredDeltaMinor" dc:"Signed expired amount change" eg:"0"`
	ReversedDeltaMinor  int64  `json:"reversedDeltaMinor" dc:"Signed reversed amount change" eg:"0"`
	SourceType          string `json:"sourceType" dc:"Typed movement source" eg:"order"`
	SourceReference     string `json:"sourceReference" dc:"Idempotent source business reference" eg:"order-001"`
	ReversalOfEntryNo   string `json:"reversalOfEntryNo" dc:"Original entry number for a reversal, or empty" eg:""`
	CreatedAt           int64  `json:"createdAt" dc:"Entry creation time in Unix milliseconds" eg:"1767301200000"`
}
