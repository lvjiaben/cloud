// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingLedgerEntry is the golang structure of table plugin_idc_core_billing_ledger_entry for DAO operations like Where/Data.
type BillingLedgerEntry struct {
	g.Meta        `orm:"table:plugin_idc_core_billing_ledger_entry, do:true"`
	Id            any        //
	TenantId      any        //
	EntryNo       any        //
	TransactionNo any        //
	CurrencyCode  any        //
	LineNumber    any        //
	AccountNo     any        //
	DirectionCode any        //
	EntryRoleCode any        //
	AmountMinor   any        //
	InvoiceNo     *string    //
	InvoiceItemNo *string    //
	ReasonCode    any        //
	ContentDigest any        //
	CreatedAt     *time.Time //
}
