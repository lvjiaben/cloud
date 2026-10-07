// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceFulfillmentUnit is the golang structure of table plugin_idc_core_commerce_fulfillment_unit for DAO operations like Where/Data.
type CommerceFulfillmentUnit struct {
	g.Meta              `orm:"table:plugin_idc_core_commerce_fulfillment_unit, do:true"`
	Id                  any        //
	TenantId            any        //
	FulfillmentNo       any        //
	OrderNo             any        //
	OrderItemNo         any        //
	UnitSequence        any        //
	StatusCode          any        //
	ReservationNo       *string    //
	BusinessOperationNo *string    //
	ServiceNo           *string    //
	LastEvidenceDigest  *string    //
	StartedAt           *time.Time //
	CompletedAt         *time.Time //
	Version             any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
