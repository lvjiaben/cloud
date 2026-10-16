// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCustomerReferrer is the golang structure of table plugin_idc_core_sales_customer_referrer for DAO operations like Where/Data.
type SalesCustomerReferrer struct {
	g.Meta             `orm:"table:plugin_idc_core_sales_customer_referrer, do:true"`
	Id                 any        //
	TenantId           any        //
	CustomerNo         any        //
	ReferrerCustomerNo any        //
	Version            any        //
	EffectiveAt        *time.Time //
	CreatedBy          any        //
}
