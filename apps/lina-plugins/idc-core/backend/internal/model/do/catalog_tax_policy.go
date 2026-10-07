// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogTaxPolicy is the golang structure of table plugin_idc_core_catalog_tax_policy for DAO operations like Where/Data.
type CatalogTaxPolicy struct {
	g.Meta               `orm:"table:plugin_idc_core_catalog_tax_policy, do:true"`
	Id                   any        //
	TenantId             any        //
	TaxPolicyNo          any        //
	VersionNo            any        //
	CurrencyCode         any        //
	PriceIncludesTax     any        //
	TaxTreatmentCode     any        //
	TaxRateBasisPoints   *int       // Tax rate in basis points; null only for exempt treatment
	TaxCategoryCode      any        //
	RoundingScopeCode    any        //
	RoundingModeCode     any        //
	RoundingQuantumMinor any        // Positive rounding quantum expressed in the configured currency minor unit
	CreatedAt            *time.Time //
}
