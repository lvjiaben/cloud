// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ExceptionFundAction is the golang structure for table exception_fund_action.
type ExceptionFundAction struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	ActionNo        string     `json:"actionNo"        orm:"action_no"         description:""`
	ExceptionNo     string     `json:"exceptionNo"     orm:"exception_no"      description:""`
	ActionCode      string     `json:"actionCode"      orm:"action_code"       description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	TargetOrderNo   string     `json:"targetOrderNo"   orm:"target_order_no"   description:""`
	TargetInvoiceNo string     `json:"targetInvoiceNo" orm:"target_invoice_no" description:""`
	AllocationNo    string     `json:"allocationNo"    orm:"allocation_no"     description:""`
	SettlementNo    string     `json:"settlementNo"    orm:"settlement_no"     description:""`
	RefundAttemptNo string     `json:"refundAttemptNo" orm:"refund_attempt_no" description:""`
	ProofReference  string     `json:"proofReference"  orm:"proof_reference"   description:""`
	ProofDigest     string     `json:"proofDigest"     orm:"proof_digest"      description:""`
	RequestedBy     int        `json:"requestedBy"     orm:"requested_by"      description:""`
	CompletedBy     int        `json:"completedBy"     orm:"completed_by"      description:""`
	RequestDigest   string     `json:"requestDigest"   orm:"request_digest"    description:""`
	FailureCode     string     `json:"failureCode"     orm:"failure_code"      description:""`
	Version         int64      `json:"version"         orm:"version"           description:""`
	RequestedAt     *time.Time `json:"requestedAt"     orm:"requested_at"      description:""`
	CompletedAt     *time.Time `json:"completedAt"     orm:"completed_at"      description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
