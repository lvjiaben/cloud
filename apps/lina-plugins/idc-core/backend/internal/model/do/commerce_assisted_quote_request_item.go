// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceAssistedQuoteRequestItem is the golang structure of table plugin_idc_core_commerce_assisted_quote_request_item for DAO operations like Where/Data.
type CommerceAssistedQuoteRequestItem struct {
	g.Meta                  `orm:"table:plugin_idc_core_commerce_assisted_quote_request_item, do:true"`
	Id                      any        //
	TenantId                any        //
	RequestItemNo           any        //
	RequestNo               any        //
	CurrencyCode            any        //
	LineNumber              any        //
	ProductNo               any        //
	VersionNo               any        //
	BillingRuleNo           any        //
	ConfigurationSnapshotNo any        //
	ProductNameZhCn         any        //
	ProductNameEnUs         any        //
	PurchaseCycles          any        //
	ContentDigest           any        //
	CreatedAt               *time.Time //
}
