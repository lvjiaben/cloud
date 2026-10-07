// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingFundHold is the golang structure of table plugin_idc_core_billing_fund_hold for DAO operations like Where/Data.
type BillingFundHold struct {
	g.Meta              `orm:"table:plugin_idc_core_billing_fund_hold, do:true"`
	Id                  any        //
	TenantId            any        //
	HoldNo              any        //
	CustomerNo          any        //
	AccountNo           any        //
	AccountTypeCode     any        //
	CurrencyCode        any        //
	SourceTypeCode      any        //
	SourceReference     any        //
	AmountMinor         any        //
	ConsumedAmountMinor any        //
	ReleasedAmountMinor any        //
	StatusCode          any        //
	ContentDigest       any        //
	ExpiresAt           *time.Time //
	ConsumedAt          *time.Time //
	ReleasedAt          *time.Time //
	Version             any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
