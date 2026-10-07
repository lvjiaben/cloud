// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceItem is the golang structure of table plugin_idc_compliance_tax_invoice_item for DAO operations like Where/Data.
type TaxInvoiceItem struct {
	g.Meta                `orm:"table:plugin_idc_compliance_tax_invoice_item, do:true"`
	Id                    any        //
	TenantId              any        //
	ItemNo                any        //
	TaxInvoiceNo          any        //
	LineNo                any        //
	ItemSnapshotRef       any        //
	ItemSnapshotSha256    any        //
	TaxClassificationCode any        //
	TaxRatePpm            any        //
	GrossAmountMinor      any        //
	NetAmountMinor        any        //
	TaxAmountMinor        any        //
	CreatedAt             *time.Time //
	AllocationNo          any        // Immutable local allocation owning this source tax item
	BillingInvoiceItemNo  any        // Opaque core billing item reference used only for bounded capacity accounting
}
