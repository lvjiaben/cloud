// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAffiliateLink is the golang structure of table plugin_idc_core_sales_affiliate_link for DAO operations like Where/Data.
type SalesAffiliateLink struct {
	g.Meta                   `orm:"table:plugin_idc_core_sales_affiliate_link, do:true"`
	Id                       any        //
	TenantId                 any        //
	LinkNo                   any        //
	OwnerCustomerNo          any        //
	CodeDigest               any        //
	AttributionPolicyCode    any        //
	AttributionWindowSeconds any        //
	StatusCode               any        //
	ExpiresAt                *time.Time //
	Version                  any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
