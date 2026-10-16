package channel

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"

	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) PurchaseFileUpload(ctx context.Context, req *v1.PurchaseFileUploadReq) (res *v1.PurchaseFileUploadRes, err error) {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, bizerr.NewCode(channel.CodePurchaseInvalid)
	}
	value, err := c.delegate.svc.UploadPurchaseFile(ctx, channel.CredentialChannelID(ctx), req.UploadToken, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.PurchaseFileUploadRes{PurchaseFile: *value}, nil
}
