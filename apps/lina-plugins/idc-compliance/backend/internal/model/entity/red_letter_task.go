// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RedLetterTask is the golang structure for table red_letter_task.
type RedLetterTask struct {
	Id                           int64      `json:"id"                           orm:"id"                             description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                      description:""`
	TaskNo                       string     `json:"taskNo"                       orm:"task_no"                        description:""`
	TaxInvoiceNo                 *string    `json:"taxInvoiceNo"                 orm:"tax_invoice_no"                 description:""`
	RequestSha256                string     `json:"requestSha256"                orm:"request_sha256"                 description:""`
	RefundOperationNo            string     `json:"refundOperationNo"            orm:"refund_operation_no"            description:""`
	ReasonCode                   string     `json:"reasonCode"                   orm:"reason_code"                    description:""`
	StatusCode                   string     `json:"statusCode"                   orm:"status_code"                    description:""`
	GrossAmountMinor             int64      `json:"grossAmountMinor"             orm:"gross_amount_minor"             description:""`
	NetAmountMinor               int64      `json:"netAmountMinor"               orm:"net_amount_minor"               description:""`
	TaxAmountMinor               int64      `json:"taxAmountMinor"               orm:"tax_amount_minor"               description:""`
	CustomerConfirmationRequired bool       `json:"customerConfirmationRequired" orm:"customer_confirmation_required" description:""`
	CustomerConfirmedAt          *time.Time `json:"customerConfirmedAt"          orm:"customer_confirmed_at"          description:""`
	ProviderCode                 *string    `json:"providerCode"                 orm:"provider_code"                  description:""`
	ExternalTaskRef              *string    `json:"externalTaskRef"              orm:"external_task_ref"              description:""`
	AvailableAt                  *time.Time `json:"availableAt"                  orm:"available_at"                   description:""`
	NextQueryAt                  *time.Time `json:"nextQueryAt"                  orm:"next_query_at"                  description:""`
	CompletedAt                  *time.Time `json:"completedAt"                  orm:"completed_at"                   description:""`
	FailureCode                  string     `json:"failureCode"                  orm:"failure_code"                   description:""`
	Version                      int64      `json:"version"                      orm:"version"                        description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                     description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                     description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                    description:""`
	CurrencyCode                 string     `json:"currencyCode"                 orm:"currency_code"                  description:""`
	AllocationResolved           bool       `json:"allocationResolved"           orm:"allocation_resolved"            description:""`
	ManualReasonCode             string     `json:"manualReasonCode"             orm:"manual_reason_code"             description:""`
	CompletionEvidenceNo         *string    `json:"completionEvidenceNo"         orm:"completion_evidence_no"         description:""`
	CustomerRecheckRequestedAt   *time.Time `json:"customerRecheckRequestedAt"   orm:"customer_recheck_requested_at"  description:""`
}
