// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAffiliateAttribution is the golang structure of table plugin_idc_core_sales_affiliate_attribution for DAO operations like Where/Data.
type SalesAffiliateAttribution struct {
	g.Meta             `orm:"table:plugin_idc_core_sales_affiliate_attribution, do:true"`
	Id                 any        //
	TenantId           any        //
	AttributionNo      any        //
	LinkNo             any        //
	TouchNo            any        //
	ReferrerCustomerNo any        //
	ReferredCustomerNo any        //
	PolicyCode         any        //
	StatusCode         any        //
	RiskReasonCode     *string    //
	EvidenceDigest     any        //
	AttributedAt       *time.Time //
	ExpiresAt          *time.Time //
}
