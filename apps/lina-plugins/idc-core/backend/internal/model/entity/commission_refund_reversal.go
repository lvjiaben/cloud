// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommissionRefundReversal is the golang structure for table commission_refund_reversal.
type CommissionRefundReversal struct {
	Id            int64      `json:"id"            orm:"id"              description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"       description:""`
	ReversalNo    string     `json:"reversalNo"    orm:"reversal_no"     description:""`
	CommissionNo  string     `json:"commissionNo"  orm:"commission_no"   description:""`
	RefundNo      string     `json:"refundNo"      orm:"refund_no"       description:""`
	SourceEventNo string     `json:"sourceEventNo" orm:"source_event_no" description:""`
	AmountMinor   int64      `json:"amountMinor"   orm:"amount_minor"    description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"      description:""`
}
