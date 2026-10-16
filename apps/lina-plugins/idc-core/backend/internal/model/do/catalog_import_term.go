// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportTerm is the golang structure of table plugin_idc_core_catalog_import_term for DAO operations like Where/Data.
type CatalogImportTerm struct {
	g.Meta                `orm:"table:plugin_idc_core_catalog_import_term, do:true"`
	Id                    any        //
	TenantId              any        //
	VersionNo             any        //
	BillingRuleNo         any        //
	CurrencyCode          any        //
	UpstreamTermReference any        //
	WholeItemRule         any        //
	MarkupPercent         any        //
	CreatedAt             *time.Time //
}
