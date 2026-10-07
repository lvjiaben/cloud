// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingCustomerQuota is the golang structure of table plugin_idc_core_marketing_customer_quota for DAO operations like Where/Data.
type MarketingCustomerQuota struct {
	g.Meta        `orm:"table:plugin_idc_core_marketing_customer_quota, do:true"`
	Id            any        //
	TenantId      any        //
	CampaignNo    any        //
	CustomerNo    any        //
	QuotaLimit    any        //
	ReservedCount any        //
	ConsumedCount any        //
	Version       any        //
	CreatedAt     *time.Time //
	UpdatedAt     *time.Time //
}
