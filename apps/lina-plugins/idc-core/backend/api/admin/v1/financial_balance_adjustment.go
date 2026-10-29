// This file defines staff-only customer gift-credit and available-cash debit
// actions. Every execution consumes a proof bound to the exact preview.

package v1

import "github.com/gogf/gf/v2/frame/g"

// FinancialManagementBalanceAdjustmentIntent excludes raw ledger directions
// and arbitrary account numbers from the public action contract.
type FinancialManagementBalanceAdjustmentIntent struct {
	RequestNo      string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Random action key stable across proof issuance and retries" eg:"balance-01J2ABCDEF012345"`
	CustomerNo     string `json:"customerNo" v:"required|max-length:40" dc:"Exact customer receiving the gift or cash deduction" eg:"CUS_01J2ABCDEF"`
	CurrencyCode   string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured currency without conversion" eg:"CNY"`
	AmountMinor    string `json:"amountMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Positive amount in decimal minor units" eg:"500"`
	Deduct         bool   `json:"deduct" dc:"True deducts available cash only; false credits non-withdrawable gift balance" eg:"false"`
	ReasonCode     string `json:"reasonCode" v:"max-length:64" dc:"Optional adjustment reason; defaults to the operation code" eg:"manual_gift"`
	EvidenceDigest string `json:"evidenceDigest" v:"regex:^$|^[0-9a-f]{64}$" dc:"Optional annotation SHA-256 digest; absent input uses a system request trace" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type FinancialManagementBalanceAdjustmentProofReq struct {
	g.Meta `path:"/admin/billing/balance-adjustments/proofs" method:"post" tags:"IDC Financial Management" summary:"Issue a customer balance action proof" dc:"Authorize the exact customer balance adjustment preview and bind it to the current direct staff session." permission:"idc:admin:billing:balance-adjustment:create"`
	FinancialManagementBalanceAdjustmentIntent
}
type FinancialManagementBalanceAdjustmentProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use action proof" eg:"{}"`
}
type FinancialManagementBalanceAdjustmentCreateReq struct {
	g.Meta `path:"/admin/billing/balance-adjustments" method:"post" tags:"IDC Financial Management" summary:"Adjust a customer balance" dc:"Credit gift balance separately from real receipts or deduct available cash without touching frozen funds; post immediately with atomic audit." permission:"idc:admin:billing:balance-adjustment:create"`
	FinancialManagementBalanceAdjustmentIntent
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Optional exact-preview proof; current action permission is sufficient" eg:"opaque-proof"`
}
type FinancialManagementBalanceAdjustmentCreateRes struct {
	Adjustment *FinancialManagementAdjustmentMutation `json:"adjustment" dc:"Committed balance adjustment" eg:"{}"`
}
