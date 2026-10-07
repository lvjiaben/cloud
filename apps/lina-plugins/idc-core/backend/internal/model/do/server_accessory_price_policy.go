// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryPricePolicy is the golang structure of table plugin_idc_core_server_accessory_price_policy for DAO operations like Where/Data.
type ServerAccessoryPricePolicy struct {
	g.Meta         `orm:"table:plugin_idc_core_server_accessory_price_policy, do:true"`
	Id             any        //
	TenantId       any        //
	PolicyNo       any        //
	AccessoryKind  any        //
	CurrencyCode   any        //
	CurrentVersion any        //
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
	Enabled        any        //
	StockQuantity  *int64     //
}
