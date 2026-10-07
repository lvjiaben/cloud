// This file defines proof and append-only credit policy management contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ManagementCreditPolicyIntent is the exact immutable policy preview shared by proof and execution.
type ManagementCreditPolicyIntent struct {
	RequestNo                string `json:"requestNo" v:"required|min-length:8|max-length:128" dc:"Stable request number shared by proof issuance and execution" eg:"credit-policy-01J2"`
	CurrencyCode             string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured policy currency" eg:"CNY"`
	MaximumLimitMinor        string `json:"maximumLimitMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Maximum permitted account limit in decimal minor units" eg:"1000000"`
	DualReviewThresholdMinor string `json:"dualReviewThresholdMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Limit requiring two distinct reviewers in decimal minor units" eg:"500000"`
	BillingDay               int    `json:"billingDay" v:"required|min:1|max:28" dc:"Merchant-timezone monthly billing day" eg:"10"`
	RepaymentPeriodDays      int    `json:"repaymentPeriodDays" v:"required|min:1|max:365" dc:"Default repayment period in days" eg:"15"`
	GracePeriodDays          int    `json:"gracePeriodDays" v:"min:0|max:365" dc:"Default delinquency grace period in days" eg:"3"`
	DailyLateFeeBasisPoints  int    `json:"dailyLateFeeBasisPoints" v:"min:0|max:10000" dc:"Default daily late-fee rate in basis points" eg:"5"`
	MaximumLateFeeMinor      string `json:"maximumLateFeeMinor" v:"required|regex:^[0-9]{1,16}$" dc:"Default total late-fee cap in decimal minor units" eg:"5000"`
}

// ManagementCreditPolicyProofReq requests a proof bound to one policy preview.
type ManagementCreditPolicyProofReq struct {
	g.Meta `path:"/admin/billing/credit/policy-proofs" method:"post" tags:"IDC Credit Management" summary:"Issue a credit policy proof" dc:"Authorize the exact policy preview and issue a short-lived proof only after a recent full login." permission:"idc:admin:billing:credit-policy:write"`
	ManagementCreditPolicyIntent
}

// ManagementCreditPolicyProofRes returns one short-lived policy proof.
type ManagementCreditPolicyProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use policy proof" eg:"{}"`
}

// ManagementCreditPolicyConfigureReq consumes an exact proof to append one policy revision.
type ManagementCreditPolicyConfigureReq struct {
	g.Meta `path:"/admin/billing/credit/policies" method:"post" tags:"IDC Credit Management" summary:"Configure a credit policy" dc:"Re-authorize and consume the exact-preview proof before appending one immutable currency-local policy revision." permission:"idc:admin:billing:credit-policy:write"`
	ManagementCreditPolicyIntent
	ProofToken string `json:"proofToken" v:"required|max-length:4096" dc:"Opaque proof issued for this exact policy preview" eg:"opaque-proof"`
}

// ManagementCreditPolicyConfigureRes returns the committed immutable policy revision.
type ManagementCreditPolicyConfigureRes struct {
	Policy *ManagementCreditPolicyMutation `json:"policy" dc:"Committed immutable credit policy revision" eg:"{}"`
}
