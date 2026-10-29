// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NativeServerTargetQuote is the golang structure of table plugin_idc_core_native_server_target_quote for DAO operations like Where/Data.
type NativeServerTargetQuote struct {
	g.Meta                     `orm:"table:plugin_idc_core_native_server_target_quote, do:true"`
	TenantId                   any        //
	Id                         any        //
	TargetQuoteNo              any        //
	QuoteItemNo                any        //
	StructuredConfigurationNo  any        //
	EntitlementNo              any        //
	CustomerNo                 any        //
	ExpectedEntitlementVersion any        //
	CustomerSubjectVersion     any        //
	CurrencyCode               any        //
	NativeTargetSnapshot       any        //
	CreatedAt                  *time.Time //
}
