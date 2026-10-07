// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingLedgerAccount is the golang structure of table plugin_idc_core_billing_ledger_account for DAO operations like Where/Data.
type BillingLedgerAccount struct {
	g.Meta            `orm:"table:plugin_idc_core_billing_ledger_account, do:true"`
	Id                any        //
	TenantId          any        //
	AccountNo         any        //
	OwnerTypeCode     any        //
	OwnerReference    any        //
	CustomerNo        *string    //
	AccountTypeCode   any        //
	NormalBalanceCode any        //
	CurrencyCode      any        //
	StatusCode        any        //
	Withdrawable      any        //
	ClosedAt          *time.Time //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
