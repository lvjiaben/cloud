// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentProductRule is the golang structure of table plugin_idc_core_sales_agent_product_rule for DAO operations like Where/Data.
type SalesAgentProductRule struct {
	g.Meta        `orm:"table:plugin_idc_core_sales_agent_product_rule, do:true"`
	Id            any        //
	TenantId      any        //
	ProductRuleNo any        //
	LevelNo       any        //
	ProductNo     any        //
	StatusCode    any        //
	EffectiveAt   *time.Time //
	ExpiresAt     *time.Time //
	Version       any        //
	RuleDigest    any        //
	CreatedAt     *time.Time //
}
