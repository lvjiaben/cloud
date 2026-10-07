package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesChannelOverviewGetReq struct {
	g.Meta `path:"/account/sales-channel/overview" method:"get" tags:"IDC Customer Sales" summary:"Read my referral and commission overview"`
}

type SalesChannelOverviewGetRes struct {
	Links      []*CustomerReferralLink       `json:"links"`
	Currencies []*CustomerCommissionCurrency `json:"currencies"`
}

type CustomerReferralLink struct {
	LinkNo string `json:"linkNo"`
	Policy string `json:"policy"`
	Status string `json:"status"`
}

type CustomerCommissionCurrency struct {
	CurrencyCode   string `json:"currencyCode"`
	PendingMinor   int64  `json:"pendingMinor"`
	AvailableMinor int64  `json:"availableMinor"`
	FrozenMinor    int64  `json:"frozenMinor"`
	PaidMinor      int64  `json:"paidMinor"`
}

type ReferralOverviewGetReq struct {
	g.Meta `path:"/account/sales-channel/referral" method:"get" tags:"IDC Customer Sales" summary:"Read my referral overview" dc:"Return referral link and attribution counters owned by the authenticated customer."`
}

type ReferralLinkCreateReq struct {
	g.Meta                   `path:"/account/sales-channel/referral-links" method:"post" tags:"IDC Customer Sales" summary:"Create my referral link" dc:"Create an idempotent referral link for the authenticated customer."`
	PolicyCode               string `json:"policyCode" v:"required|in:first_touch,last_touch" dc:"Attribution policy" eg:"first_touch"`
	AttributionWindowSeconds int    `json:"attributionWindowSeconds" v:"required|min:60|max:31536000" dc:"Attribution validity window in seconds" eg:"2592000"`
	ExpiresAtUnixMilli       *int64 `json:"expiresAt,omitempty" dc:"Optional link expiry as Unix milliseconds"`
	IdempotencyKey           string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Retry key" eg:"referral-link-20260830-001"`
}

type ReferralLinkCreateRes struct {
	Item   *CustomerReferralLink `json:"item"`
	Reused bool                  `json:"reused"`
}

type ReferralOverviewGetRes struct {
	ReferralCode             string `json:"referralCode"`
	ReferralURL              string `json:"referralUrl"`
	AttributionPolicyCode    string `json:"attributionPolicyCode"`
	AttributionExpiresAt     int64  `json:"attributionExpiresAt"`
	ReferredCustomerCount    int64  `json:"referredCustomerCount"`
	AttributedOrderCount     int64  `json:"attributedOrderCount"`
	QualifiedOrderCount      int64  `json:"qualifiedOrderCount"`
	RejectedAttributionCount int64  `json:"rejectedAttributionCount"`
}

type CommissionOverviewGetReq struct {
	g.Meta       `path:"/account/sales-channel/commission/overview" method:"get" tags:"IDC Customer Sales" summary:"Read my commission overview"`
	CurrencyCode string `json:"currencyCode" v:"max-length:3"`
	Cursor       string `json:"cursor" v:"max-length:128"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:50"`
}

type CommissionOverviewGetRes struct {
	Accounts   []*CustomerCommissionAccount `json:"accounts"`
	Entries    []*CustomerCommissionEntry   `json:"entries"`
	NextCursor string                       `json:"nextCursor"`
}

type CustomerCommissionAccount struct {
	CurrencyCode         string `json:"currencyCode"`
	AvailableAmountMinor int64  `json:"availableAmountMinor"`
	PendingAmountMinor   int64  `json:"pendingAmountMinor"`
	MaturedAmountMinor   int64  `json:"maturedAmountMinor"`
	SettledAmountMinor   int64  `json:"settledAmountMinor"`
	FrozenAmountMinor    int64  `json:"frozenAmountMinor"`
	StatusCode           string `json:"statusCode"`
}

type CustomerCommissionEntry struct {
	EntryNo             string `json:"entryNo"`
	CurrencyCode        string `json:"currencyCode"`
	AmountMinor         int64  `json:"amountMinor"`
	StatusCode          string `json:"statusCode"`
	SourceOrderNo       string `json:"sourceOrderNo"`
	MaturityAt          int64  `json:"maturityAt"`
	ReversedAmountMinor int64  `json:"reversedAmountMinor"`
	CreatedAt           int64  `json:"createdAt"`
}

type CommissionSettlementsListReq struct {
	g.Meta       `path:"/account/sales-channel/commission/settlements" method:"get" tags:"IDC Customer Sales" summary:"List my commission settlements"`
	CurrencyCode string `json:"currencyCode" v:"max-length:3"`
	Cursor       string `json:"cursor" v:"max-length:128"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:50"`
}

type CommissionSettlementsListRes struct {
	Items      []*CustomerCommissionSettlement `json:"items"`
	NextCursor string                          `json:"nextCursor"`
}

type CustomerCommissionSettlement struct {
	SettlementNo      string `json:"settlementNo"`
	CurrencyCode      string `json:"currencyCode"`
	AmountMinor       int64  `json:"amountMinor"`
	StatusCode        string `json:"statusCode"`
	SubmittedAt       int64  `json:"submittedAt"`
	ReviewedAt        int64  `json:"reviewedAt"`
	PaidAt            int64  `json:"paidAt"`
	FailureReasonCode string `json:"failureReasonCode"`
}

type CommissionSettlementRequestCreateReq struct {
	g.Meta         `path:"/account/sales-channel/commission/settlement-requests" method:"post" tags:"IDC Customer Sales" summary:"Request my commission settlement" dc:"Freeze available commission in one currency and submit it for review; customer ownership is derived from the authenticated session."`
	CurrencyCode   string `json:"currencyCode" v:"required|length:3"`
	AmountMinor    int64  `json:"amountMinor" v:"required|min:1"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}

type CommissionSettlementRequestCreateRes struct {
	Item   *CustomerCommissionSettlement `json:"item"`
	Reused bool                          `json:"reused"`
}
