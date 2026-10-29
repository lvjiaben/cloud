// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceConfigurationSnapshot is the golang structure of table plugin_idc_core_commerce_configuration_snapshot for DAO operations like Where/Data.
type CommerceConfigurationSnapshot struct {
	g.Meta                  `orm:"table:plugin_idc_core_commerce_configuration_snapshot, do:true"`
	Id                      any        //
	TenantId                any        //
	ConfigurationSnapshotNo any        //
	ProductNo               any        //
	VersionNo               any        //
	SourceRevisionDigest    any        //
	SelectionDigest         any        //
	SelectedOptionCount     any        //
	TotalSelectedQuantity   any        //
	ContentDigest           any        //
	CreatedAt               *time.Time //
	CustomerValueDigest     any        //
	TypedValueSnapshot      any        // Compiler-produced customer values for native imported pricing; NULL for historical snapshots
	ServerLoginToken        any        //
}
