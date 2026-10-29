// This file delivers a consumed operation output directly to a no-store response.
package accountnative

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerOutputReveal(ctx context.Context, req *v1.NativeServerOutputRevealReq) (*v1.NativeServerOutputRevealRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	if pathErr := bindNativePath(ctx, "operationNo", &req.OperationNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	if pathErr := bindNativePath(ctx, "name", &req.Name); pathErr != nil {
		return nil, publicError(pathErr)
	}
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, publicError(customeroperations.ErrUnavailable)
	}
	response := request.Response
	response.Header().Set("Cache-Control", "no-store, max-age=0")
	response.Header().Set("Pragma", "no-cache")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := c.service.RevealOutput(ctx, req.ServerNo, req.OperationNo, req.Name, req.Confirmed, func(value []byte) error {
		response.Write([]byte(`{"code":0,"message":"OK","data":`), value, []byte(`}`))
		return nil
	})
	if err != nil {
		return nil, publicError(err)
	}
	return nil, nil
}
