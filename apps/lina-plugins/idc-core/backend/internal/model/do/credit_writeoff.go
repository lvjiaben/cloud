// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditWriteoff is the golang structure of table plugin_idc_core_credit_writeoff for DAO operations like Where/Data.
type CreditWriteoff struct {
	g.Meta                 `orm:"table:plugin_idc_core_credit_writeoff, do:true"`
	Id                     any        //
	TenantId               any        //
	WriteoffNo             any        //
	StatementNo            any        //
	CreditAccountNo        any        //
	CustomerNo             any        //
	CurrencyCode           any        //
	WriteoffTypeCode       any        //
	AmountMinor            any        //
	OriginalWriteoffNo     *string    //
	LedgerTransactionNo    any        //
	ReasonCode             any        //
	RequestedBy            any        //
	ReviewedBy             any        //
	ApprovalEvidenceDigest any        //
	ContentDigest          any        //
	PostedAt               *time.Time //
	CreatedAt              *time.Time //
}
