// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesTransferItem is the golang structure for table sales_transfer_item.
type SalesTransferItem struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	ItemNo            string     `json:"itemNo"            orm:"item_no"             description:""`
	BatchNo           string     `json:"batchNo"           orm:"batch_no"            description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"         description:""`
	OldAssignmentNo   string     `json:"oldAssignmentNo"   orm:"old_assignment_no"   description:""`
	NewAssignmentNo   string     `json:"newAssignmentNo"   orm:"new_assignment_no"   description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"         description:""`
	FailureReasonCode *string    `json:"failureReasonCode" orm:"failure_reason_code" description:""`
	ProcessedAt       *time.Time `json:"processedAt"       orm:"processed_at"        description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
