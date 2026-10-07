// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportIssueCategory is the golang structure for table support_issue_category.
type SupportIssueCategory struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int64      `json:"tenantId"     orm:"tenant_id"     description:""`
	CategoryNo   string     `json:"categoryNo"   orm:"category_no"   description:""`
	NameZhCn     string     `json:"nameZhCn"     orm:"name_zh_cn"    description:""`
	NameEnUs     string     `json:"nameEnUs"     orm:"name_en_us"    description:""`
	DepartmentNo string     `json:"departmentNo" orm:"department_no" description:""`
	PriorityCode string     `json:"priorityCode" orm:"priority_code" description:""`
	SortOrder    int        `json:"sortOrder"    orm:"sort_order"    description:""`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedBy    int64      `json:"createdBy"    orm:"created_by"    description:""`
	UpdatedBy    int64      `json:"updatedBy"    orm:"updated_by"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
