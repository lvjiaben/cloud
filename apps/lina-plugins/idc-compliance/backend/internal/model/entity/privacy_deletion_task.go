// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyDeletionTask is the golang structure for table privacy_deletion_task.
type PrivacyDeletionTask struct {
	Id                int64      `json:"id"                orm:"id"                   description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"            description:""`
	TaskNo            string     `json:"taskNo"            orm:"task_no"              description:""`
	DataTypeCode      string     `json:"dataTypeCode"      orm:"data_type_code"       description:""`
	TargetTypeCode    string     `json:"targetTypeCode"    orm:"target_type_code"     description:""`
	TargetNo          string     `json:"targetNo"          orm:"target_no"            description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"          description:""`
	DueAt             *time.Time `json:"dueAt"             orm:"due_at"               description:""`
	HoldReasonCode    string     `json:"holdReasonCode"    orm:"hold_reason_code"     description:""`
	ConfirmedByUserId int        `json:"confirmedByUserId" orm:"confirmed_by_user_id" description:""`
	ConfirmedAt       *time.Time `json:"confirmedAt"       orm:"confirmed_at"         description:""`
	CompletedAt       *time.Time `json:"completedAt"       orm:"completed_at"         description:""`
	Version           int64      `json:"version"           orm:"version"              description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"           description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"           description:""`
}
