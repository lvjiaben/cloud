package accountnative

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerCommercialConfirmation(ctx context.Context, req *v1.NativeServerCommercialConfirmationReq) (res *v1.NativeServerCommercialConfirmationRes, err error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	if request := g.RequestFromCtx(ctx); request != nil {
		request.Response.Header().Set("Cache-Control", "no-store, max-age=0")
	}
	proof, err := c.commercial.PrepareCheckout(ctx, customeroperations.CommercialCheckoutCommand{ServerNo: req.ServerNo, QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	res = &v1.NativeServerCommercialConfirmationRes{Required: proof.Required, ProofToken: proof.Token}
	if !proof.ExpiresAt.IsZero() {
		stamp := proof.ExpiresAt.UnixMilli()
		res.ExpiresAt = &stamp
	}
	return res, nil
}
