// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesGroupMember is the golang structure for table sales_group_member.
type SalesGroupMember struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	MembershipNo string     `json:"membershipNo" orm:"membership_no" description:""`
	GroupNo      string     `json:"groupNo"      orm:"group_no"      description:""`
	StaffUserId  int        `json:"staffUserId"  orm:"staff_user_id" description:""`
	RoleCode     string     `json:"roleCode"     orm:"role_code"     description:""`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:""`
	EffectiveAt  *time.Time `json:"effectiveAt"  orm:"effective_at"  description:""`
	EndedAt      *time.Time `json:"endedAt"      orm:"ended_at"      description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
