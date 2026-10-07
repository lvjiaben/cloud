// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogConfigurationGroup is the golang structure for table catalog_configuration_group.
type CatalogConfigurationGroup struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	GroupNo           string     `json:"groupNo"           orm:"group_no"            description:""`
	VersionNo         string     `json:"versionNo"         orm:"version_no"          description:""`
	GroupCode         string     `json:"groupCode"         orm:"group_code"          description:""`
	SelectionModeCode string     `json:"selectionModeCode" orm:"selection_mode_code" description:""`
	NameZhCn          string     `json:"nameZhCn"          orm:"name_zh_cn"          description:""`
	NameEnUs          string     `json:"nameEnUs"          orm:"name_en_us"          description:""`
	MinimumSelected   int        `json:"minimumSelected"   orm:"minimum_selected"    description:""`
	MaximumSelected   int        `json:"maximumSelected"   orm:"maximum_selected"    description:""`
	SortOrder         int        `json:"sortOrder"         orm:"sort_order"          description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
