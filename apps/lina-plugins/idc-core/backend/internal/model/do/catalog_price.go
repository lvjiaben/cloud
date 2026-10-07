// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogPrice is the golang structure of table plugin_idc_core_catalog_price for DAO operations like Where/Data.
type CatalogPrice struct {
	g.Meta                        `orm:"table:plugin_idc_core_catalog_price, do:true"`
	Id                            any        //
	TenantId                      any        //
	PriceNo                       any        //
	VersionNo                     any        //
	BillingRuleNo                 any        //
	OptionNo                      *string    // Null for the product base price, otherwise a version-owned option business number
	CurrencyCode                  any        //
	AmountMinor                   any        //
	SetupFeeMinor                 any        //
	RenewalAmountMinor            *int64     //
	MinimumQuantity               any        //
	MaximumQuantity               *int64     //
	StatusCode                    any        //
	CreatedAt                     *time.Time //
	ConfigurationPriceDimensionNo any        //
}
