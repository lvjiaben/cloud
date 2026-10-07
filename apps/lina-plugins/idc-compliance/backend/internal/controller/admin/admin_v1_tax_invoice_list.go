package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
)

func (c *ControllerV1) TaxInvoiceList(ctx context.Context, req *v1.TaxInvoiceListReq) (*v1.TaxInvoiceListRes, error) {
	page, err := c.taxInvoiceManagement.List(ctx, taxinvoicemanagement.ListQuery{Cursor: req.Cursor, CustomerNo: req.CustomerNo, StatusCode: req.StatusCode, Page: req.Page, Limit: req.PageSize})
	if err != nil {
		return nil, err
	}
	res := &v1.TaxInvoiceListRes{Items: make([]v1.TaxInvoiceManagementItem, 0, len(page.Items)), Total: page.Total, NextCursor: page.NextCursor}
	for index := range page.Items {
		res.Items = append(res.Items, *taxInvoiceManagementItem(&page.Items[index]))
	}
	return res, nil
}
