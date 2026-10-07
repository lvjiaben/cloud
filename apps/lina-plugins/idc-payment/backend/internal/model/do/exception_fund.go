// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ExceptionFund is the golang structure of table plugin_idc_payment_exception_fund for DAO operations like Where/Data.
type ExceptionFund struct {
	g.Meta         `orm:"table:plugin_idc_payment_exception_fund, do:true"`
	Id             any        //
	ExceptionNo    any        //
	AttemptNo      any        //
	ChannelNo      any        //
	CurrencyCode   any        //
	AmountMinor    any        //
	ReasonCode     any        //
	StatusCode     any        //
	EvidenceDigest any        //
	CreatedAt      *time.Time //
	ResolvedAt     *time.Time //
}
