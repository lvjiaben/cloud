package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialManagementAdjustmentList(ctx context.Context, req *v1.FinancialManagementAdjustmentListReq) (res *v1.FinancialManagementAdjustmentListRes, err error) {
	page, err := c.finance.ListManagementAdjustments(ctx, billing.ManagementAdjustmentDirectoryFilter{
		CustomerNo: req.CustomerNo, AdjustmentTypeCode: req.AdjustmentTypeCode,
		StatusCode: req.StatusCode, CurrencyCode: req.CurrencyCode,
		Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementAdjustmentListRes{
		Total:       page.Total,
		Adjustments: toAPIFinancialManagementAdjustments(page.Adjustments), NextCursor: page.NextCursor,
	}, nil
}
