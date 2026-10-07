// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceConfigurationSnapshotItem is the golang structure of table plugin_idc_core_commerce_configuration_snapshot_item for DAO operations like Where/Data.
type CommerceConfigurationSnapshotItem struct {
	g.Meta                  `orm:"table:plugin_idc_core_commerce_configuration_snapshot_item, do:true"`
	Id                      any        //
	TenantId                any        //
	SnapshotItemNo          any        //
	ConfigurationSnapshotNo any        //
	VersionNo               any        //
	GroupNo                 any        //
	OptionNo                any        //
	GroupCode               any        //
	OptionCode              any        //
	ComponentTypeCode       any        //
	GroupNameZhCn           any        //
	GroupNameEnUs           any        //
	OptionNameZhCn          any        //
	OptionNameEnUs          any        //
	NormalizedValue         any        //
	Quantity                any        //
	SortOrder               any        //
	CreatedAt               *time.Time //
}
