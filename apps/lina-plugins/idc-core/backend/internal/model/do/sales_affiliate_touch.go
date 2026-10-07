// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAffiliateTouch is the golang structure of table plugin_idc_core_sales_affiliate_touch for DAO operations like Where/Data.
type SalesAffiliateTouch struct {
	g.Meta         `orm:"table:plugin_idc_core_sales_affiliate_touch, do:true"`
	Id             any        //
	TenantId       any        //
	TouchNo        any        //
	LinkNo         any        //
	VisitorDigest  any        //
	DeviceDigest   any        //
	IpDigest       any        //
	EvidenceDigest any        //
	OccurredAt     *time.Time //
	ExpiresAt      *time.Time //
}
