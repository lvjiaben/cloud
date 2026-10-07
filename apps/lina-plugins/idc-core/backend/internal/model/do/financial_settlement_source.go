// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSettlementSource is the golang structure of table plugin_idc_core_financial_settlement_source for DAO operations like Where/Data.
type FinancialSettlementSource struct {
	g.Meta              `orm:"table:plugin_idc_core_financial_settlement_source, do:true"`
	Id                  any        //
	TenantId            any        //
	SettlementSourceNo  any        //
	SettlementNo        any        //
	OrderNo             any        //
	InvoiceNo           any        //
	CustomerNo          any        //
	CurrencyCode        any        //
	SourceTypeCode      any        //
	SourceReference     any        //
	PaymentAllocationNo *string    //
	CreditExposureNo    *string    //
	AmountMinor         any        //
	EvidenceDigest      any        //
	ContentDigest       any        //
	CreatedAt           *time.Time //
}
