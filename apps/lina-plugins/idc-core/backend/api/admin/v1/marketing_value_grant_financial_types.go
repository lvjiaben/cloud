// This file defines bounded staff projections for reviewed marketing value actions.

package v1

// MarketingValueGrantFinancialMutation is one reviewed posting or reversal result.
type MarketingValueGrantFinancialMutation struct {
	AdjustmentNo        string `json:"adjustmentNo" dc:"Stable financial review request number"`
	GrantNo             string `json:"grantNo" dc:"Stable marketing value grant number"`
	AdjustmentType      string `json:"adjustmentType" dc:"Posting or reversal action type"`
	Status              string `json:"status" dc:"Pending review, rejected, or posted status"`
	CustomerNo          string `json:"customerNo" dc:"Customer business number"`
	CurrencyCode        string `json:"currencyCode" dc:"Explicit three-letter currency code"`
	AmountMinor         string `json:"amountMinor" dc:"Integer amount in the currency minor unit"`
	LedgerTransactionNo string `json:"ledgerTransactionNo" dc:"Ledger transaction number after approval, otherwise empty"`
	Reused              bool   `json:"reused" dc:"Whether the exact idempotent result was reused"`
}

// MarketingValueGrantFinancialAdjustment adds staff review evidence.
type MarketingValueGrantFinancialAdjustment struct {
	MarketingValueGrantFinancialMutation
	RequestedBy int   `json:"requestedBy" dc:"Host user identifier of the requester"`
	CreatedAt   int64 `json:"createdAt" dc:"Request creation time as Unix milliseconds"`
}
