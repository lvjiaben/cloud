// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialSettlementSource is the golang structure for table financial_settlement_source.
type FinancialSettlementSource struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	SettlementSourceNo  string     `json:"settlementSourceNo"  orm:"settlement_source_no"  description:""`
	SettlementNo        string     `json:"settlementNo"        orm:"settlement_no"         description:""`
	OrderNo             string     `json:"orderNo"             orm:"order_no"              description:""`
	InvoiceNo           string     `json:"invoiceNo"           orm:"invoice_no"            description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	SourceTypeCode      string     `json:"sourceTypeCode"      orm:"source_type_code"      description:""`
	SourceReference     string     `json:"sourceReference"     orm:"source_reference"      description:""`
	PaymentAllocationNo *string    `json:"paymentAllocationNo" orm:"payment_allocation_no" description:""`
	CreditExposureNo    *string    `json:"creditExposureNo"    orm:"credit_exposure_no"    description:""`
	AmountMinor         int64      `json:"amountMinor"         orm:"amount_minor"          description:""`
	EvidenceDigest      string     `json:"evidenceDigest"      orm:"evidence_digest"       description:""`
	ContentDigest       string     `json:"contentDigest"       orm:"content_digest"        description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
