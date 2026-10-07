// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportCustomFieldOption is the golang structure for table support_custom_field_option.
type SupportCustomFieldOption struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int64      `json:"tenantId"   orm:"tenant_id"   description:""`
	OptionNo   string     `json:"optionNo"   orm:"option_no"   description:""`
	FieldNo    string     `json:"fieldNo"    orm:"field_no"    description:""`
	NameZhCn   string     `json:"nameZhCn"   orm:"name_zh_cn"  description:""`
	NameEnUs   string     `json:"nameEnUs"   orm:"name_en_us"  description:""`
	SortOrder  int        `json:"sortOrder"  orm:"sort_order"  description:""`
	StatusCode string     `json:"statusCode" orm:"status_code" description:""`
	Version    int64      `json:"version"    orm:"version"     description:""`
	CreatedBy  int64      `json:"createdBy"  orm:"created_by"  description:""`
	UpdatedBy  int64      `json:"updatedBy"  orm:"updated_by"  description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
