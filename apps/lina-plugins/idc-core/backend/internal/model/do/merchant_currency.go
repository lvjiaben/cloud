// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MerchantCurrency is the golang structure of table plugin_idc_core_merchant_currency for DAO operations like Where/Data.
type MerchantCurrency struct {
	g.Meta       `orm:"table:plugin_idc_core_merchant_currency, do:true"`
	TenantId     any        //
	CurrencyCode any        // ISO 4217 currency code
	Enabled      any        // Whether new quotes may use this currency
	Version      any        // Optimistic concurrency version
	CreatedBy    any        //
	UpdatedBy    any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
	Deleted      any        // Whether the configuration is removed; retain version and audit continuity
}
