// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingInvoiceItemDerivation is the golang structure for table billing_invoice_item_derivation.
type BillingInvoiceItemDerivation struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	TransformNo              string     `json:"transformNo"              orm:"transform_no"                description:""`
	SourceInvoiceNo          string     `json:"sourceInvoiceNo"          orm:"source_invoice_no"           description:""`
	SourceInvoiceItemNo      string     `json:"sourceInvoiceItemNo"      orm:"source_invoice_item_no"      description:""`
	DestinationInvoiceNo     string     `json:"destinationInvoiceNo"     orm:"destination_invoice_no"      description:""`
	DestinationInvoiceItemNo string     `json:"destinationInvoiceItemNo" orm:"destination_invoice_item_no" description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                 description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"               description:""`
	SourceContentDigest      string     `json:"sourceContentDigest"      orm:"source_content_digest"       description:""`
	DestinationContentDigest string     `json:"destinationContentDigest" orm:"destination_content_digest"  description:""`
	ContentDigest            string     `json:"contentDigest"            orm:"content_digest"              description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
}
