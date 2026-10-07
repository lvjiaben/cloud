// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteChargeSnapshot is the golang structure of table plugin_idc_core_commerce_quote_charge_snapshot for DAO operations like Where/Data.
type CommerceQuoteChargeSnapshot struct {
	g.Meta                        `orm:"table:plugin_idc_core_commerce_quote_charge_snapshot, do:true"`
	Id                            any        //
	TenantId                      any        //
	QuoteChargeNo                 any        //
	QuoteItemNo                   any        //
	CurrencyCode                  any        //
	LineNumber                    any        //
	ChargeKindCode                any        //
	ChargePhaseCode               any        //
	ConfigurationSnapshotNo       *string    //
	OptionNo                      *string    //
	Quantity                      any        //
	ChargeAmountMinor             any        //
	SetupFeeMinor                 any        //
	PricedAmountMinor             any        //
	PriceSourceDigest             any        //
	CreatedAt                     *time.Time //
	ConfigurationPriceDimensionNo any        //
}
