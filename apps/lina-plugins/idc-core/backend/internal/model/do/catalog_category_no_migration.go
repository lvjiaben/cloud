// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogCategoryNoMigration is the golang structure of table plugin_idc_core_catalog_category_no_migration for DAO operations like Where/Data.
type CatalogCategoryNoMigration struct {
	g.Meta        `orm:"table:plugin_idc_core_catalog_category_no_migration, do:true"`
	TenantId      any        //
	OldCategoryNo any        //
	NewCategoryNo any        //
	MigratedAt    *time.Time //
}
