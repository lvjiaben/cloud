// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceInventoryReservation is the golang structure of table plugin_idc_core_commerce_inventory_reservation for DAO operations like Where/Data.
type CommerceInventoryReservation struct {
	g.Meta              `orm:"table:plugin_idc_core_commerce_inventory_reservation, do:true"`
	Id                  any        //
	TenantId            any        //
	ReservationNo       any        //
	OrderNo             any        //
	OrderItemNo         any        //
	BindingNo           any        //
	InventorySnapshotNo any        //
	ReservedQuantity    any        //
	StatusCode          any        //
	ReleaseReasonCode   any        //
	ContentDigest       any        //
	HoldExpiresAt       *time.Time //
	CommittedAt         *time.Time //
	ConsumedAt          *time.Time //
	ReleasedAt          *time.Time //
	Version             any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
	ConsumedQuantity    any        //
	ReleasedQuantity    any        //
	VersionNo           any        //
	InventoryVersion    any        //
}
