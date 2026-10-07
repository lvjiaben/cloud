// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogConfigurationOption is the golang structure of table plugin_idc_core_catalog_configuration_option for DAO operations like Where/Data.
type CatalogConfigurationOption struct {
	g.Meta            `orm:"table:plugin_idc_core_catalog_configuration_option, do:true"`
	Id                any        //
	TenantId          any        //
	OptionNo          any        //
	GroupNo           any        //
	VersionNo         any        //
	OptionCode        any        //
	ComponentTypeCode any        //
	NameZhCn          any        //
	NameEnUs          any        //
	NormalizedValue   any        // Business-normalized value object; raw Provider responses and credentials are forbidden
	MinimumQuantity   any        //
	MaximumQuantity   any        //
	DefaultQuantity   any        //
	SortOrder         any        //
	StatusCode        any        //
	CreatedAt         *time.Time //
}
