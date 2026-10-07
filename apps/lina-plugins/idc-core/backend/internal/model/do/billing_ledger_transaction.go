// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingLedgerTransaction is the golang structure of table plugin_idc_core_billing_ledger_transaction for DAO operations like Where/Data.
type BillingLedgerTransaction struct {
	g.Meta                `orm:"table:plugin_idc_core_billing_ledger_transaction, do:true"`
	Id                    any        //
	TenantId              any        //
	TransactionNo         any        //
	TransactionTypeCode   any        //
	CurrencyCode          any        //
	SourceTypeCode        any        //
	SourceReference       any        //
	SourceSnapshotDigest  any        //
	OriginalTransactionNo *string    //
	EntryCount            any        //
	DebitTotalMinor       any        //
	CreditTotalMinor      any        //
	ContentDigest         any        //
	PostedBy              any        //
	PostedAt              *time.Time //
	CreatedAt             *time.Time //
}
