// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BusinessOperation is the golang structure for table business_operation.
type BusinessOperation struct {
	Id                          int64      `json:"id"                          orm:"id"                            description:""`
	TenantId                    int        `json:"tenantId"                    orm:"tenant_id"                     description:""`
	OperationNo                 string     `json:"operationNo"                 orm:"operation_no"                  description:""`
	OperationTypeCode           string     `json:"operationTypeCode"           orm:"operation_type_code"           description:""`
	CustomerNo                  string     `json:"customerNo"                  orm:"customer_no"                   description:""`
	ServiceNo                   string     `json:"serviceNo"                   orm:"service_no"                    description:""`
	CurrencyCode                string     `json:"currencyCode"                orm:"currency_code"                 description:""`
	PriceSnapshotNo             *string    `json:"priceSnapshotNo"             orm:"price_snapshot_no"             description:""`
	QuoteNo                     *string    `json:"quoteNo"                     orm:"quote_no"                      description:""`
	OrderNo                     *string    `json:"orderNo"                     orm:"order_no"                      description:""`
	InvoiceNo                   *string    `json:"invoiceNo"                   orm:"invoice_no"                    description:""`
	StatusCode                  string     `json:"statusCode"                  orm:"status_code"                   description:""`
	FinancialSettlementRequired bool       `json:"financialSettlementRequired" orm:"financial_settlement_required" description:""`
	ExecutionModeCode           string     `json:"executionModeCode"           orm:"execution_mode_code"           description:""`
	ResourceOwnerCode           string     `json:"resourceOwnerCode"           orm:"resource_owner_code"           description:""`
	ChannelNo                   *string    `json:"channelNo"                   orm:"channel_no"                    description:""`
	PoolNo                      string     `json:"poolNo"                      orm:"pool_no"                       description:""`
	ProviderTaskNo              *string    `json:"providerTaskNo"              orm:"provider_task_no"              description:""`
	CapabilityEvidenceNo        *string    `json:"capabilityEvidenceNo"        orm:"capability_evidence_no"        description:""`
	CapabilityEvidenceDigest    *string    `json:"capabilityEvidenceDigest"    orm:"capability_evidence_digest"    description:""`
	IdempotencyDigest           string     `json:"idempotencyDigest"           orm:"idempotency_digest"            description:""`
	RequestDigest               string     `json:"requestDigest"               orm:"request_digest"                description:""`
	ContentDigest               string     `json:"contentDigest"               orm:"content_digest"                description:""`
	ExpectedServiceVersion      int64      `json:"expectedServiceVersion"      orm:"expected_service_version"      description:""`
	LastErrorCode               string     `json:"lastErrorCode"               orm:"last_error_code"               description:""`
	FinanciallySettledAt        *time.Time `json:"financiallySettledAt"        orm:"financially_settled_at"        description:""`
	ExecutionStartedAt          *time.Time `json:"executionStartedAt"          orm:"execution_started_at"          description:""`
	CompletedAt                 *time.Time `json:"completedAt"                 orm:"completed_at"                  description:""`
	Version                     int64      `json:"version"                     orm:"version"                       description:""`
	CreatedAt                   *time.Time `json:"createdAt"                   orm:"created_at"                    description:""`
	UpdatedAt                   *time.Time `json:"updatedAt"                   orm:"updated_at"                    description:""`
	SourceFulfillmentNo         *string    `json:"sourceFulfillmentNo"         orm:"source_fulfillment_no"         description:"Immutable new-purchase server fulfillment identity"`
	SettlementNo                *string    `json:"settlementNo"                orm:"settlement_no"                 description:"Unified financial settlement authority for a paid new-purchase operation"`
	SubjectTypeCode             string     `json:"subjectTypeCode"             orm:"subject_type_code"             description:""`
	SubjectNo                   string     `json:"subjectNo"                   orm:"subject_no"                    description:""`
	ExpectedSubjectVersion      int64      `json:"expectedSubjectVersion"      orm:"expected_subject_version"      description:""`
	PayerCustomerNo             string     `json:"payerCustomerNo"             orm:"payer_customer_no"             description:""`
}
