// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogInventorySnapshot is the golang structure for table catalog_inventory_snapshot.
type CatalogInventorySnapshot struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	SnapshotNo        string     `json:"snapshotNo"        orm:"snapshot_no"         description:""`
	BindingNo         string     `json:"bindingNo"         orm:"binding_no"          description:""`
	SourceSnapshotNo  string     `json:"sourceSnapshotNo"  orm:"source_snapshot_no"  description:""`
	SourceFingerprint string     `json:"sourceFingerprint" orm:"source_fingerprint"  description:""`
	AvailabilityCode  string     `json:"availabilityCode"  orm:"availability_code"   description:""`
	AvailableQuantity *int64     `json:"availableQuantity" orm:"available_quantity"  description:""`
	SafetyMargin      int64      `json:"safetyMargin"      orm:"safety_margin"       description:""`
	SellableQuantity  *int64     `json:"sellableQuantity"  orm:"sellable_quantity"   description:""`
	ObservedAt        *time.Time `json:"observedAt"        orm:"observed_at"         description:""`
	ExpiresAt         *time.Time `json:"expiresAt"         orm:"expires_at"          description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
	SourceCompletedAt *time.Time `json:"sourceCompletedAt" orm:"source_completed_at" description:"Completion time of the authoritative source snapshot used for this local projection"`
}
