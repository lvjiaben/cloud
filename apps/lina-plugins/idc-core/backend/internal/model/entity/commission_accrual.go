// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommissionAccrual is the golang structure for table commission_accrual.
type CommissionAccrual struct {
	Id                     int64      `json:"id"                     orm:"id"                        description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                 description:""`
	CommissionNo           string     `json:"commissionNo"           orm:"commission_no"             description:""`
	OrderNo                string     `json:"orderNo"                orm:"order_no"                  description:""`
	InvoiceNo              string     `json:"invoiceNo"              orm:"invoice_no"                description:""`
	InvoiceItemNo          string     `json:"invoiceItemNo"          orm:"invoice_item_no"           description:""`
	BeneficiaryTypeCode    string     `json:"beneficiaryTypeCode"    orm:"beneficiary_type_code"     description:""`
	BeneficiaryReference   string     `json:"beneficiaryReference"   orm:"beneficiary_reference"     description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"             description:""`
	RuleVersion            int64      `json:"ruleVersion"            orm:"rule_version"              description:""`
	BasisAmountMinor       int64      `json:"basisAmountMinor"       orm:"basis_amount_minor"        description:""`
	InvoiceItemAmountMinor int64      `json:"invoiceItemAmountMinor" orm:"invoice_item_amount_minor" description:""`
	OriginalAmountMinor    int64      `json:"originalAmountMinor"    orm:"original_amount_minor"     description:""`
	PendingAmountMinor     int64      `json:"pendingAmountMinor"     orm:"pending_amount_minor"      description:""`
	MaturedAmountMinor     int64      `json:"maturedAmountMinor"     orm:"matured_amount_minor"      description:""`
	ReversedAmountMinor    int64      `json:"reversedAmountMinor"    orm:"reversed_amount_minor"     description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"               description:""`
	MaturesAt              *time.Time `json:"maturesAt"              orm:"matures_at"                description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"                description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"                description:""`
	AgentQualificationNo   string     `json:"agentQualificationNo"   orm:"agent_qualification_no"    description:""`
	AffiliateAttributionNo string     `json:"affiliateAttributionNo" orm:"affiliate_attribution_no"  description:""`
	SalesAssignmentNo      string     `json:"salesAssignmentNo"      orm:"sales_assignment_no"       description:""`
	CommissionRuleNo       string     `json:"commissionRuleNo"       orm:"commission_rule_no"        description:""`
	CommissionTierNo       string     `json:"commissionTierNo"       orm:"commission_tier_no"        description:""`
	OperationTypeCode      string     `json:"operationTypeCode"      orm:"operation_type_code"       description:""`
}
