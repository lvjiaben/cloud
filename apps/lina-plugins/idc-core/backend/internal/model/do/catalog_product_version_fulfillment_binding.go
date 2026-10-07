// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionFulfillmentBinding is the golang structure of table plugin_idc_core_catalog_product_version_fulfillment_binding for DAO operations like Where/Data.
type CatalogProductVersionFulfillmentBinding struct {
	g.Meta                       `orm:"table:plugin_idc_core_catalog_product_version_fulfillment_binding, do:true"`
	Id                           any      //
	TenantId                     any      //
	VersionNo                    any      //
	InterfaceInstanceNo          any      //
	InterfaceDisplayNameSnapshot any      //
	PluginId                     any      //
	ExtensionPointCode           any      //
	PluginReleaseVersion         any      //
	ContractVersion              any      //
	AuthorizedMethods            []string //
	ReleaseDigest                any      //
	DeclarationDigest            any      //
	BoundBy                      any      //
	BoundAt                      any      //
}
