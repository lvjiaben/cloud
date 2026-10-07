// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceInventoryReservation is the golang structure for table commerce_inventory_reservation.
type CommerceInventoryReservation struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	ReservationNo       string     `json:"reservationNo"       orm:"reservation_no"        description:""`
	OrderNo             string     `json:"orderNo"             orm:"order_no"              description:""`
	OrderItemNo         string     `json:"orderItemNo"         orm:"order_item_no"         description:""`
	BindingNo           string     `json:"bindingNo"           orm:"binding_no"            description:""`
	InventorySnapshotNo string     `json:"inventorySnapshotNo" orm:"inventory_snapshot_no" description:""`
	ReservedQuantity    int        `json:"reservedQuantity"    orm:"reserved_quantity"     description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	ReleaseReasonCode   string     `json:"releaseReasonCode"   orm:"release_reason_code"   description:""`
	ContentDigest       string     `json:"contentDigest"       orm:"content_digest"        description:""`
	HoldExpiresAt       *time.Time `json:"holdExpiresAt"       orm:"hold_expires_at"       description:""`
	CommittedAt         *time.Time `json:"committedAt"         orm:"committed_at"          description:""`
	ConsumedAt          *time.Time `json:"consumedAt"          orm:"consumed_at"           description:""`
	ReleasedAt          *time.Time `json:"releasedAt"          orm:"released_at"           description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
	ConsumedQuantity    int        `json:"consumedQuantity"    orm:"consumed_quantity"     description:""`
	ReleasedQuantity    int        `json:"releasedQuantity"    orm:"released_quantity"     description:""`
	VersionNo           string     `json:"versionNo"           orm:"version_no"            description:""`
	InventoryVersion    int64      `json:"inventoryVersion"    orm:"inventory_version"     description:""`
}
