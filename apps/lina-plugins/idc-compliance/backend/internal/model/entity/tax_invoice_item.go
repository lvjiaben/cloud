// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaxInvoiceItem is the golang structure for table tax_invoice_item.
type TaxInvoiceItem struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	ItemNo                string     `json:"itemNo"                orm:"item_no"                 description:""`
	TaxInvoiceNo          string     `json:"taxInvoiceNo"          orm:"tax_invoice_no"          description:""`
	LineNo                int        `json:"lineNo"                orm:"line_no"                 description:""`
	ItemSnapshotRef       string     `json:"itemSnapshotRef"       orm:"item_snapshot_ref"       description:""`
	ItemSnapshotSha256    string     `json:"itemSnapshotSha256"    orm:"item_snapshot_sha256"    description:""`
	TaxClassificationCode string     `json:"taxClassificationCode" orm:"tax_classification_code" description:""`
	TaxRatePpm            int        `json:"taxRatePpm"            orm:"tax_rate_ppm"            description:""`
	GrossAmountMinor      int64      `json:"grossAmountMinor"      orm:"gross_amount_minor"      description:""`
	NetAmountMinor        int64      `json:"netAmountMinor"        orm:"net_amount_minor"        description:""`
	TaxAmountMinor        int64      `json:"taxAmountMinor"        orm:"tax_amount_minor"        description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	AllocationNo          string     `json:"allocationNo"          orm:"allocation_no"           description:"Immutable local allocation owning this source tax item"`
	BillingInvoiceItemNo  string     `json:"billingInvoiceItemNo"  orm:"billing_invoice_item_no" description:"Opaque core billing item reference used only for bounded capacity accounting"`
}
