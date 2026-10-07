// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentLevel is the golang structure for table sales_agent_level.
type SalesAgentLevel struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	LevelNo    string     `json:"levelNo"    orm:"level_no"    description:""`
	LevelCode  string     `json:"levelCode"  orm:"level_code"  description:""`
	NameZhCn   string     `json:"nameZhCn"   orm:"name_zh_cn"  description:""`
	NameEnUs   string     `json:"nameEnUs"   orm:"name_en_us"  description:""`
	SortOrder  int        `json:"sortOrder"  orm:"sort_order"  description:""`
	StatusCode string     `json:"statusCode" orm:"status_code" description:""`
	Version    int64      `json:"version"    orm:"version"     description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
