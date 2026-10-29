// This file polls the durable local task, never the upstream API.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerOperationDetail(ctx context.Context, req *v1.NativeServerOperationDetailReq) (*v1.NativeServerOperationDetailRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	if pathErr := bindNativePath(ctx, "operationNo", &req.OperationNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	operation, err := c.service.GetOperation(ctx, req.ServerNo, req.OperationNo)
	if err != nil {
		return nil, publicError(err)
	}
	return &v1.NativeServerOperationDetailRes{Operation: projectOperation(*operation)}, nil
}
