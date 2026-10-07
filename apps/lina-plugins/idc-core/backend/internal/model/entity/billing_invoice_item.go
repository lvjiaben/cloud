// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingInvoiceItem is the golang structure for table billing_invoice_item.
type BillingInvoiceItem struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	InvoiceItemNo      string     `json:"invoiceItemNo"      orm:"invoice_item_no"      description:""`
	InvoiceNo          string     `json:"invoiceNo"          orm:"invoice_no"           description:""`
	CustomerNo         string     `json:"customerNo"         orm:"customer_no"          description:""`
	CurrencyCode       string     `json:"currencyCode"       orm:"currency_code"        description:""`
	LineNumber         int        `json:"lineNumber"         orm:"line_number"          description:""`
	ItemTypeCode       string     `json:"itemTypeCode"       orm:"item_type_code"       description:""`
	OrderNo            *string    `json:"orderNo"            orm:"order_no"             description:""`
	OrderItemNo        *string    `json:"orderItemNo"        orm:"order_item_no"        description:""`
	SourceReference    string     `json:"sourceReference"    orm:"source_reference"     description:""`
	DescriptionZhCn    string     `json:"descriptionZhCn"    orm:"description_zh_cn"    description:""`
	DescriptionEnUs    string     `json:"descriptionEnUs"    orm:"description_en_us"    description:""`
	SubtotalMinor      int64      `json:"subtotalMinor"      orm:"subtotal_minor"       description:""`
	DiscountMinor      int64      `json:"discountMinor"      orm:"discount_minor"       description:""`
	NetAmountMinor     int64      `json:"netAmountMinor"     orm:"net_amount_minor"     description:""`
	TaxAmountMinor     int64      `json:"taxAmountMinor"     orm:"tax_amount_minor"     description:""`
	TotalAmountMinor   int64      `json:"totalAmountMinor"   orm:"total_amount_minor"   description:""`
	RefundableCapMinor int64      `json:"refundableCapMinor" orm:"refundable_cap_minor" description:""`
	ContentDigest      string     `json:"contentDigest"      orm:"content_digest"       description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
}
