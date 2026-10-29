package accountnative

import (
	"context"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerOperations(ctx context.Context, req *v1.NativeServerOperationsReq) (res *v1.NativeServerOperationsRes, err error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	page, err := c.service.Operations(ctx, req.ServerNo, req.Page, req.Limit, req.Action)
	if err != nil {
		return nil, publicError(err)
	}
	res = &v1.NativeServerOperationsRes{Items: make([]v1.NativeServerOperation, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, projectOperation(item))
	}
	return res, nil
}
