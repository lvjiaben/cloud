// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingConditionValue is the golang structure for table marketing_condition_value.
type MarketingConditionValue struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	ConditionNo  string     `json:"conditionNo"  orm:"condition_no"  description:""`
	ValueText    *string    `json:"valueText"    orm:"value_text"    description:""`
	ValueInteger *int64     `json:"valueInteger" orm:"value_integer" description:""`
	SortOrder    int        `json:"sortOrder"    orm:"sort_order"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
