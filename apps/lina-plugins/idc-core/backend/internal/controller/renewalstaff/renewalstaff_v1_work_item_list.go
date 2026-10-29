// This file projects authorized renewal work items into public millisecond DTOs.

package renewalstaff

import (
	"context"
	"lina-plugin-idc-core/backend/api/renewalstaff/v1"
)

func (c *ControllerV1) WorkItemList(ctx context.Context, req *v1.WorkItemListReq) (*v1.WorkItemListRes, error) {
	page, err := c.service.List(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.WorkItemListRes{Items: make([]v1.WorkItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		result.Items = append(result.Items, v1.WorkItem{WorkItemNo: item.WorkItemNo, InvoiceNo: item.InvoiceNo, CustomerNo: item.CustomerNo, SubjectNo: item.SubjectNo, ReasonCode: item.ReasonCode, CreatedAt: item.CreatedAt.UnixMilli()})
	}
	return result, nil
}
