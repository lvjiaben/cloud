// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesGroup is the golang structure for table sales_group.
type SalesGroup struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	GroupNo    string     `json:"groupNo"    orm:"group_no"    description:""`
	GroupCode  string     `json:"groupCode"  orm:"group_code"  description:""`
	NameZhCn   string     `json:"nameZhCn"   orm:"name_zh_cn"  description:""`
	NameEnUs   string     `json:"nameEnUs"   orm:"name_en_us"  description:""`
	StatusCode string     `json:"statusCode" orm:"status_code" description:""`
	Version    int64      `json:"version"    orm:"version"     description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
