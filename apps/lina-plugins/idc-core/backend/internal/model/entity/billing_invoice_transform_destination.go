// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingInvoiceTransformDestination is the golang structure for table billing_invoice_transform_destination.
type BillingInvoiceTransformDestination struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	TransformNo              string     `json:"transformNo"              orm:"transform_no"               description:""`
	DestinationInvoiceNo     string     `json:"destinationInvoiceNo"     orm:"destination_invoice_no"     description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"              description:""`
	Position                 int        `json:"position"                 orm:"position"                   description:""`
	DestinationContentDigest string     `json:"destinationContentDigest" orm:"destination_content_digest" description:""`
	ContentDigest            string     `json:"contentDigest"            orm:"content_digest"             description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
}
