// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogTaxPolicy is the golang structure for table catalog_tax_policy.
type CatalogTaxPolicy struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	TaxPolicyNo          string     `json:"taxPolicyNo"          orm:"tax_policy_no"          description:""`
	VersionNo            string     `json:"versionNo"            orm:"version_no"             description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"          description:""`
	PriceIncludesTax     bool       `json:"priceIncludesTax"     orm:"price_includes_tax"     description:""`
	TaxTreatmentCode     string     `json:"taxTreatmentCode"     orm:"tax_treatment_code"     description:""`
	TaxRateBasisPoints   *int       `json:"taxRateBasisPoints"   orm:"tax_rate_basis_points"  description:"Tax rate in basis points; null only for exempt treatment"`
	TaxCategoryCode      string     `json:"taxCategoryCode"      orm:"tax_category_code"      description:""`
	RoundingScopeCode    string     `json:"roundingScopeCode"    orm:"rounding_scope_code"    description:""`
	RoundingModeCode     string     `json:"roundingModeCode"     orm:"rounding_mode_code"     description:""`
	RoundingQuantumMinor int64      `json:"roundingQuantumMinor" orm:"rounding_quantum_minor" description:"Positive rounding quantum expressed in the configured currency minor unit"`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
}
