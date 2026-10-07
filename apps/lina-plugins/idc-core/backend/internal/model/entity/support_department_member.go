// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportDepartmentMember is the golang structure for table support_department_member.
type SupportDepartmentMember struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int64      `json:"tenantId"     orm:"tenant_id"     description:""`
	DepartmentNo string     `json:"departmentNo" orm:"department_no" description:""`
	UserId       int64      `json:"userId"       orm:"user_id"       description:""`
	RoleCode     string     `json:"roleCode"     orm:"role_code"     description:""`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
