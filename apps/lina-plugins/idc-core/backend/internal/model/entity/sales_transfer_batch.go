// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesTransferBatch is the golang structure for table sales_transfer_batch.
type SalesTransferBatch struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	BatchNo            string     `json:"batchNo"            orm:"batch_no"             description:""`
	SourceMembershipNo string     `json:"sourceMembershipNo" orm:"source_membership_no" description:""`
	TargetMembershipNo string     `json:"targetMembershipNo" orm:"target_membership_no" description:""`
	RequestedCount     int        `json:"requestedCount"     orm:"requested_count"      description:""`
	SucceededCount     int        `json:"succeededCount"     orm:"succeeded_count"      description:""`
	FailedCount        int        `json:"failedCount"        orm:"failed_count"         description:""`
	ReasonCode         string     `json:"reasonCode"         orm:"reason_code"          description:""`
	RequestedBy        int        `json:"requestedBy"        orm:"requested_by"         description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	CompletedAt        *time.Time `json:"completedAt"        orm:"completed_at"         description:""`
}
