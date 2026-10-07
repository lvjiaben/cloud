// This file issues an exact-preview proof for customer balance credit repayment.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

// CreditBalanceRepaymentProof issues an exact-preview repayment proof.
func (c *ControllerV1) CreditBalanceRepaymentProof(ctx context.Context, req *v1.CreditBalanceRepaymentProofReq) (res *v1.CreditBalanceRepaymentProofRes, err error) {
	intent, err := creditBalanceRepaymentIntent(req.StatementNo, req.CreditBalanceRepaymentIntent)
	if err != nil {
		return nil, err
	}
	proof, err := c.creditCommands.IssueCreditBalanceRepaymentProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.CreditBalanceRepaymentProofRes{Proof: toAPICreditProof(proof)}, nil
}
