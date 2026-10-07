// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionRule is the golang structure for table sales_commission_rule.
type SalesCommissionRule struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	RuleNo              string     `json:"ruleNo"              orm:"rule_no"               description:""`
	BeneficiaryTypeCode string     `json:"beneficiaryTypeCode" orm:"beneficiary_type_code" description:""`
	OperationTypeCode   string     `json:"operationTypeCode"   orm:"operation_type_code"   description:""`
	ProductNo           *string    `json:"productNo"           orm:"product_no"            description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	CalculationTypeCode string     `json:"calculationTypeCode" orm:"calculation_type_code" description:""`
	FixedAmountMinor    *int64     `json:"fixedAmountMinor"    orm:"fixed_amount_minor"    description:""`
	RateBasisPoints     *int       `json:"rateBasisPoints"     orm:"rate_basis_points"     description:""`
	CapAmountMinor      *int64     `json:"capAmountMinor"      orm:"cap_amount_minor"      description:""`
	MaturityDays        int        `json:"maturityDays"        orm:"maturity_days"         description:""`
	Priority            int        `json:"priority"            orm:"priority"              description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	EffectiveAt         *time.Time `json:"effectiveAt"         orm:"effective_at"          description:""`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"            description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	RuleDigest          string     `json:"ruleDigest"          orm:"rule_digest"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
