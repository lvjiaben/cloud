// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerCustomFieldOption is the golang structure for table customer_custom_field_option.
type CustomerCustomFieldOption struct {
	Id          int64      `json:"id"          orm:"id"           description:""`
	TenantId    int        `json:"tenantId"    orm:"tenant_id"    description:""`
	OptionNo    string     `json:"optionNo"    orm:"option_no"    description:""`
	FieldNo     string     `json:"fieldNo"     orm:"field_no"     description:""`
	OptionValue string     `json:"optionValue" orm:"option_value" description:""`
	LabelZhCn   string     `json:"labelZhCn"   orm:"label_zh_cn"  description:""`
	LabelEnUs   string     `json:"labelEnUs"   orm:"label_en_us"  description:""`
	Priority    int        `json:"priority"    orm:"priority"     description:""`
	StatusCode  string     `json:"statusCode"  orm:"status_code"  description:""`
	CreatedAt   *time.Time `json:"createdAt"   orm:"created_at"   description:""`
	UpdatedAt   *time.Time `json:"updatedAt"   orm:"updated_at"   description:""`
}
