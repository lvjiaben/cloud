// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceQuoteItemInventorySnapshot is the golang structure for table commerce_quote_item_inventory_snapshot.
type CommerceQuoteItemInventorySnapshot struct {
	QuoteItemNo      string     `json:"quoteItemNo"      orm:"quote_item_no"     description:""`
	VersionNo        string     `json:"versionNo"        orm:"version_no"        description:""`
	TrackingEnabled  bool       `json:"trackingEnabled"  orm:"tracking_enabled"  description:""`
	InventoryVersion int64      `json:"inventoryVersion" orm:"inventory_version" description:""`
	SellableQuantity int64      `json:"sellableQuantity" orm:"sellable_quantity" description:""`
	ContentDigest    string     `json:"contentDigest"    orm:"content_digest"    description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
}
