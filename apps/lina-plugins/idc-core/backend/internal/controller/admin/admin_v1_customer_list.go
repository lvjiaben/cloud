package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
)

func (c *ControllerV1) CustomerList(ctx context.Context, req *v1.CustomerListReq) (res *v1.CustomerListRes, err error) {
	page, err := c.customerManagement.Search(ctx, customermanagement.SearchInput{
		CustomerCode: req.CustomerCode, NamePrefix: req.NamePrefix, Phone: req.Phone, Type: customer.Type(req.Type), Status: customer.Status(req.Status),
		GroupNo: req.GroupNo, TagNo: req.TagNo, Cursor: req.Cursor, Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CustomerSummaryItem, 0, len(page.Customers))
	for _, item := range page.Customers {
		items = append(items, toAPICustomerSummary(item))
	}
	return &v1.CustomerListRes{Customers: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
