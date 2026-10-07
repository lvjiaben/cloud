// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentPriceRule is the golang structure of table plugin_idc_core_sales_agent_price_rule for DAO operations like Where/Data.
type SalesAgentPriceRule struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_agent_price_rule, do:true"`
	Id                  any        //
	TenantId            any        //
	PriceRuleNo         any        //
	ProductRuleNo       any        //
	CurrencyCode        any        //
	AdjustmentTypeCode  any        //
	FixedPriceMinor     *int64     //
	DiscountBasisPoints *int       //
	MinimumMarginMinor  any        //
	StatusCode          any        //
	EffectiveAt         *time.Time //
	ExpiresAt           *time.Time //
	Version             any        //
	RuleDigest          any        //
	CreatedAt           *time.Time //
}
