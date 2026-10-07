// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogConfigurationGroup is the golang structure of table plugin_idc_core_catalog_configuration_group for DAO operations like Where/Data.
type CatalogConfigurationGroup struct {
	g.Meta            `orm:"table:plugin_idc_core_catalog_configuration_group, do:true"`
	Id                any        //
	TenantId          any        //
	GroupNo           any        //
	VersionNo         any        //
	GroupCode         any        //
	SelectionModeCode any        //
	NameZhCn          any        //
	NameEnUs          any        //
	MinimumSelected   any        //
	MaximumSelected   any        //
	SortOrder         any        //
	CreatedAt         *time.Time //
}
