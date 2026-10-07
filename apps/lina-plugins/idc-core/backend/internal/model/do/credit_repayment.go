// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditRepayment is the golang structure of table plugin_idc_core_credit_repayment for DAO operations like Where/Data.
type CreditRepayment struct {
	g.Meta                     `orm:"table:plugin_idc_core_credit_repayment, do:true"`
	Id                         any        //
	TenantId                   any        //
	RepaymentNo                any        //
	CreditAccountNo            any        //
	CustomerNo                 any        //
	CurrencyCode               any        //
	RepaymentTypeCode          any        //
	FundingSourceTypeCode      any        //
	FundingSourceReference     any        //
	LedgerTransactionNo        *string    //
	AmountMinor                any        //
	AllocatedAmountMinor       any        //
	UnallocatedAmountMinor     any        //
	RefundedAmountMinor        any        //
	StatusCode                 any        //
	SourceSnapshotDigest       any        //
	ConfirmationEvidenceDigest *string    //
	ConfirmedAt                *time.Time //
	FailedAt                   *time.Time //
	RefundedAt                 *time.Time //
	Version                    any        //
	CreatedAt                  *time.Time //
	UpdatedAt                  *time.Time //
}
