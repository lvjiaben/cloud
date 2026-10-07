// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialSettlementDelivery is the golang structure for table financial_settlement_delivery.
type FinancialSettlementDelivery struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	DeliveryNo        string     `json:"deliveryNo"        orm:"delivery_no"         description:""`
	EventNo           string     `json:"eventNo"           orm:"event_no"            description:""`
	EventType         string     `json:"eventType"         orm:"event_type"          description:""`
	SettlementNo      string     `json:"settlementNo"      orm:"settlement_no"       description:""`
	OrderNo           string     `json:"orderNo"           orm:"order_no"            description:""`
	InvoiceNo         string     `json:"invoiceNo"         orm:"invoice_no"          description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"         description:""`
	CurrencyCode      string     `json:"currencyCode"      orm:"currency_code"       description:""`
	FulfillmentCount  int        `json:"fulfillmentCount"  orm:"fulfillment_count"   description:""`
	ReadyCount        int        `json:"readyCount"        orm:"ready_count"         description:""`
	WaitingGateCount  int        `json:"waitingGateCount"  orm:"waiting_gate_count"  description:""`
	ManualReviewCount int        `json:"manualReviewCount" orm:"manual_review_count" description:""`
	EvidenceDigest    string     `json:"evidenceDigest"    orm:"evidence_digest"     description:""`
	ContentDigest     string     `json:"contentDigest"     orm:"content_digest"      description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
