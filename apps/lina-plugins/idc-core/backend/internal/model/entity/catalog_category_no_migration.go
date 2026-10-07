// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogCategoryNoMigration is the golang structure for table catalog_category_no_migration.
type CatalogCategoryNoMigration struct {
	TenantId      int64      `json:"tenantId"      orm:"tenant_id"       description:""`
	OldCategoryNo string     `json:"oldCategoryNo" orm:"old_category_no" description:""`
	NewCategoryNo string     `json:"newCategoryNo" orm:"new_category_no" description:""`
	MigratedAt    *time.Time `json:"migratedAt"    orm:"migrated_at"     description:""`
}
