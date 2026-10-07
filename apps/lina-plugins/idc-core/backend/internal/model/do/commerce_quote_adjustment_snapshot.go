// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteAdjustmentSnapshot is the golang structure of table plugin_idc_core_commerce_quote_adjustment_snapshot for DAO operations like Where/Data.
type CommerceQuoteAdjustmentSnapshot struct {
	g.Meta               `orm:"table:plugin_idc_core_commerce_quote_adjustment_snapshot, do:true"`
	Id                   any        //
	TenantId             any        //
	QuoteAdjustmentNo    any        //
	QuoteItemNo          any        //
	CurrencyCode         any        //
	ApplicationOrder     any        //
	AdjustmentTypeCode   any        //
	SourceReference      any        //
	SourceVersionDigest  any        //
	ReservationReference *string    //
	AmountMinor          any        //
	ContentDigest        any        //
	CreatedAt            *time.Time //
}
