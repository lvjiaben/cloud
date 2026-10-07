// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportDepartment is the golang structure for table support_department.
type SupportDepartment struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int64      `json:"tenantId"           orm:"tenant_id"            description:""`
	DepartmentNo       string     `json:"departmentNo"       orm:"department_no"        description:""`
	NameZhCn           string     `json:"nameZhCn"           orm:"name_zh_cn"           description:""`
	NameEnUs           string     `json:"nameEnUs"           orm:"name_en_us"           description:""`
	ParentDepartmentNo string     `json:"parentDepartmentNo" orm:"parent_department_no" description:""`
	AssignmentMode     string     `json:"assignmentMode"     orm:"assignment_mode"      description:""`
	RoundRobinCursor   int64      `json:"roundRobinCursor"   orm:"round_robin_cursor"   description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
	CreatedBy          int64      `json:"createdBy"          orm:"created_by"           description:""`
	UpdatedBy          int64      `json:"updatedBy"          orm:"updated_by"           description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
}
