// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteSnapshot is the golang structure of table plugin_idc_core_commerce_quote_snapshot for DAO operations like Where/Data.
type CommerceQuoteSnapshot struct {
	g.Meta                 `orm:"table:plugin_idc_core_commerce_quote_snapshot, do:true"`
	Id                     any        //
	TenantId               any        //
	QuoteNo                any        //
	CartNo                 *string    //
	CustomerNo             *string    //
	CustomerSubjectVersion *int64     //
	PurposeCode            any        //
	SourceCode             any        //
	CurrencyCode           any        //
	ItemCount              any        //
	SubtotalMinor          any        //
	DiscountMinor          any        //
	NetAmountMinor         any        //
	TaxAmountMinor         any        //
	TotalAmountMinor       any        //
	InputDigest            any        //
	PricingRuleDigest      any        //
	EligibilityDigest      any        //
	InventoryDigest        any        //
	ContentDigest          any        //
	IssuedAt               *time.Time //
	ExpiresAt              *time.Time //
	CreatedBy              any        //
}
