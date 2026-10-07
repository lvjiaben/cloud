// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceConfigurationSnapshotItem is the golang structure for table commerce_configuration_snapshot_item.
type CommerceConfigurationSnapshotItem struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	SnapshotItemNo          string     `json:"snapshotItemNo"          orm:"snapshot_item_no"          description:""`
	ConfigurationSnapshotNo string     `json:"configurationSnapshotNo" orm:"configuration_snapshot_no" description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                description:""`
	GroupNo                 string     `json:"groupNo"                 orm:"group_no"                  description:""`
	OptionNo                string     `json:"optionNo"                orm:"option_no"                 description:""`
	GroupCode               string     `json:"groupCode"               orm:"group_code"                description:""`
	OptionCode              string     `json:"optionCode"              orm:"option_code"               description:""`
	ComponentTypeCode       string     `json:"componentTypeCode"       orm:"component_type_code"       description:""`
	GroupNameZhCn           string     `json:"groupNameZhCn"           orm:"group_name_zh_cn"          description:""`
	GroupNameEnUs           string     `json:"groupNameEnUs"           orm:"group_name_en_us"          description:""`
	OptionNameZhCn          string     `json:"optionNameZhCn"          orm:"option_name_zh_cn"         description:""`
	OptionNameEnUs          string     `json:"optionNameEnUs"          orm:"option_name_en_us"         description:""`
	NormalizedValue         string     `json:"normalizedValue"         orm:"normalized_value"          description:""`
	Quantity                int        `json:"quantity"                orm:"quantity"                  description:""`
	SortOrder               int        `json:"sortOrder"               orm:"sort_order"                description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
}
