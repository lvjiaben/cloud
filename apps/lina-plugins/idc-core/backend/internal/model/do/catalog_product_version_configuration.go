// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionConfiguration is the golang structure of table plugin_idc_core_catalog_product_version_configuration for DAO operations like Where/Data.
type CatalogProductVersionConfiguration struct {
	g.Meta                         `orm:"table:plugin_idc_core_catalog_product_version_configuration, do:true"`
	Id                             any        //
	TenantId                       any        //
	VersionNo                      any        //
	InterfaceInstanceNo            any        //
	InterfaceConfigurationRevision any        //
	ProductSchemaVersion           any        //
	ProductDeclarationSnapshot     any        //
	MerchantValueSnapshot          any        //
	CustomerSchemaVersion          any        //
	CustomerDeclarationSnapshot    any        //
	DeclarationDigest              any        //
	MerchantValueDigest            any        //
	Version                        any        //
	ChangedBy                      any        //
	CreatedAt                      *time.Time //
	ChangedAt                      *time.Time //
	CustomerPresentationSnapshot   any        //
}
