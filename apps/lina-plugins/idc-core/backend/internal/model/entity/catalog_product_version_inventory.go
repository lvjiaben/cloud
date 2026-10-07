// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProductVersionInventory is the golang structure for table catalog_product_version_inventory.
type CatalogProductVersionInventory struct {
	VersionNo        string     `json:"versionNo"        orm:"version_no"        description:""`
	TrackingEnabled  bool       `json:"trackingEnabled"  orm:"tracking_enabled"  description:""`
	SellableQuantity int64      `json:"sellableQuantity" orm:"sellable_quantity" description:""`
	Version          int64      `json:"version"          orm:"version"           description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"        description:""`
}
