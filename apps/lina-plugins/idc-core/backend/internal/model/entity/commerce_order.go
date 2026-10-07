// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceOrder is the golang structure for table commerce_order.
type CommerceOrder struct {
	Id                           int64      `json:"id"                           orm:"id"                              description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                       description:""`
	OrderNo                      string     `json:"orderNo"                      orm:"order_no"                        description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                     description:""`
	QuoteNo                      string     `json:"quoteNo"                      orm:"quote_no"                        description:""`
	CurrencyCode                 string     `json:"currencyCode"                 orm:"currency_code"                   description:""`
	SourceCode                   string     `json:"sourceCode"                   orm:"source_code"                     description:""`
	StatusCode                   string     `json:"statusCode"                   orm:"status_code"                     description:""`
	RiskDecisionCode             string     `json:"riskDecisionCode"             orm:"risk_decision_code"              description:""`
	RequiresContract             bool       `json:"requiresContract"             orm:"requires_contract"               description:""`
	ItemCount                    int        `json:"itemCount"                    orm:"item_count"                      description:""`
	SubtotalMinor                int64      `json:"subtotalMinor"                orm:"subtotal_minor"                  description:""`
	DiscountMinor                int64      `json:"discountMinor"                orm:"discount_minor"                  description:""`
	NetAmountMinor               int64      `json:"netAmountMinor"               orm:"net_amount_minor"                description:""`
	TaxAmountMinor               int64      `json:"taxAmountMinor"               orm:"tax_amount_minor"                description:""`
	TotalAmountMinor             int64      `json:"totalAmountMinor"             orm:"total_amount_minor"              description:""`
	CheckoutIdempotencyDigest    string     `json:"checkoutIdempotencyDigest"    orm:"checkout_idempotency_digest"     description:""`
	CheckoutRequestDigest        string     `json:"checkoutRequestDigest"        orm:"checkout_request_digest"         description:""`
	PaymentDueAt                 *time.Time `json:"paymentDueAt"                 orm:"payment_due_at"                  description:""`
	PaidAt                       *time.Time `json:"paidAt"                       orm:"paid_at"                         description:""`
	CompletedAt                  *time.Time `json:"completedAt"                  orm:"completed_at"                    description:""`
	CancelledAt                  *time.Time `json:"cancelledAt"                  orm:"cancelled_at"                    description:""`
	Version                      int64      `json:"version"                      orm:"version"                         description:""`
	CreatedBy                    int        `json:"createdBy"                    orm:"created_by"                      description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                      description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                      description:""`
	AssistedRequestNo            *string    `json:"assistedRequestNo"            orm:"assisted_request_no"             description:""`
	PaymentTermCode              string     `json:"paymentTermCode"              orm:"payment_term_code"               description:""`
	ContractOverridePolicyCode   string     `json:"contractOverridePolicyCode"   orm:"contract_override_policy_code"   description:"Order choice preserved separately: inherit product, explicit none, or required"`
	ContractOverrideTemplateNo   string     `json:"contractOverrideTemplateNo"   orm:"contract_override_template_no"   description:""`
	EnterpriseContractPolicyCode string     `json:"enterpriseContractPolicyCode" orm:"enterprise_contract_policy_code" description:""`
	EnterpriseContractTemplateNo string     `json:"enterpriseContractTemplateNo" orm:"enterprise_contract_template_no" description:""`
	ContractDecisionSourceCode   string     `json:"contractDecisionSourceCode"   orm:"contract_decision_source_code"   description:"Winning rule source after order > product > enterprise > default resolution"`
	ContractTemplateNo           string     `json:"contractTemplateNo"           orm:"contract_template_no"            description:""`
	ContractTemplateVersionNo    int64      `json:"contractTemplateVersionNo"    orm:"contract_template_version_no"    description:""`
	ContractTemplateName         string     `json:"contractTemplateName"         orm:"contract_template_name"          description:""`
	ContractLanguageCode         string     `json:"contractLanguageCode"         orm:"contract_language_code"          description:""`
	ContractScopeCode            string     `json:"contractScopeCode"            orm:"contract_scope_code"             description:""`
	ContractSigningOrderCode     string     `json:"contractSigningOrderCode"     orm:"contract_signing_order_code"     description:""`
}
