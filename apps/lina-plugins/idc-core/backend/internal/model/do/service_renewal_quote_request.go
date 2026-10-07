// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceRenewalQuoteRequest is the golang structure of table plugin_idc_core_service_renewal_quote_request for DAO operations like Where/Data.
type ServiceRenewalQuoteRequest struct {
	g.Meta                     `orm:"table:plugin_idc_core_service_renewal_quote_request, do:true"`
	Id                         any        //
	TenantId                   any        //
	RequestNo                  any        //
	CustomerNo                 any        //
	EntitlementNo              any        //
	CurrencyCode               any        //
	RequestSourceCode          any        //
	RenewalCycles              any        //
	IdempotencyDigest          any        //
	RequestDigest              any        //
	QuoteNo                    any        //
	PriceSnapshotNo            any        //
	ExpectedEntitlementVersion any        //
	ContentDigest              any        //
	CreatedAt                  *time.Time //
}
