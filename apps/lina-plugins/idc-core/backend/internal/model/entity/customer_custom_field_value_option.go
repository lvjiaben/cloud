// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerCustomFieldValueOption is the golang structure for table customer_custom_field_value_option.
type CustomerCustomFieldValueOption struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	CustomerNo string     `json:"customerNo" orm:"customer_no" description:""`
	FieldNo    string     `json:"fieldNo"    orm:"field_no"    description:""`
	ValueNo    string     `json:"valueNo"    orm:"value_no"    description:""`
	OptionNo   string     `json:"optionNo"   orm:"option_no"   description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
}
