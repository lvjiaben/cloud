// This file projects the bounded operating overview into documented API fields.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/util/gconv"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

// OperationsDataGet returns actual receipts and operational metrics.
func (c *ControllerV1) OperationsDataGet(ctx context.Context, req *v1.OperationsDataGetReq) (*v1.OperationsDataGetRes, error) {
	data, err := c.operations.LoadData(ctx, req.CurrencyCode)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsDataGetRes{}
	if err = gconv.Struct(data, &res.OperationsData); err != nil {
		return nil, err
	}
	return res, nil
}
