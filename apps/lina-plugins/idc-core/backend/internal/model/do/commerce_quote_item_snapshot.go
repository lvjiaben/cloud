// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteItemSnapshot is the golang structure of table plugin_idc_core_commerce_quote_item_snapshot for DAO operations like Where/Data.
type CommerceQuoteItemSnapshot struct {
	g.Meta                  `orm:"table:plugin_idc_core_commerce_quote_item_snapshot, do:true"`
	Id                      any        //
	TenantId                any        //
	QuoteItemNo             any        //
	QuoteNo                 any        //
	CartNo                  *string    //
	CartItemNo              *string    //
	LineNumber              any        //
	CurrencyCode            any        //
	ProductNo               any        //
	VersionNo               any        //
	BillingRuleNo           any        //
	ConfigurationSnapshotNo any        //
	ProductNameZhCn         any        //
	ProductNameEnUs         any        //
	PurchaseCycles          any        //
	ChargeTypeCode          any        //
	PeriodUnitCode          any        //
	PeriodCount             any        //
	RequiresContract        any        //
	FulfillmentModeCode     any        //
	RiskDecisionCode        any        //
	InventoryDecisionCode   any        //
	RouteDecisionDigest     any        //
	TaxPolicyNo             any        //
	PriceIncludesTax        any        //
	TaxTreatmentCode        any        //
	TaxRateBasisPoints      *int       //
	TaxCategoryCode         any        //
	RoundingScopeCode       any        //
	RoundingModeCode        any        //
	RoundingQuantumMinor    any        //
	SetupSubtotalMinor      any        //
	PeriodSubtotalMinor     any        //
	SubtotalMinor           any        //
	DiscountMinor           any        //
	NetAmountMinor          any        //
	TaxAmountMinor          any        //
	TotalAmountMinor        any        //
	ContentDigest           any        //
	CreatedAt               *time.Time //
	TrialLimitQuantity      any        //
	PriorTrialUses          any        //
	ContractPolicyCode      any        //
	ContractTemplateNo      any        //
}
