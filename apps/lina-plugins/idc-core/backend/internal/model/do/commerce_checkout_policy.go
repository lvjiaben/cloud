// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceCheckoutPolicy is the golang structure of table plugin_idc_core_commerce_checkout_policy for DAO operations like Where/Data.
type CommerceCheckoutPolicy struct {
	g.Meta                 `orm:"table:plugin_idc_core_commerce_checkout_policy, do:true"`
	Id                     any        //
	TenantId               any        //
	PolicyNo               any        //
	PaymentValiditySeconds any        //
	Version                any        //
	UpdatedBy              any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
}
