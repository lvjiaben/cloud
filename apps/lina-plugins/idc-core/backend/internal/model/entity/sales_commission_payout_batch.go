// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionPayoutBatch is the golang structure for table sales_commission_payout_batch.
type SalesCommissionPayoutBatch struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	BatchNo              string     `json:"batchNo"              orm:"batch_no"               description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"          description:""`
	ItemCount            int        `json:"itemCount"            orm:"item_count"             description:""`
	TotalAmountMinor     int64      `json:"totalAmountMinor"     orm:"total_amount_minor"     description:""`
	SucceededCount       int        `json:"succeededCount"       orm:"succeeded_count"        description:""`
	SucceededAmountMinor int64      `json:"succeededAmountMinor" orm:"succeeded_amount_minor" description:""`
	FailedCount          int        `json:"failedCount"          orm:"failed_count"           description:""`
	FailedAmountMinor    int64      `json:"failedAmountMinor"    orm:"failed_amount_minor"    description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	CreatedBy            int        `json:"createdBy"            orm:"created_by"             description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	CompletedAt          *time.Time `json:"completedAt"          orm:"completed_at"           description:""`
}
