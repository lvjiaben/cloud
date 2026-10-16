// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportContract is the golang structure of table plugin_idc_core_catalog_import_contract for DAO operations like Where/Data.
type CatalogImportContract struct {
	g.Meta              `orm:"table:plugin_idc_core_catalog_import_contract, do:true"`
	Id                  any        //
	TenantId            any        //
	VersionNo           any        //
	InterfaceInstanceNo any        //
	Snapshot            any        //
	SnapshotDigest      any        //
	ReadEvidence        any        //
	CreatedAt           *time.Time //
}
