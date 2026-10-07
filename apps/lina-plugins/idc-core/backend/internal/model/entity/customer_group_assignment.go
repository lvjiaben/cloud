// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerGroupAssignment is the golang structure for table customer_group_assignment.
type CustomerGroupAssignment struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	CustomerNo string     `json:"customerNo" orm:"customer_no" description:""`
	GroupNo    string     `json:"groupNo"    orm:"group_no"    description:""`
	SourceCode string     `json:"sourceCode" orm:"source_code" description:""`
	StatusCode string     `json:"statusCode" orm:"status_code" description:""`
	AssignedBy int        `json:"assignedBy" orm:"assigned_by" description:""`
	AssignedAt *time.Time `json:"assignedAt" orm:"assigned_at" description:""`
	RevokedBy  int        `json:"revokedBy"  orm:"revoked_by"  description:""`
	RevokedAt  *time.Time `json:"revokedAt"  orm:"revoked_at"  description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
