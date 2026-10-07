// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialDeliveryIntent is the golang structure of table plugin_idc_core_financial_delivery_intent for DAO operations like Where/Data.
type FinancialDeliveryIntent struct {
	g.Meta              `orm:"table:plugin_idc_core_financial_delivery_intent, do:true"`
	Id                  any        //
	TenantId            any        //
	IntentNo            any        //
	InvoiceNo           any        //
	CustomerNo          any        //
	ContactNo           any        //
	InvoiceVersion      any        //
	NotificationEventNo any        //
	RequestedBy         any        //
	IdempotencyDigest   any        //
	RequestDigest       any        //
	CreatedAt           *time.Time //
}
