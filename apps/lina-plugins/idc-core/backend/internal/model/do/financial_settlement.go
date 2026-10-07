// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSettlement is the golang structure of table plugin_idc_core_financial_settlement for DAO operations like Where/Data.
type FinancialSettlement struct {
	g.Meta                 `orm:"table:plugin_idc_core_financial_settlement, do:true"`
	Id                     any        //
	TenantId               any        //
	SettlementNo           any        //
	OrderNo                any        //
	InvoiceNo              any        //
	CustomerNo             any        //
	CurrencyCode           any        //
	BasisCode              any        // Mutually exclusive funded, credit, zero-amount, full-discount, or trial basis
	TotalAmountMinor       any        //
	PaidAmountMinor        any        //
	CreditAmountMinor      any        //
	SourceCount            any        //
	IdentityEvidenceDigest any        // Digest of the re-read customer identity authority without personal material
	SourceSnapshotDigest   any        //
	ContentDigest          any        //
	SettledAt              *time.Time //
	CreatedAt              *time.Time //
}
