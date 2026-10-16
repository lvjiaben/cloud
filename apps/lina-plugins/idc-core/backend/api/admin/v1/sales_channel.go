package v1

import "github.com/gogf/gf/v2/frame/g"

// SalesChannelOfferEvaluateReq evaluates the current agent rules without
// changing an order. The returned snapshot is what checkout must freeze.
type SalesChannelOfferEvaluateReq struct {
	g.Meta             `path:"/admin/sales-channel/agent/offer/evaluate" method:"post" tags:"IDC Sales Channel" summary:"Evaluate agent offer" dc:"Evaluate one bounded agent offer and return the immutable price snapshot." permission:"idc:admin:sales-channel:offer:view"`
	CustomerNo         string `json:"customerNo" v:"required|max-length:40" dc:"Customer short number" eg:"C10001"`
	ProductNo          string `json:"productNo" v:"required|max-length:40" dc:"Generic product number" eg:"product.demo"`
	CurrencyCode       string `json:"currencyCode" v:"required|length:3,3" dc:"ISO currency code" eg:"CNY"`
	BasePriceMinor     int64  `json:"basePriceMinor" v:"min:0" dc:"Public price in minor units" eg:"10000"`
	CostMinor          int64  `json:"costMinor" v:"min:0" dc:"Cost floor in minor units" eg:"6000"`
	ExistingCreditUsed int64  `json:"existingCreditUsed" v:"min:0" dc:"Already used credit in minor units" eg:"0"`
	NowUnixMilli       int64  `json:"now" v:"min:1" dc:"Evaluation time as Unix milliseconds" eg:"1787702400000"`
}

// SalesChannelQualificationTransitionValidateReq validates one append-only
// agent qualification transition before the caller persists its event.
type SalesChannelQualificationTransitionValidateReq struct {
	g.Meta               `path:"/admin/sales-channel/agent/qualification/transition/validate" method:"post" tags:"IDC Sales Channel" summary:"Validate agent qualification transition" dc:"Validate an agent qualification state change without mutating stored records." permission:"idc:admin:sales-channel:qualification:manage"`
	NowUnixMilli         int64  `json:"now" v:"min:1" dc:"Validation time as Unix milliseconds" eg:"1787702400000"`
	CurrentStatus        string `json:"currentStatus" dc:"Current qualification status" eg:"active"`
	NextStatus           string `json:"nextStatus" v:"required" dc:"Requested qualification status" eg:"suspended"`
	CurrentLevelNo       string `json:"currentLevelNo" dc:"Current agent level number" eg:"agent.standard"`
	NextLevelNo          string `json:"nextLevelNo" v:"required|max-length:40" dc:"Requested agent level number" eg:"agent.pro"`
	EffectiveAtUnixMilli int64  `json:"effectiveAt" v:"min:1" dc:"Transition effective time as Unix milliseconds" eg:"1787702400000"`
	ExpiresAtUnixMilli   *int64 `json:"expiresAt,omitempty" dc:"Optional qualification expiry as Unix milliseconds"`
	ReasonCode           string `json:"reasonCode" v:"required|max-length:64" dc:"Business reason for the transition" eg:"manual_review"`
	ActorReference       string `json:"actorReference" v:"required|max-length:128" dc:"Opaque actor reference" eg:"staff_1001"`
	EvidenceDigest       string `json:"evidenceDigest" v:"required|length:64" dc:"Lowercase SHA-256 evidence digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type SalesChannelQualificationTransitionValidateRes struct {
	Allowed    bool   `json:"allowed" dc:"Whether the transition is allowed"`
	ReasonCode string `json:"reasonCode,omitempty" dc:"Machine-readable rejection reason"`
}

type SalesChannelOfferEvaluateRes struct {
	Eligible             bool                       `json:"eligible" dc:"Whether the agent offer can be used"`
	PriceMinor           int64                      `json:"priceMinor" dc:"Calculated price in minor units"`
	RemainingCreditMinor *int64                     `json:"remainingCreditMinor,omitempty" dc:"Remaining credit after this quote"`
	ReasonCode           string                     `json:"reasonCode,omitempty" dc:"Machine-readable rejection reason"`
	Snapshot             *SalesChannelPriceSnapshot `json:"snapshot,omitempty" dc:"Frozen rule and price snapshot"`
}

type SalesChannelPriceSnapshot struct {
	QualificationNo      string `json:"qualificationNo"`
	QualificationVersion int64  `json:"qualificationVersion"`
	ProductRuleNo        string `json:"productRuleNo"`
	ProductRuleVersion   int64  `json:"productRuleVersion"`
	PriceRuleNo          string `json:"priceRuleNo"`
	PriceRuleVersion     int64  `json:"priceRuleVersion"`
	CurrencyCode         string `json:"currencyCode"`
	BasePriceMinor       int64  `json:"basePriceMinor"`
	PriceMinor           int64  `json:"priceMinor"`
}

type SalesChannelAffiliateAttributeReq struct {
	g.Meta                   `path:"/admin/sales-channel/affiliate/attribute" method:"post" tags:"IDC Sales Channel" summary:"Evaluate affiliate attribution" permission:"idc:admin:sales-channel:affiliate:manage"`
	NowUnixMilli             int64  `json:"now" v:"min:1"`
	LinkNo                   string `json:"linkNo" v:"required|max-length:40"`
	OwnerCustomerNo          string `json:"ownerCustomerNo" v:"required|max-length:40"`
	PolicyCode               string `json:"policyCode" v:"required|in:first_touch,last_touch"`
	LinkStatus               string `json:"linkStatus" v:"required"`
	LinkExpiresAtUnixMilli   *int64 `json:"linkExpiresAt,omitempty"`
	TouchNo                  string `json:"touchNo" v:"required|max-length:40"`
	TouchOccurredAtUnixMilli int64  `json:"touchOccurredAt" v:"min:1"`
	TouchExpiresAtUnixMilli  *int64 `json:"touchExpiresAt,omitempty"`
	ReferredCustomerNo       string `json:"referredCustomerNo" v:"required|max-length:40"`
}

type SalesChannelAffiliateAttributeRes struct {
	StatusCode         string `json:"statusCode"`
	RiskCode           string `json:"riskCode,omitempty"`
	PolicyCode         string `json:"policyCode"`
	ReferrerCustomerNo string `json:"referrerCustomerNo"`
	ReferredCustomerNo string `json:"referredCustomerNo"`
}

type SalesChannelCommissionCalculateReq struct {
	g.Meta               `path:"/admin/sales-channel/commission/calculate" method:"post" tags:"IDC Sales Channel" summary:"Calculate commission" permission:"idc:admin:sales-channel:commission:view"`
	NowUnixMilli         int64                              `json:"now" v:"min:1"`
	OperationTypeCode    string                             `json:"operationTypeCode" v:"required|max-length:32"`
	BeneficiaryTypeCode  string                             `json:"beneficiaryTypeCode" v:"required|max-length:32"`
	ProductNo            string                             `json:"productNo" v:"max-length:40"`
	CurrencyCode         string                             `json:"currencyCode" v:"required|length:3,3"`
	BaseAmountMinor      int64                              `json:"baseAmountMinor" v:"min:0"`
	RuleNo               string                             `json:"ruleNo" v:"required|max-length:40"`
	CalculationTypeCode  string                             `json:"calculationTypeCode" v:"required|in:fixed,rate,tiered"`
	FixedAmountMinor     *int64                             `json:"fixedAmountMinor,omitempty"`
	RateBasisPoints      *int                               `json:"rateBasisPoints,omitempty"`
	CapAmountMinor       *int64                             `json:"capAmountMinor,omitempty"`
	MaturityDays         int                                `json:"maturityDays" v:"min:0|max:3650"`
	RuleVersion          int64                              `json:"ruleVersion" v:"min:1"`
	EffectiveAtUnixMilli int64                              `json:"effectiveAt" v:"min:1"`
	ExpiresAtUnixMilli   *int64                             `json:"expiresAt,omitempty"`
	Tiers                []*SalesChannelCommissionTierInput `json:"tiers,omitempty"`
}

type SalesChannelCommissionTierInput struct {
	TierNo          string `json:"tierNo" v:"required|max-length:40"`
	LowerBoundMinor int64  `json:"lowerBoundMinor" v:"min:0"`
	UpperBoundMinor *int64 `json:"upperBoundMinor,omitempty"`
	RateBasisPoints int    `json:"rateBasisPoints" v:"min:0|max:10000"`
}

type SalesChannelCommissionCalculateRes struct {
	AmountMinor        int64  `json:"amountMinor"`
	MaturesAtUnixMilli int64  `json:"maturesAt"`
	RuleNo             string `json:"ruleNo"`
	RuleVersion        int64  `json:"ruleVersion"`
	TierNo             string `json:"tierNo,omitempty"`
	CurrencyCode       string `json:"currencyCode"`
	OperationTypeCode  string `json:"operationTypeCode"`
}

type SalesChannelOverviewGetReq struct {
	g.Meta `path:"/admin/sales-channel/overview" method:"get" tags:"IDC Sales Channel" summary:"Get sales channel overview" permission:"idc:admin:sales-channel:overview:view"`
}

type SalesChannelOverviewGetRes struct {
	Qualifications    int64                           `json:"qualifications"`
	AffiliateLinks    int64                           `json:"affiliateLinks"`
	Attributions      int64                           `json:"attributions"`
	PendingMaturity   int64                           `json:"pendingMaturity"`
	PendingSettlement int64                           `json:"pendingSettlement"`
	Currencies        []*SalesChannelCurrencyOverview `json:"currencies"`
}

type SalesChannelCustomerReferrerGetReq struct {
	g.Meta     `path:"/admin/sales-channel/customers/{customerNo}/referrer" method:"get" tags:"IDC Sales Channel" summary:"Get customer referrer" dc:"Return the display-safe current eligible referrer after customer visibility has been authorized." permission:"idc:admin:customer:query"`
	CustomerNo string `json:"customerNo" v:"required|max-length:40" dc:"Stable referred customer number"`
}

type SalesChannelCustomerReferrerGetRes struct {
	CustomerNo   string `json:"customerNo,omitempty" dc:"Stable referrer customer number, or empty"`
	CustomerCode string `json:"customerCode,omitempty" dc:"Short display customer identifier, or empty"`
	DisplayName  string `json:"displayName,omitempty" dc:"Display-safe referrer name, or empty"`
}

type SalesChannelSettlementReviewReq struct {
	g.Meta         `path:"/admin/sales-channel/commission/settlements/{settlementNo}/review" method:"post" tags:"IDC Sales Channel" summary:"Release historical commission request" dc:"Only rejected is accepted to release a historical unreviewed request. Approval and new settlement applications are retired." permission:"idc:admin:sales-channel:commission:review"`
	SettlementNo   string `json:"settlementNo" v:"required|max-length:40"`
	DecisionCode   string `json:"decisionCode" v:"required|in:approved,rejected,manual_review"`
	ReasonCode     string `json:"reasonCode" v:"required|max-length:64"`
	EvidenceDigest string `json:"evidenceDigest" v:"required|length:64"`
}
type SalesChannelSettlementReviewRes struct {
	Success bool `json:"success"`
}

type SalesChannelPayoutBatchCreateReq struct {
	g.Meta        `path:"/admin/sales-channel/commission/payout-batches" method:"post" tags:"IDC Sales Channel" summary:"Retired commission payout batch endpoint" dc:"This endpoint always rejects writes. Administrators record offline commission payments in sales management." permission:"idc:admin:sales-channel:commission:payout"`
	CurrencyCode  string   `json:"currencyCode" v:"required|length:3"`
	SettlementNos []string `json:"settlementNos" v:"required"`
}
type SalesChannelPayoutBatchCreateRes struct {
	BatchNo          string `json:"batchNo"`
	CurrencyCode     string `json:"currencyCode"`
	ItemCount        int    `json:"itemCount"`
	TotalAmountMinor int64  `json:"totalAmountMinor"`
}

type SalesChannelPayoutResultReq struct {
	g.Meta            `path:"/admin/sales-channel/commission/payout-batches/{batchNo}/items/{settlementNo}/result" method:"post" tags:"IDC Sales Channel" summary:"Release failed historical commission payout" dc:"Only failed is accepted after confirming a historical payment did not occur. Successful payments and new payout batches are retired." permission:"idc:admin:sales-channel:commission:payout"`
	BatchNo           string `json:"batchNo" v:"required|max-length:40"`
	SettlementNo      string `json:"settlementNo" v:"required|max-length:40"`
	StatusCode        string `json:"statusCode" v:"required|in:succeeded,failed,result_unknown,manual_review"`
	FailureReasonCode string `json:"failureReasonCode,omitempty"`
}
type SalesChannelPayoutResultRes struct {
	Success bool `json:"success"`
}

type SalesChannelCurrencyOverview struct {
	CurrencyCode string `json:"currencyCode"`
	AccruedMinor int64  `json:"accruedMinor"`
	MaturedMinor int64  `json:"maturedMinor"`
}
