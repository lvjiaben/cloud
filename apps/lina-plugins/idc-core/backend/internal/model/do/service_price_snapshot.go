// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServicePriceSnapshot is the golang structure of table plugin_idc_core_service_price_snapshot for DAO operations like Where/Data.
type ServicePriceSnapshot struct {
	g.Meta                            `orm:"table:plugin_idc_core_service_price_snapshot, do:true"`
	Id                                any        //
	TenantId                          any        //
	PriceSnapshotNo                   any        //
	ServiceNo                         any        //
	CustomerNo                        any        //
	PhaseCode                         any        //
	SourceQuoteNo                     *string    //
	SourceQuoteItemNo                 *string    //
	ProductNo                         any        //
	VersionNo                         any        //
	BillingRuleNo                     any        //
	ConfigurationSnapshotNo           any        //
	CurrencyCode                      any        //
	PurchaseCycles                    any        //
	PeriodUnitCode                    any        //
	PeriodCount                       any        //
	RenewalModeCode                   any        //
	CapabilityEvidenceNo              *string    //
	CapabilityEvidenceDigest          *string    //
	CapabilityEvidenceObservedAt      *time.Time //
	CapabilityEvidenceExpiresAt       *time.Time //
	OriginalExpiresAt                 *time.Time //
	ProjectedExpiresAt                *time.Time //
	TaxPolicyNo                       any        //
	PriceIncludesTax                  any        //
	TaxTreatmentCode                  any        //
	TaxRateBasisPoints                *int       //
	TaxCategoryCode                   any        //
	RoundingScopeCode                 any        //
	RoundingModeCode                  any        //
	RoundingQuantumMinor              any        //
	ComponentCount                    any        //
	SubtotalMinor                     any        //
	DiscountMinor                     any        //
	NetAmountMinor                    any        //
	TaxAmountMinor                    any        //
	TotalAmountMinor                  any        //
	PricingRuleDigest                 any        //
	ContentDigest                     any        //
	CreatedAt                         *time.Time //
	SubjectTypeCode                   any        //
	SubjectNo                         any        //
	StructuredConfigurationSnapshotNo any        //
}
