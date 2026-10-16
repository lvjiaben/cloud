// This file adapts operating metric filters to the permission-checked read service.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/util/gconv"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/operations"
)

// OperationsDataRecordsGet returns an exact total and bounded detail page.
func (c *ControllerV1) OperationsDataRecordsGet(ctx context.Context, req *v1.OperationsDataRecordsGetReq) (*v1.OperationsDataRecordsGetRes, error) {
	data, err := c.operations.ListDataRecords(ctx, operations.DataQuery{Metric: operations.DataMetric(req.Metric), StartAt: req.StartAt, EndAt: req.EndAt, AsOf: req.AsOf, CurrencyCode: req.CurrencyCode, Status: req.Status, ProductNo: req.ProductNo, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsDataRecordsGetRes{}
	if err = gconv.Struct(data, &res.OperationsDataRecords); err != nil {
		return nil, err
	}
	return res, nil
}
