// This file prepares a current-session confirmation without issuing a cloud write.
package accountnative

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerConfirmation(ctx context.Context, req *v1.NativeServerConfirmationReq) (*v1.NativeServerConfirmationRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	if request := g.RequestFromCtx(ctx); request != nil {
		request.Response.Header().Set("Cache-Control", "no-store, max-age=0")
	}
	prepared, err := c.service.Prepare(ctx, customeroperations.Command{ServerNo: req.ServerNo, Action: req.Action, Parameters: req.Parameters, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, publicError(err)
	}
	result := &v1.NativeServerConfirmationRes{Required: prepared.Required, ProofToken: prepared.Token}
	if !prepared.ExpiresAt.IsZero() {
		stamp := prepared.ExpiresAt.UnixMilli()
		result.ExpiresAt = &stamp
	}
	return result, nil
}
