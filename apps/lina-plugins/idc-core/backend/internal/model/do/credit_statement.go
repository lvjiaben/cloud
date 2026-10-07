// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditStatement is the golang structure of table plugin_idc_core_credit_statement for DAO operations like Where/Data.
type CreditStatement struct {
	g.Meta                 `orm:"table:plugin_idc_core_credit_statement, do:true"`
	Id                     any        //
	TenantId               any        //
	StatementNo            any        //
	CreditAccountNo        any        //
	CustomerNo             any        //
	CurrencyCode           any        //
	TermsNo                any        //
	TermsVersion           any        //
	PeriodStartedAt        *time.Time //
	PeriodEndedAt          *time.Time //
	LineCount              any        //
	PrincipalMinor         any        //
	FeeMinor               any        //
	AdjustmentMinor        any        //
	TotalAmountMinor       any        //
	PaidAmountMinor        any        //
	WrittenOffAmountMinor  any        //
	OutstandingAmountMinor any        //
	StatusCode             any        //
	ContentDigest          any        //
	IssuedAt               *time.Time //
	PaymentDueAt           *time.Time //
	OverdueAt              *time.Time //
	SettledAt              *time.Time //
	CancelledAt            *time.Time //
	WrittenOffAt           *time.Time //
	Version                any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
}
