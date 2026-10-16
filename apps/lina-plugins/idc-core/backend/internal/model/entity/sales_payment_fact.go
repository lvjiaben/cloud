// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesPaymentFact is the golang structure for table sales_payment_fact.
type SalesPaymentFact struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	InvoiceNo           string     `json:"invoiceNo"           orm:"invoice_no"            description:""`
	OrderNo             *string    `json:"orderNo"             orm:"order_no"              description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	OperationTypeCode   string     `json:"operationTypeCode"   orm:"operation_type_code"   description:""`
	ReceivedAmountMinor int64      `json:"receivedAmountMinor" orm:"received_amount_minor" description:""`
	ReceivedAt          *time.Time `json:"receivedAt"          orm:"received_at"           description:""`
	SettlementNo        string     `json:"settlementNo"        orm:"settlement_no"         description:""`
	SourceDigest        string     `json:"sourceDigest"        orm:"source_digest"         description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
