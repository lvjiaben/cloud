// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSettlementDelivery is the golang structure of table plugin_idc_core_financial_settlement_delivery for DAO operations like Where/Data.
type FinancialSettlementDelivery struct {
	g.Meta            `orm:"table:plugin_idc_core_financial_settlement_delivery, do:true"`
	Id                any        //
	TenantId          any        //
	DeliveryNo        any        //
	EventNo           any        //
	EventType         any        //
	SettlementNo      any        //
	OrderNo           any        //
	InvoiceNo         any        //
	CustomerNo        any        //
	CurrencyCode      any        //
	FulfillmentCount  any        //
	ReadyCount        any        //
	WaitingGateCount  any        //
	ManualReviewCount any        //
	EvidenceDigest    any        //
	ContentDigest     any        //
	CreatedAt         *time.Time //
}
