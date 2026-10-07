// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServicePriceSnapshot is the golang structure for table service_price_snapshot.
type ServicePriceSnapshot struct {
	Id                                int64      `json:"id"                                orm:"id"                                   description:""`
	TenantId                          int        `json:"tenantId"                          orm:"tenant_id"                            description:""`
	PriceSnapshotNo                   string     `json:"priceSnapshotNo"                   orm:"price_snapshot_no"                    description:""`
	ServiceNo                         string     `json:"serviceNo"                         orm:"service_no"                           description:""`
	CustomerNo                        string     `json:"customerNo"                        orm:"customer_no"                          description:""`
	PhaseCode                         string     `json:"phaseCode"                         orm:"phase_code"                           description:""`
	SourceQuoteNo                     *string    `json:"sourceQuoteNo"                     orm:"source_quote_no"                      description:""`
	SourceQuoteItemNo                 *string    `json:"sourceQuoteItemNo"                 orm:"source_quote_item_no"                 description:""`
	ProductNo                         string     `json:"productNo"                         orm:"product_no"                           description:""`
	VersionNo                         string     `json:"versionNo"                         orm:"version_no"                           description:""`
	BillingRuleNo                     string     `json:"billingRuleNo"                     orm:"billing_rule_no"                      description:""`
	ConfigurationSnapshotNo           string     `json:"configurationSnapshotNo"           orm:"configuration_snapshot_no"            description:""`
	CurrencyCode                      string     `json:"currencyCode"                      orm:"currency_code"                        description:""`
	PurchaseCycles                    int        `json:"purchaseCycles"                    orm:"purchase_cycles"                      description:""`
	PeriodUnitCode                    string     `json:"periodUnitCode"                    orm:"period_unit_code"                     description:""`
	PeriodCount                       int        `json:"periodCount"                       orm:"period_count"                         description:""`
	RenewalModeCode                   string     `json:"renewalModeCode"                   orm:"renewal_mode_code"                    description:""`
	CapabilityEvidenceNo              *string    `json:"capabilityEvidenceNo"              orm:"capability_evidence_no"               description:""`
	CapabilityEvidenceDigest          *string    `json:"capabilityEvidenceDigest"          orm:"capability_evidence_digest"           description:""`
	CapabilityEvidenceObservedAt      *time.Time `json:"capabilityEvidenceObservedAt"      orm:"capability_evidence_observed_at"      description:""`
	CapabilityEvidenceExpiresAt       *time.Time `json:"capabilityEvidenceExpiresAt"       orm:"capability_evidence_expires_at"       description:""`
	OriginalExpiresAt                 *time.Time `json:"originalExpiresAt"                 orm:"original_expires_at"                  description:""`
	ProjectedExpiresAt                *time.Time `json:"projectedExpiresAt"                orm:"projected_expires_at"                 description:""`
	TaxPolicyNo                       string     `json:"taxPolicyNo"                       orm:"tax_policy_no"                        description:""`
	PriceIncludesTax                  bool       `json:"priceIncludesTax"                  orm:"price_includes_tax"                   description:""`
	TaxTreatmentCode                  string     `json:"taxTreatmentCode"                  orm:"tax_treatment_code"                   description:""`
	TaxRateBasisPoints                *int       `json:"taxRateBasisPoints"                orm:"tax_rate_basis_points"                description:""`
	TaxCategoryCode                   string     `json:"taxCategoryCode"                   orm:"tax_category_code"                    description:""`
	RoundingScopeCode                 string     `json:"roundingScopeCode"                 orm:"rounding_scope_code"                  description:""`
	RoundingModeCode                  string     `json:"roundingModeCode"                  orm:"rounding_mode_code"                   description:""`
	RoundingQuantumMinor              int64      `json:"roundingQuantumMinor"              orm:"rounding_quantum_minor"               description:""`
	ComponentCount                    int        `json:"componentCount"                    orm:"component_count"                      description:""`
	SubtotalMinor                     int64      `json:"subtotalMinor"                     orm:"subtotal_minor"                       description:""`
	DiscountMinor                     int64      `json:"discountMinor"                     orm:"discount_minor"                       description:""`
	NetAmountMinor                    int64      `json:"netAmountMinor"                    orm:"net_amount_minor"                     description:""`
	TaxAmountMinor                    int64      `json:"taxAmountMinor"                    orm:"tax_amount_minor"                     description:""`
	TotalAmountMinor                  int64      `json:"totalAmountMinor"                  orm:"total_amount_minor"                   description:""`
	PricingRuleDigest                 string     `json:"pricingRuleDigest"                 orm:"pricing_rule_digest"                  description:""`
	ContentDigest                     string     `json:"contentDigest"                     orm:"content_digest"                       description:""`
	CreatedAt                         *time.Time `json:"createdAt"                         orm:"created_at"                           description:""`
	SubjectTypeCode                   string     `json:"subjectTypeCode"                   orm:"subject_type_code"                    description:""`
	SubjectNo                         string     `json:"subjectNo"                         orm:"subject_no"                           description:""`
	StructuredConfigurationSnapshotNo string     `json:"structuredConfigurationSnapshotNo" orm:"structured_configuration_snapshot_no" description:""`
}
