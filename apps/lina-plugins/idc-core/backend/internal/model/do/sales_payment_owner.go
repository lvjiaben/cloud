// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesPaymentOwner is the golang structure of table plugin_idc_core_sales_payment_owner for DAO operations like Where/Data.
type SalesPaymentOwner struct {
	g.Meta                 `orm:"table:plugin_idc_core_sales_payment_owner, do:true"`
	Id                     any        //
	TenantId               any        //
	InvoiceNo              any        //
	BeneficiaryTypeCode    any        //
	BeneficiaryReference   any        //
	SalesAssignmentNo      *string    //
	AffiliateAttributionNo *string    //
	ManualReferrerId       *int64     //
	CreatedAt              *time.Time //
}
