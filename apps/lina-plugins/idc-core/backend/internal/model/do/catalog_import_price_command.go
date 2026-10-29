// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportPriceCommand is the golang structure of table plugin_idc_core_catalog_import_price_command for DAO operations like Where/Data.
type CatalogImportPriceCommand struct {
	g.Meta        `orm:"table:plugin_idc_core_catalog_import_price_command, do:true"`
	Id            any        //
	TenantId      any        //
	ActorUserId   any        //
	RequestKey    any        //
	RequestDigest any        //
	VersionNo     any        //
	CreatedAt     *time.Time //
}
