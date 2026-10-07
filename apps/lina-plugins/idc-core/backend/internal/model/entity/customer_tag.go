// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerTag is the golang structure for table customer_tag.
type CustomerTag struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	TagNo      string     `json:"tagNo"      orm:"tag_no"      description:""`
	TagCode    string     `json:"tagCode"    orm:"tag_code"    description:""`
	NameZhCn   string     `json:"nameZhCn"   orm:"name_zh_cn"  description:""`
	NameEnUs   string     `json:"nameEnUs"   orm:"name_en_us"  description:""`
	ColorToken string     `json:"colorToken" orm:"color_token" description:""`
	StatusCode string     `json:"statusCode" orm:"status_code" description:""`
	Version    int64      `json:"version"    orm:"version"     description:""`
	CreatedBy  int        `json:"createdBy"  orm:"created_by"  description:""`
	UpdatedBy  int        `json:"updatedBy"  orm:"updated_by"  description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
