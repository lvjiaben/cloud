// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationLine is the golang structure of table plugin_idc_payment_reconciliation_line for DAO operations like Where/Data.
type ReconciliationLine struct {
	g.Meta             `orm:"table:plugin_idc_payment_reconciliation_line, do:true"`
	Id                 any        //
	LineNo             any        //
	BatchNo            any        //
	AttemptNo          any        //
	RefundAttemptNo    any        //
	ExternalReference  any        //
	DifferenceCode     any        //
	LocalAmountMinor   any        //
	ChannelAmountMinor any        //
	CurrencyCode       any        //
	ResolutionCode     any        //
	EvidenceDigest     any        //
	CreatedAt          *time.Time //
	ResolvedAt         *time.Time //
	RecordTypeCode     any        //
	CustomerRefDigest  any        //
	LocalStatusCode    any        //
	ChannelStatusCode  any        //
	Version            any        //
}
