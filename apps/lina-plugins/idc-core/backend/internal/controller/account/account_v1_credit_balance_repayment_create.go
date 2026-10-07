// This file executes an authorized customer balance credit repayment.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

// CreditBalanceRepaymentCreate consumes a proof and repays from the current customer's balance.
func (c *ControllerV1) CreditBalanceRepaymentCreate(ctx context.Context, req *v1.CreditBalanceRepaymentCreateReq) (res *v1.CreditBalanceRepaymentCreateRes, err error) {
	intent, err := creditBalanceRepaymentIntent(req.StatementNo, req.CreditBalanceRepaymentIntent)
	if err != nil {
		return nil, err
	}
	result, err := c.creditCommands.RepayCreditStatementFromBalance(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.CreditBalanceRepaymentCreateRes{Result: toAPICreditBalanceRepayment(result)}, nil
}
