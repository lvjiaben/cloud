// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductImport is the golang structure of table plugin_idc_core_catalog_product_import for DAO operations like Where/Data.
type CatalogProductImport struct {
	g.Meta                       `orm:"table:plugin_idc_core_catalog_product_import, do:true"`
	Id                           any        //
	TenantId                     any        //
	RequestKey                   any        //
	RequestDigest                any        //
	ActorUserId                  any        //
	InterfaceInstanceNo          any        //
	UpstreamReference            any        //
	ProductNo                    *string    //
	VersionNo                    *string    //
	SourceSnapshot               any        //
	MissingConfigurationSnapshot any        //
	PricingIncomplete            any        //
	CreatedAt                    *time.Time //
}
