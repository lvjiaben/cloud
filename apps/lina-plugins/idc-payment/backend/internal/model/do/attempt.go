// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Attempt is the golang structure of table plugin_idc_payment_attempt for DAO operations like Where/Data.
type Attempt struct {
	g.Meta                 `orm:"table:plugin_idc_payment_attempt, do:true"`
	Id                     any        //
	TenantId               any        //
	AttemptNo              any        //
	ChannelNo              any        //
	InvoiceNo              any        //
	OrderNo                any        //
	PurposeCode            any        //
	PurposeReference       any        //
	CustomerRefDigest      any        //
	CurrencyCode           any        //
	AmountMinor            any        //
	BalanceLockReference   any        //
	BalanceLockedMinor     any        //
	ModeCode               any        //
	RequestDigest          any        //
	ExternalTradeReference any        //
	StatusCode             any        //
	ExpiresAt              *time.Time //
	SubmittedAt            *time.Time //
	ConfirmedAt            *time.Time //
	ClosedAt               *time.Time //
	ResultUnknown          any        //
	FundingResultDigest    any        //
	Version                any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
}
