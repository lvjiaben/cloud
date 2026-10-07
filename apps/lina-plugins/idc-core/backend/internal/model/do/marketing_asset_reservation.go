// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingAssetReservation is the golang structure of table plugin_idc_core_marketing_asset_reservation for DAO operations like Where/Data.
type MarketingAssetReservation struct {
	g.Meta              `orm:"table:plugin_idc_core_marketing_asset_reservation, do:true"`
	Id                  any        //
	TenantId            any        //
	ReservationNo       any        //
	AssetNo             any        //
	CustomerNo          any        //
	QuoteReference      any        //
	CurrencyCode        any        //
	ReservedUses        any        //
	DiscountAmountMinor any        //
	StatusCode          any        //
	ExpiresAt           *time.Time //
	ConsumedAt          *time.Time //
	ReleasedAt          *time.Time //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
	ReversedAt          *time.Time //
}
