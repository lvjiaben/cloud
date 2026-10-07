// This file contains the public projections shared by the contract-failure
// policy and immediate-cancellation APIs.

package v1

// ElectronicContractFailurePolicyItem is the active installation-local rule.
type ElectronicContractFailurePolicyItem struct {
	PolicyNo      string `json:"policyNo" dc:"Opaque policy revision number" eg:"cfp_0123456789abcdef0123456789abcdef"`
	PolicyVersion int64  `json:"policyVersion" dc:"Monotonic policy version" eg:"2"`
	HandlingDays  int    `json:"handlingDays" dc:"Natural-day handling period from 0 through 30" eg:"3"`
	CreatedAt     int64  `json:"createdAt" dc:"Policy creation time as Unix timestamp in milliseconds" eg:"1787011200000"`
	Reused        bool   `json:"reused" dc:"Whether an exact update replay returned the existing revision" eg:"false"`
}

// ElectronicContractFailureCompensationItem is a safe local handoff result.
type ElectronicContractFailureCompensationItem struct {
	CompensationNo  string `json:"compensationNo" dc:"Core-owned compensation number" eg:"ccp_0123456789abcdef0123456789abcdef"`
	RefundRequestNo string `json:"refundRequestNo" dc:"Payment-owner handoff; empty when no money needs returning" eg:"rfr_0123456789abcdef0123456789abcdef"`
	OrderNo         string `json:"orderNo" dc:"Stopped order number" eg:"ORD_0123456789ABCDEF"`
	StatusCode      string `json:"statusCode" dc:"refund_pending, completed, or manual_review" eg:"refund_pending"`
	ReasonCode      string `json:"reasonCode" dc:"Safe machine-readable decision reason" eg:"required_contract_terminal"`
	Reused          bool   `json:"reused" dc:"Whether the exact compensation was already created" eg:"false"`
}
