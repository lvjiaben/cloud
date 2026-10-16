// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesPaymentFact is the golang structure of table plugin_idc_core_sales_payment_fact for DAO operations like Where/Data.
type SalesPaymentFact struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_payment_fact, do:true"`
	Id                  any        //
	TenantId            any        //
	InvoiceNo           any        //
	OrderNo             *string    //
	CustomerNo          any        //
	CurrencyCode        any        //
	OperationTypeCode   any        //
	ReceivedAmountMinor any        //
	ReceivedAt          *time.Time //
	SettlementNo        any        //
	SourceDigest        any        //
	CreatedAt           *time.Time //
}
