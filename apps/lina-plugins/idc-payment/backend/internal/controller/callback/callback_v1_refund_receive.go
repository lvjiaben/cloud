// This file receives signed refund callbacks and writes only the provider's
// required acknowledgement after the payment service stores verified evidence.

package callback

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-payment/backend/api/callback/v1"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

// RefundReceive verifies and stores a refund callback before acknowledging the provider.
func (c *ControllerV1) RefundReceive(ctx context.Context, req *v1.RefundReceiveReq) (*v1.RefundReceiveRes, error) {
	request := g.RequestFromCtx(ctx)
	headers := map[string]string{}
	for key, values := range request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	_, provider, err := c.flow.RefundCallback(ctx, req.ChannelNo, paymentcap.CallbackRequest{
		Body: append([]byte(nil), request.GetBody()...), Headers: headers,
	})
	if err != nil {
		return nil, err
	}
	if provider == paymentcap.ProviderWechatPay {
		request.Response.Header().Set("Content-Type", "application/json")
		request.Response.Write(`{"code":"SUCCESS","message":"OK"}`)
	} else {
		request.Response.Write("success")
	}
	return nil, nil
}
