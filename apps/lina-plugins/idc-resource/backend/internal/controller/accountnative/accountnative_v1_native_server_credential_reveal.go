// This file writes one confirmed initial password directly to a no-store response.
package accountnative

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerCredentialReveal(ctx context.Context, req *v1.NativeServerCredentialRevealReq) (*v1.NativeServerCredentialRevealRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, publicError(customeroperations.ErrUnavailable)
	}
	response := request.Response
	response.Header().Set("Cache-Control", "no-store, max-age=0")
	response.Header().Set("Pragma", "no-cache")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := c.service.Reveal(ctx, req.ServerNo, req.Confirmed, func(value []byte) error {
		payload, marshalErr := json.Marshal(struct {
			Password string `json:"password"`
		}{Password: string(value)})
		if marshalErr != nil {
			return marshalErr
		}
		defer clear(payload)
		response.Write([]byte(`{"code":0,"message":"OK","data":`), payload, []byte(`}`))
		return nil
	})
	if err != nil {
		return nil, publicError(err)
	}
	return nil, nil
}
