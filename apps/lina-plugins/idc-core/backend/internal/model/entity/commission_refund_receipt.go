// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommissionRefundReceipt is the golang structure for table commission_refund_receipt.
type CommissionRefundReceipt struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	EventNo             string     `json:"eventNo"             orm:"event_no"              description:""`
	RefundNo            string     `json:"refundNo"            orm:"refund_no"             description:""`
	PayloadDigest       string     `json:"payloadDigest"       orm:"payload_digest"        description:""`
	ReversalCount       int        `json:"reversalCount"       orm:"reversal_count"        description:""`
	ReversedAmountMinor int64      `json:"reversedAmountMinor" orm:"reversed_amount_minor" description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
