// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingInvoiceTransform is the golang structure for table billing_invoice_transform.
type BillingInvoiceTransform struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	TransformNo       string     `json:"transformNo"       orm:"transform_no"        description:""`
	TransformTypeCode string     `json:"transformTypeCode" orm:"transform_type_code" description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"         description:""`
	CurrencyCode      string     `json:"currencyCode"      orm:"currency_code"       description:""`
	SourceCount       int        `json:"sourceCount"       orm:"source_count"        description:""`
	DestinationCount  int        `json:"destinationCount"  orm:"destination_count"   description:""`
	ItemCount         int        `json:"itemCount"         orm:"item_count"          description:""`
	IdempotencyDigest string     `json:"idempotencyDigest" orm:"idempotency_digest"  description:""`
	RequestDigest     string     `json:"requestDigest"     orm:"request_digest"      description:""`
	ContentDigest     string     `json:"contentDigest"     orm:"content_digest"      description:""`
	RequestedBy       int        `json:"requestedBy"       orm:"requested_by"        description:""`
	CompletedAt       *time.Time `json:"completedAt"       orm:"completed_at"        description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
