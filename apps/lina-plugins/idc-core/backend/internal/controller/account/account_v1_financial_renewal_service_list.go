package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialRenewalServiceList(ctx context.Context, req *v1.FinancialRenewalServiceListReq) (res *v1.FinancialRenewalServiceListRes, err error) {
	page, err := c.finance.ListAccountRenewalServices(ctx, billing.RenewalServiceDirectoryFilter{
		StatusCode: req.StatusCode, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialRenewalServiceListRes{
		Entitlements: toAPIFinancialRenewalServices(page.Services), NextCursor: page.NextCursor,
	}, nil
}
