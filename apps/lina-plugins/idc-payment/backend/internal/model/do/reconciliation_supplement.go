// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationSupplement is the golang structure of table plugin_idc_payment_reconciliation_supplement for DAO operations like Where/Data.
type ReconciliationSupplement struct {
	g.Meta              `orm:"table:plugin_idc_payment_reconciliation_supplement, do:true"`
	Id                  any        //
	RequestNo           any        //
	LineNo              any        //
	AttemptNo           any        //
	ChannelNo           any        //
	CustomerRefDigest   any        //
	CurrencyCode        any        //
	AmountMinor         any        //
	ExternalReference   any        //
	EvidenceDigest      any        //
	ContentDigest       any        //
	StatusCode          any        //
	SubmitterUserId     any        //
	ReviewerUserId      any        //
	RejectionReasonCode any        //
	Version             any        //
	SubmittedAt         *time.Time //
	ReviewedAt          *time.Time //
	AppliedAt           *time.Time //
}
