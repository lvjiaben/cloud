// This file defines proof and execution contracts for customer balance credit repayment.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CreditBalanceRepaymentIntent is the exact repayment preview shared by proof and execution.
type CreditBalanceRepaymentIntent struct {
	AmountMinor string `json:"amountMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Exact partial or full repayment in decimal minor units" eg:"10000"`
	RequestNo   string `json:"requestNo" v:"required|min-length:8|max-length:128" dc:"Stable repayment business request number" eg:"credit-repay-01J2"`
}

// CreditBalanceRepaymentProofReq requests a proof bound to one owned statement repayment.
type CreditBalanceRepaymentProofReq struct {
	g.Meta      `path:"/account/billing/credit/statements/{statementNo}/balance-repayment-proofs" method:"post" tags:"IDC Customer Credit" summary:"Issue my credit repayment proof" dc:"Verify statement ownership and a current active session before issuing an exact-preview single-use proof."`
	StatementNo string `json:"statementNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer credit statement number" eg:"CST_01J2ABCDEF"`
	CreditBalanceRepaymentIntent
}

// CreditBalanceRepaymentProofRes returns one short-lived single-use proof.
type CreditBalanceRepaymentProofRes struct {
	Proof *CreditActionProof `json:"proof" dc:"Short-lived single-use repayment proof" eg:"{}"`
}

// CreditBalanceRepaymentCreateReq consumes an exact proof to repay from customer balance.
type CreditBalanceRepaymentCreateReq struct {
	g.Meta      `path:"/account/billing/credit/statements/{statementNo}/balance-repayments" method:"post" tags:"IDC Customer Credit" summary:"Repay my credit statement from balance" dc:"Consume the exact-preview proof and atomically debit only my same-currency available balance before allocating repayment."`
	StatementNo string `json:"statementNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer credit statement number" eg:"CST_01J2ABCDEF"`
	CreditBalanceRepaymentIntent
	ProofToken string `json:"proofToken" v:"required|max-length:4096" dc:"Opaque proof issued for this exact repayment preview" eg:"opaque-proof"`
}

// CreditBalanceRepaymentCreateRes returns the atomic committed repayment outcome.
type CreditBalanceRepaymentCreateRes struct {
	Result *CreditBalanceRepaymentResult `json:"result" dc:"Committed customer-safe credit repayment result" eg:"{}"`
}
