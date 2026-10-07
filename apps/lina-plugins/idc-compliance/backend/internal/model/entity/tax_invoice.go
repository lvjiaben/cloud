// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaxInvoice is the golang structure for table tax_invoice.
type TaxInvoice struct {
	Id                           int64      `json:"id"                           orm:"id"                               description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                        description:""`
	TaxInvoiceNo                 string     `json:"taxInvoiceNo"                 orm:"tax_invoice_no"                   description:""`
	RequestSha256                string     `json:"requestSha256"                orm:"request_sha256"                   description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                      description:""`
	PrincipalNo                  string     `json:"principalNo"                  orm:"principal_no"                     description:""`
	SellerSnapshotRef            string     `json:"sellerSnapshotRef"            orm:"seller_snapshot_ref"              description:"Private immutable seller snapshot for the installation merchant, never the software platform"`
	SellerSnapshotSha256         string     `json:"sellerSnapshotSha256"         orm:"seller_snapshot_sha256"           description:""`
	BuyerSnapshotRef             string     `json:"buyerSnapshotRef"             orm:"buyer_snapshot_ref"               description:""`
	BuyerSnapshotSha256          string     `json:"buyerSnapshotSha256"          orm:"buyer_snapshot_sha256"            description:""`
	InvoiceTypeCode              string     `json:"invoiceTypeCode"              orm:"invoice_type_code"                description:"Customer requests use pending_confirmation; an authorized reviewer selects a supported type later"`
	StatusCode                   string     `json:"statusCode"                   orm:"status_code"                      description:""`
	TaxCurrencyCode              string     `json:"taxCurrencyCode"              orm:"tax_currency_code"                description:""`
	GrossAmountMinor             int64      `json:"grossAmountMinor"             orm:"gross_amount_minor"               description:""`
	NetAmountMinor               int64      `json:"netAmountMinor"               orm:"net_amount_minor"                 description:""`
	TaxAmountMinor               int64      `json:"taxAmountMinor"               orm:"tax_amount_minor"                 description:""`
	BillingCurrencyCode          string     `json:"billingCurrencyCode"          orm:"billing_currency_code"            description:""`
	ExchangeRateNumerator        *int64     `json:"exchangeRateNumerator"        orm:"exchange_rate_numerator"          description:""`
	ExchangeRateDenominator      *int64     `json:"exchangeRateDenominator"      orm:"exchange_rate_denominator"        description:""`
	ExchangeRateDate             *time.Time `json:"exchangeRateDate"             orm:"exchange_rate_date"               description:""`
	ExchangeRateSourceCode       *string    `json:"exchangeRateSourceCode"       orm:"exchange_rate_source_code"        description:""`
	ProviderCode                 *string    `json:"providerCode"                 orm:"provider_code"                    description:""`
	ExternalInvoiceRef           *string    `json:"externalInvoiceRef"           orm:"external_invoice_ref"             description:""`
	MaskedInvoiceNumber          string     `json:"maskedInvoiceNumber"          orm:"masked_invoice_number"            description:""`
	InvoiceNumberLookupDigest    *string    `json:"invoiceNumberLookupDigest"    orm:"invoice_number_lookup_digest"     description:"Keyed lookup digest for exact invoice-number reconciliation; raw values and unsalted public hashes are forbidden"`
	SubmittedAt                  *time.Time `json:"submittedAt"                  orm:"submitted_at"                     description:""`
	IssuedAt                     *time.Time `json:"issuedAt"                     orm:"issued_at"                        description:""`
	DeliveredAt                  *time.Time `json:"deliveredAt"                  orm:"delivered_at"                     description:""`
	CanceledAt                   *time.Time `json:"canceledAt"                   orm:"canceled_at"                      description:""`
	Version                      int64      `json:"version"                      orm:"version"                          description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                       description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                       description:""`
	DecisionReasonCode           string     `json:"decisionReasonCode"           orm:"decision_reason_code"             description:""`
	DecisionInternalNote         string     `json:"decisionInternalNote"         orm:"decision_internal_note"           description:"Optional administrator-only note; customer projections must never expose it"`
	ReviewedByUserId             *int       `json:"reviewedByUserId"             orm:"reviewed_by_user_id"              description:""`
	ReviewedAt                   *time.Time `json:"reviewedAt"                   orm:"reviewed_at"                      description:""`
	ProviderNotIssuedConfirmedAt *time.Time `json:"providerNotIssuedConfirmedAt" orm:"provider_not_issued_confirmed_at" description:"Authoritative positive evidence required before safely canceling a failed provider attempt"`
}
