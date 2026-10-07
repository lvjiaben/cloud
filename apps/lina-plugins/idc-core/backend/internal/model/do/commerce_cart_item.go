// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceCartItem is the golang structure of table plugin_idc_core_commerce_cart_item for DAO operations like Where/Data.
type CommerceCartItem struct {
	g.Meta                  `orm:"table:plugin_idc_core_commerce_cart_item, do:true"`
	Id                      any        //
	TenantId                any        //
	CartItemNo              any        //
	CartNo                  any        //
	ProductNo               any        //
	VersionNo               any        //
	BillingRuleNo           any        //
	ConfigurationSnapshotNo any        //
	PurchaseCycles          any        //
	StatusCode              any        //
	IssueCode               any        //
	SortOrder               any        //
	Version                 any        //
	RemovedAt               *time.Time //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
