// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogConfigurationOption is the golang structure for table catalog_configuration_option.
type CatalogConfigurationOption struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	OptionNo          string     `json:"optionNo"          orm:"option_no"           description:""`
	GroupNo           string     `json:"groupNo"           orm:"group_no"            description:""`
	VersionNo         string     `json:"versionNo"         orm:"version_no"          description:""`
	OptionCode        string     `json:"optionCode"        orm:"option_code"         description:""`
	ComponentTypeCode string     `json:"componentTypeCode" orm:"component_type_code" description:""`
	NameZhCn          string     `json:"nameZhCn"          orm:"name_zh_cn"          description:""`
	NameEnUs          string     `json:"nameEnUs"          orm:"name_en_us"          description:""`
	NormalizedValue   string     `json:"normalizedValue"   orm:"normalized_value"    description:"Business-normalized value object; raw Provider responses and credentials are forbidden"`
	MinimumQuantity   int        `json:"minimumQuantity"   orm:"minimum_quantity"    description:""`
	MaximumQuantity   int        `json:"maximumQuantity"   orm:"maximum_quantity"    description:""`
	DefaultQuantity   int        `json:"defaultQuantity"   orm:"default_quantity"    description:""`
	SortOrder         int        `json:"sortOrder"         orm:"sort_order"          description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"         description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
