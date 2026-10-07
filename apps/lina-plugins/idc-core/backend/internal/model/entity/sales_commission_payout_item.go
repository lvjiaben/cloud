// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionPayoutItem is the golang structure for table sales_commission_payout_item.
type SalesCommissionPayoutItem struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	ItemNo            string     `json:"itemNo"            orm:"item_no"             description:""`
	BatchNo           string     `json:"batchNo"           orm:"batch_no"            description:""`
	SettlementNo      string     `json:"settlementNo"      orm:"settlement_no"       description:""`
	CurrencyCode      string     `json:"currencyCode"      orm:"currency_code"       description:""`
	AmountMinor       int64      `json:"amountMinor"       orm:"amount_minor"        description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"         description:""`
	AttemptNo         int        `json:"attemptNo"         orm:"attempt_no"          description:""`
	FailureReasonCode *string    `json:"failureReasonCode" orm:"failure_reason_code" description:""`
	ProcessedAt       *time.Time `json:"processedAt"       orm:"processed_at"        description:""`
}
