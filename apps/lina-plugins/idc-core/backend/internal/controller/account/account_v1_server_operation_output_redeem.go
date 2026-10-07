// This file writes one-time server credentials directly to a no-store response.

package account

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerOperationOutputRedeem(ctx context.Context,
	req *v1.ServerOperationOutputRedeemReq) (res *v1.ServerOperationOutputRedeemRes, err error) {
	if !req.Confirmed {
		return nil, serveroperation.PublicError(serveroperation.ErrInvalidInput)
	}
	response := g.RequestFromCtx(ctx).Response
	response.Header().Set("Cache-Control", "no-store, max-age=0")
	response.Header().Set("Pragma", "no-cache")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = c.serverOperations.RedeemCurrentCustomerOutput(ctx, req.ServiceNo, req.OperationNo,
		func(payload []byte) error {
			if !json.Valid(payload) {
				return serveroperation.ErrInvalidInput
			}
			response.Write([]byte(`{"code":0,"message":"OK","data":{"payload":`), payload, []byte(`}}`))
			return nil
		})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return nil, nil
}
