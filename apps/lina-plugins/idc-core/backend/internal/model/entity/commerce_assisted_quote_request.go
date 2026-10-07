// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceAssistedQuoteRequest is the golang structure for table commerce_assisted_quote_request.
type CommerceAssistedQuoteRequest struct {
	Id                         int64      `json:"id"                         orm:"id"                            description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                     description:""`
	RequestNo                  string     `json:"requestNo"                  orm:"request_no"                    description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                   description:""`
	CurrencyCode               string     `json:"currencyCode"               orm:"currency_code"                 description:""`
	StatusCode                 string     `json:"statusCode"                 orm:"status_code"                   description:""`
	ItemCount                  int        `json:"itemCount"                  orm:"item_count"                    description:""`
	ManualDiscountMinor        int64      `json:"manualDiscountMinor"        orm:"manual_discount_minor"         description:""`
	PaymentTermCode            string     `json:"paymentTermCode"            orm:"payment_term_code"             description:""`
	PaymentDueSeconds          int        `json:"paymentDueSeconds"          orm:"payment_due_seconds"           description:""`
	Reason                     string     `json:"reason"                     orm:"reason"                        description:""`
	InternalNote               string     `json:"internalNote"               orm:"internal_note"                 description:""`
	IdempotencyDigest          string     `json:"idempotencyDigest"          orm:"idempotency_digest"            description:""`
	RequestDigest              string     `json:"requestDigest"              orm:"request_digest"                description:""`
	ExpiresAt                  *time.Time `json:"expiresAt"                  orm:"expires_at"                    description:""`
	RequestedBy                int        `json:"requestedBy"                orm:"requested_by"                  description:""`
	DecisionCode               *string    `json:"decisionCode"               orm:"decision_code"                 description:""`
	DecisionBy                 *int       `json:"decisionBy"                 orm:"decision_by"                   description:""`
	DecisionAt                 *time.Time `json:"decisionAt"                 orm:"decision_at"                   description:""`
	DecisionReason             *string    `json:"decisionReason"             orm:"decision_reason"               description:""`
	DecisionRequestDigest      *string    `json:"decisionRequestDigest"      orm:"decision_request_digest"       description:""`
	DecisionEvidenceDigest     *string    `json:"decisionEvidenceDigest"     orm:"decision_evidence_digest"      description:""`
	QuoteNo                    *string    `json:"quoteNo"                    orm:"quote_no"                      description:""`
	OrderNo                    *string    `json:"orderNo"                    orm:"order_no"                      description:""`
	Version                    int64      `json:"version"                    orm:"version"                       description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                    description:""`
	UpdatedAt                  *time.Time `json:"updatedAt"                  orm:"updated_at"                    description:""`
	ContractOverridePolicyCode string     `json:"contractOverridePolicyCode" orm:"contract_override_policy_code" description:"Administrator order choice captured before assisted checkout"`
	ContractOverrideTemplateNo string     `json:"contractOverrideTemplateNo" orm:"contract_override_template_no" description:""`
}
