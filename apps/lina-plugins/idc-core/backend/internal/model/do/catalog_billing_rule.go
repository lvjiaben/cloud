// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogBillingRule is the golang structure of table plugin_idc_core_catalog_billing_rule for DAO operations like Where/Data.
type CatalogBillingRule struct {
	g.Meta                  `orm:"table:plugin_idc_core_catalog_billing_rule, do:true"`
	Id                      any        //
	TenantId                any        //
	BillingRuleNo           any        //
	VersionNo               any        //
	RuleCode                any        //
	ChargeTypeCode          any        //
	PeriodUnitCode          any        //
	PeriodCount             any        //
	PricePhaseCode          any        //
	QuantityModeCode        any        //
	TrialQuantity           any        //
	StatusCode              any        //
	CreatedAt               *time.Time //
	MinimumPurchaseCycles   any        // Minimum number of this explicit billing period accepted for a new purchase
	MaximumRenewalCycles    *int       // Optional maximum number of this explicit billing period accepted per renewal quote
	MeterUnitCode           *string    // Merchant-defined normalized authoritative meter unit for usage billing
	TierModeCode            *string    //
	UsageCollectionUnitCode *string    //
	UsageCollectionCount    *int       //
	UsageRoundingModeCode   *string    //
	UsageRoundingQuantity   *int64     //
	UsageMinimumQuantity    *int64     //
	UsageMaximumQuantity    *int64     //
	UsageMissingPolicyCode  *string    // Fail-closed pending recalculation or explicit previous-period estimate policy
}
