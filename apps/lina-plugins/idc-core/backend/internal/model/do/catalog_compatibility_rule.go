// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogCompatibilityRule is the golang structure of table plugin_idc_core_catalog_compatibility_rule for DAO operations like Where/Data.
type CatalogCompatibilityRule struct {
	g.Meta                `orm:"table:plugin_idc_core_catalog_compatibility_rule, do:true"`
	Id                    any        //
	TenantId              any        //
	RuleNo                any        //
	VersionNo             any        //
	RuleTypeCode          any        //
	SourceOptionNo        any        //
	TargetOptionNo        any        //
	MinimumTargetQuantity any        //
	MaximumTargetQuantity *int       //
	StatusCode            any        //
	CreatedAt             *time.Time //
}
