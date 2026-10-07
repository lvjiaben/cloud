// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ReconciliationBatch is the golang structure for table reconciliation_batch.
type ReconciliationBatch struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	BatchNo          string     `json:"batchNo"          orm:"batch_no"          description:""`
	ChannelNo        string     `json:"channelNo"        orm:"channel_no"        description:""`
	BusinessDate     time.Time  `json:"businessDate"     orm:"business_date"     description:""`
	SourceDigest     string     `json:"sourceDigest"     orm:"source_digest"     description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"       description:""`
	TotalCount       int        `json:"totalCount"       orm:"total_count"       description:""`
	DifferenceCount  int        `json:"differenceCount"  orm:"difference_count"  description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
	FinishedAt       *time.Time `json:"finishedAt"       orm:"finished_at"       description:""`
	DailyScopeKey    string     `json:"dailyScopeKey"    orm:"daily_scope_key"   description:""`
	MerchantTimezone string     `json:"merchantTimezone" orm:"merchant_timezone" description:""`
	SourceTypeCode   string     `json:"sourceTypeCode"   orm:"source_type_code"  description:""`
	CapabilityCode   string     `json:"capabilityCode"   orm:"capability_code"   description:""`
	PaymentCount     int        `json:"paymentCount"     orm:"payment_count"     description:""`
	RefundCount      int        `json:"refundCount"      orm:"refund_count"      description:""`
	ResultUnknown    bool       `json:"resultUnknown"    orm:"result_unknown"    description:""`
	FailureCode      string     `json:"failureCode"      orm:"failure_code"      description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"        description:""`
	Version          int64      `json:"version"          orm:"version"           description:""`
}
