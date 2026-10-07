// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditAccount is the golang structure of table plugin_idc_core_credit_account for DAO operations like Where/Data.
type CreditAccount struct {
	g.Meta                   `orm:"table:plugin_idc_core_credit_account, do:true"`
	Id                       any        //
	TenantId                 any        //
	CreditAccountNo          any        //
	CustomerNo               any        //
	CurrencyCode             any        //
	LedgerAccountNo          any        //
	LedgerAccountTypeCode    any        //
	OriginApplicationNo      any        //
	CurrentTermsNo           any        //
	CurrentTermsVersion      any        //
	StatusCode               any        //
	TotalLimitMinor          any        //
	AvailableLimitMinor      any        //
	UnbilledPrincipalMinor   any        //
	BilledPrincipalMinor     any        //
	DelinquentPrincipalMinor any        //
	WrittenOffPrincipalMinor any        //
	ValidFrom                *time.Time //
	ValidUntil               *time.Time //
	FreezeReasonCode         *string    //
	FrozenAt                 *time.Time //
	ClosedAt                 *time.Time //
	Version                  any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
