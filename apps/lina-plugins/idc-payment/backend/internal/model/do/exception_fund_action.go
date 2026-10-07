// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ExceptionFundAction is the golang structure of table plugin_idc_payment_exception_fund_action for DAO operations like Where/Data.
type ExceptionFundAction struct {
	g.Meta          `orm:"table:plugin_idc_payment_exception_fund_action, do:true"`
	Id              any        //
	ActionNo        any        //
	ExceptionNo     any        //
	ActionCode      any        //
	StatusCode      any        //
	TargetOrderNo   any        //
	TargetInvoiceNo any        //
	AllocationNo    any        //
	SettlementNo    any        //
	RefundAttemptNo any        //
	ProofReference  any        //
	ProofDigest     any        //
	RequestedBy     any        //
	CompletedBy     any        //
	RequestDigest   any        //
	FailureCode     any        //
	Version         any        //
	RequestedAt     *time.Time //
	CompletedAt     *time.Time //
	UpdatedAt       *time.Time //
}
