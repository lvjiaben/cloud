// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerFollowUp is the golang structure for table customer_follow_up.
type CustomerFollowUp struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	FollowUpNo     string     `json:"followUpNo"     orm:"follow_up_no"     description:""`
	CustomerNo     string     `json:"customerNo"     orm:"customer_no"      description:""`
	KindCode       string     `json:"kindCode"       orm:"kind_code"        description:""`
	ChannelCode    string     `json:"channelCode"    orm:"channel_code"     description:""`
	StatusCode     string     `json:"statusCode"     orm:"status_code"      description:""`
	Subject        string     `json:"subject"        orm:"subject"          description:""`
	Detail         string     `json:"detail"         orm:"detail"           description:""`
	OutcomeCode    string     `json:"outcomeCode"    orm:"outcome_code"     description:""`
	AssignedUserId int        `json:"assignedUserId" orm:"assigned_user_id" description:""`
	PlannedAt      *time.Time `json:"plannedAt"      orm:"planned_at"       description:""`
	CompletedAt    *time.Time `json:"completedAt"    orm:"completed_at"     description:""`
	CancelledAt    *time.Time `json:"cancelledAt"    orm:"cancelled_at"     description:""`
	CreatedBy      int        `json:"createdBy"      orm:"created_by"       description:""`
	UpdatedBy      int        `json:"updatedBy"      orm:"updated_by"       description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"       description:""`
}
