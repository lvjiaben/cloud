package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskList(ctx context.Context, req *v1.ProductFulfillmentTaskListReq) (*v1.ProductFulfillmentTaskListRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	page, err := c.productFulfillment.List(ctx, productfulfillment.ListInput{
		OrderNo: req.OrderNo, CustomerNo: req.CustomerNo,
		Mode: req.FulfillmentModeCode, SafeStatus: req.StatusCode,
		Cursor: req.Cursor, Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	tasks := make([]*v1.ProductFulfillmentTaskSummary, 0, len(page.Items))
	for index := range page.Items {
		tasks = append(tasks, toAPIProductFulfillmentSummary(&page.Items[index]))
	}
	return &v1.ProductFulfillmentTaskListRes{Tasks: tasks, NextCursor: page.NextCursor, Total: page.Total}, nil
}
