// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialDeliveryIntent is the golang structure for table financial_delivery_intent.
type FinancialDeliveryIntent struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	IntentNo            string     `json:"intentNo"            orm:"intent_no"             description:""`
	InvoiceNo           string     `json:"invoiceNo"           orm:"invoice_no"            description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	ContactNo           string     `json:"contactNo"           orm:"contact_no"            description:""`
	InvoiceVersion      int64      `json:"invoiceVersion"      orm:"invoice_version"       description:""`
	NotificationEventNo string     `json:"notificationEventNo" orm:"notification_event_no" description:""`
	RequestedBy         int        `json:"requestedBy"         orm:"requested_by"          description:""`
	IdempotencyDigest   string     `json:"idempotencyDigest"   orm:"idempotency_digest"    description:""`
	RequestDigest       string     `json:"requestDigest"       orm:"request_digest"        description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
