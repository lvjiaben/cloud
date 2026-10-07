package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) FinancialInvoiceCreditSettlementCreate(ctx context.Context, req *v1.FinancialInvoiceCreditSettlementCreateReq) (res *v1.FinancialInvoiceCreditSettlementCreateRes, err error) {
	settlement, err := c.financeCommands.PayAccountInvoiceFromCredit(ctx, req.InvoiceNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceCreditSettlementCreateRes{
		Settlement: toAPIFinancialCreditSettlement(settlement),
	}, nil
}
