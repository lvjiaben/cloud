// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceCart is the golang structure of table plugin_idc_core_commerce_cart for DAO operations like Where/Data.
type CommerceCart struct {
	g.Meta           `orm:"table:plugin_idc_core_commerce_cart, do:true"`
	Id               any        //
	TenantId         any        //
	CartNo           any        //
	CustomerNo       *string    //
	GuestOwnerDigest *string    //
	CurrencyCode     any        //
	StatusCode       any        //
	LatestQuoteNo    *string    //
	MergedIntoCartNo *string    //
	ExpiresAt        *time.Time //
	ClosedAt         *time.Time //
	Version          any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
