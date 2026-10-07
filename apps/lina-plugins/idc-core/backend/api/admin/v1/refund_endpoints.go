// This file defines staff refund review, high-amount proof, policy, and mutation DTOs.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RefundManagementReviewIntent struct {
	RequestNo       string `json:"requestNo" v:"required|min-length:8|max-length:128" dc:"Random action key kept unchanged for proof issuance and retries" eg:"refund-review-01J2ABCDEF"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Refund version reviewed by the employee" eg:"1"`
	DecisionCode    string `json:"decisionCode" v:"required|in:approve,reject" dc:"Approve or reject the pending refund" eg:"approve"`
	CommentDigest   string `json:"commentDigest" v:"regex:^$|^[0-9a-f]{64}$" dc:"Optional lowercase SHA-256 digest of the review comment" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	EvidenceDigest  string `json:"evidenceDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"Lowercase SHA-256 digest of the immutable review evidence" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type RefundManagementReviewProofReq struct {
	g.Meta   `path:"/admin/billing/refunds/{refundNo}/review-proofs" method:"post" tags:"IDC Refund Management" summary:"Issue a high-amount refund review proof" dc:"Issue an exact single-use proof after a recent full login only when this pending refund is high amount and the requested decision is approval." permission:"idc:admin:billing:refund:review"`
	RefundNo string `json:"refundNo" in:"path" v:"required|max-length:40" dc:"Stable pending refund number" eg:"rfd_01J2ABCDEF"`
	RefundManagementReviewIntent
}

type RefundManagementReviewProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived exact-review proof" eg:"{}"`
}

type RefundManagementReviewReq struct {
	g.Meta   `path:"/admin/billing/refunds/{refundNo}/reviews" method:"post" tags:"IDC Refund Management" summary:"Review a pending refund" dc:"Approve or reject one exact pending refund version. High-amount approval consumes the exact proof; rejection never requires that proof." permission:"idc:admin:billing:refund:review"`
	RefundNo string `json:"refundNo" in:"path" v:"required|max-length:40" dc:"Stable pending refund number" eg:"rfd_01J2ABCDEF"`
	RefundManagementReviewIntent
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Required only for high-amount approval and issued for this exact review" eg:"opaque-proof"`
}

type RefundManagementReviewRes struct {
	Refund *RefundManagementMutation `json:"refund" dc:"Committed refund review result" eg:"{}"`
}

type RefundManagementPolicyConfigureReq struct {
	g.Meta                   `path:"/admin/billing/refund-policies" method:"post" tags:"IDC Refund Management" summary:"Configure a currency refund policy" dc:"Append one immutable currency-local policy version using optimistic concurrency." permission:"idc:admin:billing:refund-policy:write"`
	CurrencyCode             string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured currency" eg:"CNY"`
	HighAmountThresholdMinor string `json:"highAmountThresholdMinor" v:"required|regex:^[1-9][0-9]{0,18}$" dc:"Amount at or above which approval requires a recent full login" eg:"500000"`
	AutomaticApprovalEnabled bool   `json:"automaticApprovalEnabled" dc:"Allow only clear unprovisioned and risk-free requests to be approved automatically" eg:"true"`
	ExpectedVersion          int64  `json:"expectedVersion" v:"min:0" dc:"Latest currency policy version observed before editing; zero creates the first version" eg:"0"`
}

type RefundManagementPolicyConfigureRes struct {
	Policy *RefundManagementPolicy `json:"policy" dc:"Committed immutable refund policy version" eg:"{}"`
}

type RefundManagementMutation struct {
	RefundNo                       string `json:"refundNo" dc:"Stable refund number" eg:"rfd_01J2ABCDEF"`
	CustomerNo                     string `json:"customerNo" dc:"Authoritative customer number" eg:"CUS_01J2ABCDEF"`
	CustomerCode                   string `json:"customerCode" dc:"Short numeric customer number" eg:"10000"`
	CustomerDisplayName            string `json:"customerDisplayName" dc:"Display-safe customer name"`
	CustomerPrimaryPhone           string `json:"customerPrimaryPhone" dc:"Primary customer phone"`
	OrderNo                        string `json:"orderNo" dc:"Authoritative order number" eg:"ORD_01J2ABCDEF"`
	InvoiceNo                      string `json:"invoiceNo" dc:"Authoritative invoice number" eg:"BIL_01J2ABCDEF"`
	CurrencyCode                   string `json:"currencyCode" dc:"Exact refund currency" eg:"CNY"`
	StatusCode                     string `json:"statusCode" dc:"Committed refund status" eg:"pending_review"`
	ReasonCode                     string `json:"reasonCode" dc:"Customer-selected refund reason" eg:"wrong_configuration"`
	ReasonDetail                   string `json:"reasonDetail" dc:"Optional customer reason detail" eg:"This item is no longer needed"`
	AmountMinor                    string `json:"amountMinor" dc:"Exact refund amount in decimal minor units" eg:"1299"`
	ApprovalModeCode               string `json:"approvalModeCode" dc:"Automatic or manual approval mode" eg:"manual"`
	DestinationCode                string `json:"destinationCode" dc:"Original source, same-currency balance, or mixed destination" eg:"original_channel"`
	RequiresEntitlementTermination bool   `json:"requiresEntitlementTermination" dc:"Whether a selected long-lived entitlement must be terminated" eg:"true"`
	HasEntitlementImpact           bool   `json:"hasEntitlementImpact" dc:"Whether a selected long-lived entitlement is affected" eg:"true"`
	HasDangerousOperation          bool   `json:"hasDangerousOperation" dc:"Whether a dangerous or unknown-result operation conflicts" eg:"false"`
	HighAmount                     bool   `json:"highAmount" dc:"Whether approval crosses the currency high-amount threshold" eg:"false"`
	ImpactDigest                   string `json:"impactDigest" dc:"Immutable selected-item and entitlement-impact digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Version                        int64  `json:"version" dc:"Current optimistic refund version" eg:"1"`
	SubmittedAt                    int64  `json:"submittedAt" dc:"Submission time as Unix timestamp in milliseconds" eg:"1735689600000"`
	ApprovedAt                     *int64 `json:"approvedAt,omitempty" dc:"Approval time as Unix timestamp in milliseconds, or empty" eg:"1735689660000"`
	CompletedAt                    *int64 `json:"completedAt,omitempty" dc:"Completion time as Unix timestamp in milliseconds, or empty" eg:"1735689720000"`
	Reused                         bool   `json:"reused" dc:"Whether an exact safe replay returned the committed result" eg:"false"`
}

type RefundManagementPolicy struct {
	PolicyNo                 string `json:"policyNo" dc:"Stable immutable policy version number" eg:"rfp_01J2ABCDEF"`
	CurrencyCode             string `json:"currencyCode" dc:"Exact policy currency" eg:"CNY"`
	Version                  int64  `json:"version" dc:"Monotonic currency-local policy version" eg:"1"`
	HighAmountThresholdMinor string `json:"highAmountThresholdMinor" dc:"High-amount approval threshold in decimal minor units" eg:"500000"`
	AutomaticApprovalEnabled bool   `json:"automaticApprovalEnabled" dc:"Whether clear unprovisioned risk-free requests may auto approve" eg:"true"`
	ConfiguredBy             int    `json:"configuredBy" dc:"Employee that appended this version" eg:"1"`
	EffectiveAt              int64  `json:"effectiveAt" dc:"Effective time as Unix timestamp in milliseconds" eg:"1735689600000"`
}
